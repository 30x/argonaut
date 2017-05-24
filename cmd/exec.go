// Copyright © 2016 Apigee Corporation
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	remoteUtils "k8s.io/apimachinery/pkg/util/remotecommand"
	"k8s.io/client-go/kubernetes"
	api "k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/tools/remotecommand"

	"strconv"

	"github.com/30x/argonaut/utils"
	"github.com/fatih/color"
	"github.com/lunixbochs/vtclean"
	"github.com/spf13/cobra"
)

var execContainerFlag string
var stdinFlag bool
var ttyFlag bool

// execCmd represents the exec command
var execCmd = &cobra.Command{
	Use:   "exec",
	Short: "Execute a command in a container for all matching pods.",
	Long: `Execute a command in a container for all matching pods.

Examples:
# Get output from running 'date' in all "app=hello" pods, using the first container by default
argonaut exec "app=hello" date

# Get output from running 'nginx -V' for the ingress container in all "app=hello" pods
argonaut exec "app=hello" -c ingress -- nginx -V

# Opening an interactive shell session with all "app=hello" pods & colored output labels
argonaut exec "app=hello" sh -li

# Opening an interactive shell TTY session with all "app=hello" pods & colored output labels
argonaut exec "app=hello" sh -lit
`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 1 {
			fmt.Println("Missing required argument: labelSelector")
			return
		}

		labelSelector := args[0]

		if len(args) < 2 {
			fmt.Println("Missing required argument: command")
			return
		}

		command := args[1]

		client, err := utils.GetClient()
		if err != nil {
			fmt.Println(err)
			return
		}

		err = MultiExec(client, labelSelector, command, namespaceFlag, execContainerFlag, stdinFlag, ttyFlag, colorFlag)
		if err != nil {
			fmt.Println(err)
		}
	},
}

func init() {
	RootCmd.AddCommand(execCmd)

	execCmd.Flags().StringVarP(&execContainerFlag, "container", "c", "", "Container name. If omitted, the first container in the pod will be chosen")
	execCmd.Flags().BoolVarP(&stdinFlag, "stdin", "i", false, "Pass stdin to the container")
	execCmd.Flags().BoolVarP(&ttyFlag, "tty", "t", false, "Stdin is a TTY")
}

// MultiExec applies the
func MultiExec(client *kubernetes.Clientset, labelSelector string, command string, namespace string, container string, stdin bool, tty bool, useColor bool) (err error) {
	// parse given label selector
	selector, err := labels.Parse(labelSelector)
	if err != nil {
		return
	}

	// determine namespace to query
	if namespace == "" {
		namespace = api.NamespaceDefault
	}

	podIntr := client.Pods(namespace)

	// retrieve all pods by label selector
	pods, err := podIntr.List(metav1.ListOptions{
		FieldSelector: fields.Everything().String(),
		LabelSelector: selector.String(),
	})
	if err != nil {
		return
	}

	// notify caller that there were no pods
	if len(pods.Items) == 0 {
		return fmt.Errorf("No pods in namespace: %s", namespace)
	}

	var wg sync.WaitGroup
	var printLock sync.Mutex
	var stdinIO io.Reader
	var col *color.Color
	var writes []*io.PipeWriter
	colorLen := len(colors)

	if stdin {
		stdinIO = os.Stdin
	}

	restConf, err := utils.GetK8sRestConfig()
	if err != nil {
		return err
	}

	// podExecOpts := &api.PodExecOptions{
	// 	Container: container,
	// 	Command:   strings.Split(command, " "),
	// 	Stdin:     stdin, // let stdin flag decide
	// 	Stdout:    true,
	// 	Stderr:    true,
	// 	TTY:       tty, // let tty flag decide
	// }

	// start exec'ing on these pods
	for ndx, pod := range pods.Items {
		req := client.CoreV1().RESTClient().Post().
			Resource(api.ResourcePods.String()).
			Name(pod.Name).
			Namespace(pod.Namespace).
			SubResource("exec").
			Param("container", container).
			Param("command", command).
			Param("stdin", strconv.FormatBool(stdin)).
			Param("stdout", strconv.FormatBool(true)).
			Param("stderr", strconv.FormatBool(true)).
			Param("tty", strconv.FormatBool(tty))

		// VersionedParams(podExecOpts, metav1.ParameterCodec)

		// fmt.Printf("Request: %+v\n", req)

		streamExec, err := remotecommand.NewExecutor(restConf, "POST", req.URL())
		if err != nil {
			return err
		}

		if useColor {
			col = colors[ndx%colorLen] // give this stream one of the set colors
		} else {
			color.NoColor = true           // turn off all colors
			col = color.New(color.FgWhite) // set color to white to be safe
		}

		if tty || stdin {
			wg.Add(2)

			rtRead, mainWrite := io.Pipe()     // create main->routine pipe
			writes = append(writes, mainWrite) // keep track of main's writing end

			mainRead, rtWrite := io.Pipe() // create routine->main pipe

			opts := remotecommand.StreamOptions{
				SupportedProtocols: remoteUtils.SupportedStreamingProtocols,
				Stdin:              rtRead,
				Stdout:             rtWrite,
				Stderr:             os.Stderr,
				Tty:                tty,
			}

			// start threads
			go openPodSession(streamExec, opts, pod.Name, &wg, col)
			go readRoutineToStdout(pod.Name, mainRead, &wg, col, &printLock)
		} else {
			col.Printf("\"%s\" for pod %s:\n", command, pod.Name)

			opts := remotecommand.StreamOptions{
				SupportedProtocols: remoteUtils.SupportedStreamingProtocols,
				Stdin:              stdinIO,
				Stdout:             os.Stdout,
				Stderr:             os.Stderr,
				Tty:                false,
			}
			// this shouldn't have tty == true, b.c it should be a one-off command
			err = streamExec.Stream(opts)
			if err != nil {
				return err
			}
		}
	}

	if tty || stdin { // if using stdin or a tty, buffer os.Stdin and write to all consumers
		err = stdinToPods(writes)
		if err != nil {
			return err
		}

		fmt.Println("Waiting for threads...")

		wg.Wait()
	}

	return
}

// opens a stream with a pod as configured  by the given remote command, should be run in a go routine
func openPodSession(rmtCmd remotecommand.StreamExecutor, opts remotecommand.StreamOptions, podName string, wg *sync.WaitGroup, col *color.Color) {
	defer wg.Done()

	col.Printf("session for pod %s active\n", podName)
	err := rmtCmd.Stream(opts)
	if err != nil {
		fmt.Println("Error from routine for", podName, ":", err)
		return
	}

	return
}

// writes stdin from user to all pipes
func stdinToPods(writes []*io.PipeWriter) error {
	consolereader := bufio.NewScanner(os.Stdin)
	for consolereader.Scan() { // read stdin line by line
		input := consolereader.Text()

		err := writeToPods(writes, input) // write stdin to all pipes
		if err != nil {
			closePipes(writes)
			return err
		}
	}

	if err := consolereader.Err(); err != nil {
		closePipes(writes)
		return err
	}

	return nil
}

// writes given data (usually from stdin) to each consumer pipe
func writeToPods(writes []*io.PipeWriter, input string) error {
	for _, pipe := range writes {
		_, err := pipe.Write([]byte(input + "\n")) // add newline to flush writer
		if err != nil {
			return err
		}
	}

	return nil
}

// reads data from given read-in pipe, writes it stdout with a buffer
func readRoutineToStdout(name string, read *io.PipeReader, wg *sync.WaitGroup, col *color.Color, printLock *sync.Mutex) {
	defer wg.Done()

	// buffer each line before writing to stdout
	scanner := bufio.NewScanner(read)
	for scanner.Scan() {
		printLock.Lock() // request printing lock
		col.Printf("%s: ", name)
		fmt.Println(vtclean.Clean(scanner.Text(), false))
		printLock.Unlock() // unlock printing lock so other threads can print
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "There was an error with the scanner in pod %s: %v\n", name, err)
		read.CloseWithError(err)
	}
}

func closePipes(writes []*io.PipeWriter) {
	for _, pipe := range writes {
		err := pipe.Close()
		if err != nil {
			fmt.Fprintln(os.Stderr, "closing write pipes:", err)
		}
	}
}
