package cmd

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

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
)

// roleListCmd represents the list command
var roleListCmd = &cobra.Command{
	Use:   "list",
	Short: "List roles",
	Long:  `List roles`,
	Run: func(cmd *cobra.Command, args []string) {
		roles, err := GetRoles()
		if err != nil {
			log.Fatal(err)
		}

		fmt.Fprintf(cmd.OutOrStdout(), "%20s", "Role")
		fmt.Fprintf(cmd.OutOrStdout(), "%20s", "Storage System")
		fmt.Fprintf(cmd.OutOrStdout(), "%20s", "Storage Pool")
		fmt.Fprintf(cmd.OutOrStdout(), "%20s", "Quota")
		fmt.Fprintln(cmd.OutOrStdout(), "")

		fmt.Fprintf(cmd.OutOrStdout(), "%20s", "----")
		fmt.Fprintf(cmd.OutOrStdout(), "%20s", "--------------")
		fmt.Fprintf(cmd.OutOrStdout(), "%20s", "------------")
		fmt.Fprintf(cmd.OutOrStdout(), "%20s", "-----")
		fmt.Fprintln(cmd.OutOrStdout(), "")

		for roleName, roleDetails := range roles {
			for _, role := range roleDetails {
				for _, poolQuota := range role.PoolQuotas {
					fmt.Fprintf(cmd.OutOrStdout(), "%20s", roleName)
					fmt.Fprintf(cmd.OutOrStdout(), "%20s", role.StorageSystemID)
					fmt.Fprintf(cmd.OutOrStdout(), "%20s", poolQuota.Pool)
					fmt.Fprintf(cmd.OutOrStdout(), "%20s", humanize.Bytes(uint64(poolQuota.Quota*1024)))
					fmt.Fprintln(cmd.OutOrStdout(), "")
				}
			}
		}

	},
}

// GetRoles returns all of the roles with associated storage systems, storage pools, and quotas
func GetRoles() (map[string][]Role, error) {
	r, err := http.NewRequest(http.MethodGet, "https://localhost/proxy/roles", nil)
	if err != nil {
		return nil, err
	}

	h := http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
	}

	res, err := h.Do(r)
	if err != nil {
		return nil, err
	}

	var resp struct {
		Result map[string][]Role `json:"result"`
	}

	if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
		return nil, err
	}
	defer res.Body.Close()

	return resp.Result, nil
}

func init() {
	roleCmd.AddCommand(roleListCmd)
}
