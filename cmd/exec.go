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
	"fmt"

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
k8s-multi-pod exec "app=hello" date

# Get output from running 'nginx -V' for the ingress container in all "app=hello" pods
k8s-multi-pod exec "app=hello" -c ingress -- nginx -V
`,
	Run: func(cmd *cobra.Command, args []string) {
		// TODO: Work your own magic here
		fmt.Println("exec called")
	},
}

func init() {
	RootCmd.AddCommand(execCmd)

	execCmd.Flags().StringVarP(&execContainerFlag, "container", "c", "", "Container name. If omitted, the first container in the pod will be chosen")
	execCmd.Flags().BoolVarP(&stdinFlag, "stdin", "i", false, "Pass stdin to the container")
	execCmd.Flags().BoolVarP(&ttyFlag, "tty", "t", false, "Stdin is a TTY")
}
