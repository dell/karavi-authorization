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
	"crypto/tls"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strings"

	"github.com/dustin/go-humanize"
	"github.com/spf13/cobra"
)

// roleListCmd represents the list command
var roleListCmd = &cobra.Command{
	Use:   "list",
	Short: "List roles",
	Long:  `List roles`,
	Run: func(cmd *cobra.Command, args []string) {
		r, err := http.NewRequest(http.MethodGet, "https://localhost/proxy/roles", nil)
		if err != nil {
			log.Fatal(err)
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
			log.Fatal(err)
		}

		var resp struct {
			Result map[string]struct {
				Pools []string `json:"pools"`
				Quota int64    `json:"quota"`
			} `json:"result"`
		}

		if err := json.NewDecoder(res.Body).Decode(&resp); err != nil {
			log.Fatal(err)
		}
		defer res.Body.Close()

		// Output the result by quota, ascending.
		var keys []string
		for k := range resp.Result {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			return resp.Result[keys[i]].Quota < resp.Result[keys[j]].Quota
		})
		fmt.Printf(`           Role          Pools          Quota
           ----          -----          -----`)
		fmt.Println()
		for _, k := range keys {
			v := resp.Result[k]
			fmt.Printf("%15s", k)
			fmt.Printf("%15s", strings.Join(v.Pools, ","))
			fmt.Printf("%15s\n", humanize.Bytes(uint64(v.Quota*1024)))
		}
	},
}

func init() {
	roleCmd.AddCommand(roleListCmd)
}
