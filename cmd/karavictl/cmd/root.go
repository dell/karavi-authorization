// Copyright © 2021-2023 Dell Inc., or its subsidiaries. All Rights Reserved.
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
	"karavi-authorization/cmd/karavictl/cmd/api"
	"karavi-authorization/internal/token"
	"net/http"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

// Common constants.
const (
	// K3sPath is the expected path to the k3s binary.
	K3sPath = "/usr/local/bin/k3s"
)

// NewRootCmd creates a new base command when called without any subcommands
func NewRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "karavictl",
		Short: "karavictl is used to interact with karavi server",
		Long: `karavictl provides security, RBAC, and quota limits for accessing Dell
	storage products from Kubernetes clusters`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := cmd.Execute(); err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		},
	}

	rootCmd.AddCommand(NewRoleCmd())
	rootCmd.AddCommand(NewRoleBindingCmd())
	rootCmd.AddCommand(NewTenantCmd())
	rootCmd.AddCommand(NewClusterInfoCmd())
	rootCmd.AddCommand(NewGenerateCmd())
	rootCmd.AddCommand(NewStorageCmd())
	rootCmd.AddCommand(NewAdminCmd())
	return rootCmd
}

func createHTTPClient(addr string, insecure bool) (api.Client, error) {
	c, err := api.New(context.Background(), addr, api.ClientOptions{
		Insecure:   insecure,
		HTTPClient: http.DefaultClient,
	})
	if err != nil {
		return nil, err
	}

	return c, nil
}

func readAccessAdminToken(admTknFile string) (string, string, error) {
	admintoken := token.AdminToken{}

	if admTknFile != "" {
		dat, err := os.ReadFile(filepath.Clean(admTknFile))
		if err != nil {
			return "", "", err
		}

		if err := yaml.Unmarshal(dat, &admintoken); err != nil {
			return "", "", err
		}
		return string(admintoken.Access), string(admintoken.Refresh), nil
	}
	return "", "", errors.New("specify admin token file")
}
