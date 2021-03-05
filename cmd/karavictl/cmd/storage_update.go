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
	"log"
	"os"

	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

// storageUpdateCmd represents the storage update command
var storageUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update a registered storage system.",
	Long:  `Updates a registered storage system.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Get the storage systems and update it in place?
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		// Convenience functions for ignoring errors whilst
		// getting flag values.
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
			Endpoint string
			SystemID string
			User     string
			Password string
			Insecure bool
		}{
			Type:     flagStringValue(cmd.Flags().GetString("type")),
			Endpoint: flagStringValue(cmd.Flags().GetString("endpoint")),
			SystemID: flagStringValue(cmd.Flags().GetString("system-id")),
			User:     flagStringValue(cmd.Flags().GetString("user")),
			Password: flagStringValue(cmd.Flags().GetString("password")),
			Insecure: flagBoolValue(cmd.Flags().GetBool("insecure")),
		}

		// TODO(ian): Check for password-stdin

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

		var didUpdate bool
		for k := range storage {
			if k != input.Type {
				continue
			}
			_, ok := storage[k][input.SystemID]
			if !ok {
				continue
			}

			storage[k][input.SystemID] = System{
				User:     input.User,
				Password: input.Password,
				Endpoint: input.Endpoint,
				Insecure: input.Insecure,
			}
			didUpdate = true
			break
		}
		if !didUpdate {
			fmt.Fprintf(os.Stderr, "no matching storage systems to update\n")
			os.Exit(1)
		}

		listData["storage"] = storage
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
	},
}

func init() {
	storageCmd.AddCommand(storageUpdateCmd)

	storageUpdateCmd.Flags().StringP("type", "t", "powerflex", "Type of storage system")
	storageUpdateCmd.Flags().StringP("endpoint", "e", "https://10.0.0.1", "Endpoint of REST API gateway")
	storageUpdateCmd.Flags().StringP("system-id", "s", "systemid", "System identifier")
	storageUpdateCmd.Flags().StringP("user", "u", "admin", "Username")
	storageUpdateCmd.Flags().StringP("password", "p", "****", "Password")
	storageUpdateCmd.Flags().BoolP("insecure", "i", false, "Insecure skip verify")
}
