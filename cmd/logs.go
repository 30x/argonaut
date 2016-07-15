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

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/unversioned"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"

	"github.com/spf13/cobra"
)

var containerFlag string
var tailFlag int

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

		fmt.Println("Retrieving logs...this could take a minute.")

		// retrieve k8s client via .kube/config
		client, err := getClient()
		if err != nil {
			fmt.Println(err)
			return
		}

		err = GetMultiLogs(client, labelSelector, namespaceFlag, containerFlag, tailFlag)
		if err != nil {
			fmt.Println(err)
		}

		return
	},
}

// GetMultiLogs retrieves all logs for the given label selector
func GetMultiLogs(client *unversioned.Client, labelSelector string, namespace string, container string, tail int) error {
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

	// iterate over pods and get logs
	for _, pod := range pods.Items {
		// set pod logging options
		podLogOpts := &api.PodLogOptions{}
		if container != "" {
			podLogOpts.Container = container
		}

		if tail != -1 {
			convTail := int64(tail)
			podLogOpts.TailLines = &convTail
		}

		// get specified pod's log request and run it
		req := podIntr.GetLogs(pod.Name, podLogOpts)
		stream, err := req.Stream()
		if err != nil {
			return err
		}

		fmt.Println("Logs for pod", pod.Name, ":")
		// gather log request output
		defer stream.Close()
		_, err = io.Copy(os.Stdout, stream)
		if err != nil {
			return err
		}
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
}
