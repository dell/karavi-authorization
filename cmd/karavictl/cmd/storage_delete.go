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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"karavi-authorization/pb"
	"log"
	"os"

	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

// NewStorageDeleteCmd creates a new delete command
func NewStorageDeleteCmd() *cobra.Command {
	deleteCmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a registered storage system.",
		Long:  `Deletes a registered storage system.`,
		Run: func(cmd *cobra.Command, args []string) {
			log.SetFlags(log.Llongfile | log.LstdFlags)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			flagStringValue := func(v string, err error) string {
				if err != nil {
					log.Fatal(err)
				}
				return v
			}
			flagBoolValue := func(v bool, err error) bool {
				if err != nil {
					log.Fatal(err)
				}
				return v
			}

			// Gather the inputs
			var input = struct {
				Type     string
				SystemID string
			}{
				Type:     flagStringValue(cmd.Flags().GetString("type")),
				SystemID: flagStringValue(cmd.Flags().GetString("system-id")),
			}

			addr := flagStringValue(cmd.Flags().GetString("addr"))
			insecure := flagBoolValue(cmd.Flags().GetBool("insecure"))
			if addr != "" {
				// if addr flag is specified, make a grpc request
				if err := doStorageDeleteRequest(addr, input.Type, input.SystemID, insecure); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "error: %+v\n", err)
					osExit(1)
				}
			} else {

				// Get the current resource

				k3sCmd := execCommandContext(ctx, K3sPath, "kubectl", "get",
					"--namespace=karavi",
					"--output=json",
					"secret/karavi-storage-secret")

				b, err := k3sCmd.Output()
				if err != nil {
					log.Fatal(err)
				}

				base64Systems := struct {
					Data map[string]string
				}{}
				if err := json.Unmarshal(b, &base64Systems); err != nil {
					log.Fatal(err)
				}
				decodedSystems, err := base64.StdEncoding.DecodeString(base64Systems.Data["storage-systems.yaml"])
				if err != nil {
					log.Fatal(err)
				}

				var listData map[string]Storage
				if err := yaml.Unmarshal(decodedSystems, &listData); err != nil {
					log.Fatal(err)
				}
				if listData == nil || listData["storage"] == nil {
					listData = make(map[string]Storage)
					listData["storage"] = make(Storage)
				}
				var storage = listData["storage"]

				if storage == nil {
					log.Println("no config")
					return
				}
				m, ok := storage[input.Type]
				if !ok {
					log.Println("no storage of type", input.Type)
					return
				}
				if _, ok := m[input.SystemID]; !ok {
					log.Println("system id does not exist")
					return
				}

				delete(m, input.SystemID)
				storage[input.Type] = m
				listData["storage"] = storage

				// Merge the new connection details and apply them.

				b, err = yaml.Marshal(&listData)
				if err != nil {
					log.Fatal(err)
				}

				tmpFile, err := ioutil.TempFile("", "karavi")
				if err != nil {
					log.Fatal(err)
				}
				defer func() {
					if err := tmpFile.Close(); err != nil {
						fmt.Fprintf(os.Stderr, "error: %+v\n", err)
					}
					if err := os.Remove(tmpFile.Name()); err != nil {
						fmt.Fprintf(os.Stderr, "error: %+v\n", err)
					}
				}()
				_, err = tmpFile.WriteString(string(b))
				if err != nil {
					log.Fatal(err)
				}

				crtCmd := execCommandContext(ctx, K3sPath, "kubectl", "create",
					"--namespace=karavi",
					"secret", "generic", "karavi-storage-secret",
					fmt.Sprintf("--from-file=storage-systems.yaml=%s", tmpFile.Name()),
					"--output=yaml",
					"--dry-run=client")
				appCmd := execCommandContext(ctx, K3sPath, "kubectl", "apply", "-f", "-")

				if err := pipeCommands(crtCmd, appCmd); err != nil {
					log.Fatal(err)
				}
			}
		},
	}
	deleteCmd.Flags().StringP("type", "t", "powerflex", "Type of storage system")
	deleteCmd.Flags().StringP("system-id", "s", "systemid", "System identifier")
	return deleteCmd
}

func doStorageDeleteRequest(addr string, systemType string, systemID string, grpcInsecure bool) error {

	client, conn, err := CreateStorageServiceClient(addr, grpcInsecure)
	if err != nil {
		return err
	}
	defer conn.Close()

	req := &pb.StorageDeleteRequest{
		StorageType: systemType,
		SystemId:    systemID,
	}

	_, err = client.Delete(context.Background(), req)
	if err != nil {
		return err
	}

	return nil
}
