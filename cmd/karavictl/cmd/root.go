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
	"fmt"
	"net/http"
	"os"

	"karavi-authorization/cmd/karavictl/cmd/api"

	"github.com/spf13/cobra"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/viper"
)

// Common constants.
const (
	// K3sPath is the expected path to the k3s binary.
	K3sPath = "/usr/local/bin/k3s"
)

var cfgFile string

// NewRootCmd creates a new base command when called without any subcommands
func NewRootCmd() *cobra.Command {
	cobra.OnInitialize(initConfig)

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
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.karavictl.yaml)")
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

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

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".karavictl" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".karavictl")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	_ = viper.ReadInConfig()
}
