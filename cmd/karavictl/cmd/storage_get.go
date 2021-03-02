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
	"log"

	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

// getCmd represents the get command
var getCmd = &cobra.Command{
	Use:   "get",
	Short: "Get details on a registered storage system.",
	Long:  `Gets details on a registered storage system.`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		sysType, err := cmd.Flags().GetString("type")
		if err != nil {
			log.Fatal(err)
		}
		sysID, err := cmd.Flags().GetString("system-id")
		if err != nil {
			log.Fatal(err)
		}

		// Get the current list of registered storage systems
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

		for k := range storage {
			if k != sysType {
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
	},
}

func init() {
	storageCmd.AddCommand(getCmd)

	getCmd.Flags().StringP("type", "t", "powerflex", "Type of storage system")
	getCmd.Flags().StringP("system-id", "s", "systemid", "System identifier")
}
