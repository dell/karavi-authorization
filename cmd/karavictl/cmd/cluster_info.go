// Copyright © 2021 Dell Inc., or its subsidiaries. All Rights Reserved.
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
	"context"
	"errors"
	"fmt"
	"karavi-authorization/internal/web"
	"net/http"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

// NewClusterInfoCmd creates a new clusterInfo command
func NewClusterInfoCmd() *cobra.Command {
	clusterInfoCmd := &cobra.Command{
		Use:   "cluster-info",
		Short: "Display the state of resources within the cluster",
		Long:  `Prints table of resources within the cluster, including their readiness`,
		Run: func(cmd *cobra.Command, args []string) {
			errAndExit := func(err error) {
				fmt.Fprintf(cmd.ErrOrStderr(), "error: %+v\n", err)
				osExit(1)
			}

			// Convenience functions for ignoring errors whilst
			// getting flag values.
			flagStringValue := func(v string, err error) string {
				if err != nil {
					errAndExit(err)
				}
				return v
			}

			flagBoolValue := func(v bool, err error) bool {
				if err != nil {
					errAndExit(err)
				}
				return v
			}

			addr := flagStringValue(cmd.Flags().GetString("addr"))
			if addr == "" {
				errAndExit(fmt.Errorf("address not specified"))
			}

			insecure := flagBoolValue(cmd.Flags().GetBool("insecure"))

			// validate token by making arbitrary request to proxy-server
			client, err := CreateHTTPClient(fmt.Sprintf("https://%s", addr), insecure)
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
			}

			err = client.Get(context.Background(), "/proxy/storage/", nil, nil, nil)
			if err != nil {
				var jsonErr web.JSONError
				if errors.As(err, &jsonErr) {
					if jsonErr.Code == http.StatusUnauthorized {
						reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf("unauthorized"))
					}
				}
			}

			cmdArgs := []string{"kubectl", "get", "deploy", "-n", "karavi"}
			if v, _ := cmd.Flags().GetBool("watch"); v {
				cmdArgs = append(cmdArgs, "--watch")
			}
			kCmd := exec.Command(K3sPath, cmdArgs...)
			kCmd.Stdout = os.Stdout
			err = kCmd.Start()
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

	clusterInfoCmd.Flags().BoolP("watch", "w", false, "Watch for changes")
	return clusterInfoCmd
}
