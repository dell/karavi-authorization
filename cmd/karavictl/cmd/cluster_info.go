/*
Copyright © 2020 Dell Inc., or its subsidiaries. All Rights Reserved.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

// clusterInfoCmd represents the clusterInfo command
var clusterInfoCmd = &cobra.Command{
	Use:   "cluster-info",
	Short: "Display the state of resources within the cluster.",
	Long:  `Prints table of resources within the cluster, including their readiness.`,
	Run: func(cmd *cobra.Command, args []string) {
		cmdArgs := []string{"kubectl", "get", "deploy", "-n", "karavi"}
		if v, _ := cmd.Flags().GetBool("watch"); v {
			cmdArgs = append(cmdArgs, "--watch")
		}
		kCmd := exec.Command("k3s", cmdArgs...)
		kCmd.Stdout = os.Stdout
		err := kCmd.Start()
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}

		if err := kCmd.Wait(); err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	rootCmd.AddCommand(clusterInfoCmd)
	clusterInfoCmd.Flags().BoolP("watch", "w", false, "Watch for changes.")
}
