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
	"errors"
	"fmt"
	"io"
	"os"
	"sync"
	"bufio"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"

	"github.com/spf13/cobra"
	"github.com/fatih/color"
)

var containerFlag string
var tailFlag int
var followFlag bool
var colorFlag bool
var colors []*color.Color

// logsCmd represents the logs command
var logsCmd = &cobra.Command{
	Use:   "logs <labelSelector>",
	Short: "Print the logs for a container in all matching pods.",
	Long: `Print the logs for a container in all matching pods. If the pod has only one container, the container name is optional.
Examples:
# Return snapshot logs in all "app=hello" pods with only one container
k8s-multi-pod logs "app=hello"

# Return snapshot logs in the ingress container for all "app=hello" pods
k8s-multi-pod logs "app=hello" -c ingress`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 1 {
			fmt.Println("Missing required argument: labelSelector")
			return
		}

		labelSelector := args[0]

		fmt.Println("\nRetrieving logs...this could take a minute.\n")

		// retrieve k8s client via .kube/config
		client, err := getClient()
		if err != nil {
			fmt.Println(err)
			return
		}

		err = GetMultiLogs(client, labelSelector, namespaceFlag, containerFlag, tailFlag, followFlag, colorFlag)
		if err != nil {
			fmt.Println(err)
		}

		return
	},
}

// GetMultiLogs retrieves all logs for the given label selector
func GetMultiLogs(client *unversioned.Client, labelSelector string, namespace string, container string, tail int, follow bool, useColor bool) error {
	// parse given label selector
	selector, err := labels.Parse(labelSelector)
	if err != nil {
		return err
	}

	// determine namespace to query
	if namespace == "" {
		namespace = api.NamespaceDefault
	}

	podIntr := client.Pods(namespace)

	// retrieve all pods by label selector
	pods, err := podIntr.List(api.ListOptions{
		FieldSelector: fields.Everything(),
		LabelSelector: selector,
	})
	if err != nil {
		return err
	}

	// notify caller that there were no pods
	if len(pods.Items) == 0 {
		return errors.New("No pods in namespace: " + namespace)
	}

	var wg sync.WaitGroup
	var col *color.Color
	if len(pods.Items) > 7 {
		useColor = false
	}

	// iterate over pods and get logs
	for ndx, pod := range pods.Items {
		// set pod logging options
		podLogOpts := &api.PodLogOptions{}
		if container != "" {
			podLogOpts.Container = container
		}

		if tail != -1 {
			convTail := int64(tail)
			podLogOpts.TailLines = &convTail
		}

		podLogOpts.Follow = follow

		if useColor {
			col = colors[ndx]
		} else {
			color.NoColor = true
			col = color.New(color.FgWhite)
		}

		// get specified pod's log request and run it
		req := podIntr.GetLogs(pod.Name, podLogOpts)
		stream, err := req.Stream()
		if err != nil {
			return err
		}

		// gather log request output
		if follow {
			wg.Add(1)
			go func(stream io.ReadCloser, podName string, wg *sync.WaitGroup, col *color.Color) {
				defer stream.Close()
				defer wg.Done()

				buf := bufio.NewReader(stream)
				for {
					line, _, err := buf.ReadLine()
					if err != nil {
						fmt.Println("Error from routine for", podName, ":", err)
						return
					}

					col.Printf("POD %s: %q\n", podName, line)
				}
			}(stream, pod.Name, &wg, col)
		} else {
			col.Set()
			fmt.Println("Logs for pod", pod.Name, ":")

			defer stream.Close()
			_, err = io.Copy(os.Stdout, stream)
			if err != nil {
				return err
			}

			color.Unset()
		}
	}

	if follow {
		wg.Wait()
	}

	return nil
}

func getClient() (*unversioned.Client, error) {
	// retrieve necessary kube config settings
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	configOverrides := &clientcmd.ConfigOverrides{}
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)

	// make a client config with kube config
	config, err := kubeConfig.ClientConfig()
	if err != nil {
		return nil, err
	}

	// make a client out of the kube client config
	client, err := unversioned.New(config)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func init() {
	RootCmd.AddCommand(logsCmd)
	logsCmd.Flags().StringVarP(&containerFlag, "container", "c", "", "Print the logs of this container")
	logsCmd.Flags().IntVarP(&tailFlag, "tail", "t", -1, "Lines of recent log file to display. Defaults to -1, showing all log lines.")
	logsCmd.Flags().BoolVarP(&followFlag, "follow", "f", false, "Attach the logging streams and watch them")
	logsCmd.Flags().BoolVarP(&colorFlag, "color", "l", false, "Use color in log output. Up to 7 pods.")

	colors = []*color.Color{color.New(color.FgBlue), color.New(color.FgWhite), color.New(color.FgGreen), color.New(color.FgMagenta),
		color.New(color.FgRed), color.New(color.FgCyan), color.New(color.FgYellow)}
}
