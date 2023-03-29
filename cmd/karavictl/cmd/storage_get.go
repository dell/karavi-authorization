// Copyright © 2021-2022 Dell Inc., or its subsidiaries. All Rights Reserved.
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
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"karavi-authorization/pb"
	"net/url"

	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

var (
	errSystemTypeNotSpecified = errors.New("system type not specified")
	errSystemIDNotSpecified   = errors.New("system id not specified")
)

// NewStorageGetCmd creates a new get command
func NewStorageGetCmd() *cobra.Command {
	getCmd := &cobra.Command{
		Use:   "get",
		Short: "Get details on a registered storage system.",
		Long:  `Gets details on a registered storage system.`,
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

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

			storType := flagStringValue(cmd.Flags().GetString("type"))
			if storType == "" {
				errAndExit(errSystemTypeNotSpecified)
			}

			sysID := flagStringValue(cmd.Flags().GetString("system-id"))
			if sysID == "" {
				errAndExit(errSystemIDNotSpecified)
			}

			addr := flagStringValue(cmd.Flags().GetString("addr"))
			insecure := flagBoolValue(cmd.Flags().GetBool("insecure"))
			var decodedSystem []byte
			var err error
			if addr != "" {
				// if addr flag is specified, make a grpc request
				decodedSystem, err = doStorageGetRequest(addr, storType, sysID, insecure, cmd)
				if err != nil {
					reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
				}

				m := make(map[string]interface{})
				if err := yaml.Unmarshal(decodedSystem, &m); err != nil {
					reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
				}

				err = JSONOutput(cmd.OutOrStdout(), m)
				if err != nil {
					reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf("unable to format json output: %v", err))
				}

			} else {

				// Get the current list of registered storage systems
				k3sCmd := execCommandContext(ctx, K3sPath, "kubectl", "get",
					"--namespace=karavi",
					"--output=json",
					"secret/karavi-storage-secret")

				b, err := k3sCmd.Output()
				if err != nil {
					errAndExit(err)
				}
				base64Systems := struct {
					Data map[string]string
				}{}
				if err := json.Unmarshal(b, &base64Systems); err != nil {
					errAndExit(err)
				}
				decodedSystems, err := base64.StdEncoding.DecodeString(base64Systems.Data["storage-systems.yaml"])
				if err != nil {
					errAndExit(err)
				}

				var listData map[string]Storage
				if err := yaml.Unmarshal(decodedSystems, &listData); err != nil {
					errAndExit(err)
				}
				if listData == nil || listData["storage"] == nil {
					listData = make(map[string]Storage)
					listData["storage"] = make(Storage)
				}
				var storage = listData["storage"]

				for k := range storage {
					if k != storType {
						continue
					}
					id, ok := storage[k][sysID]
					if !ok {
						continue
					}

					id.Password = "(omitted)"
					if err := JSONOutput(cmd.OutOrStdout(), &id); err != nil {
						reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
					}
					break
				}
			}
		},
	}
	getCmd.Flags().StringP("type", "t", "", "Type of storage system")
	getCmd.Flags().StringP("system-id", "s", "", "System identifier")
	return getCmd
}

func doStorageGetRequest(addr string, storageType string, systemID string, insecure bool, cmd *cobra.Command) ([]byte, error) {

	client, err := CreateHttpClient(fmt.Sprintf("https://%s", addr), insecure)
	if err != nil {
		reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
	}

	query := url.Values{
		"StorageType": []string{storageType},
		"SystemId":    []string{systemID},
	}

	var resp pb.StorageGetResponse
	err = client.Get(context.Background(), "/proxy/storage/get", nil, query, &resp)
	if err != nil {
		reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
	}

	return resp.Storage, nil
}
