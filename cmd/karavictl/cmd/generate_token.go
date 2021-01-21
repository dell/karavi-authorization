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
	"karavi-authorization/internal/token"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// generateTokenCmd represents the token command
var generateTokenCmd = &cobra.Command{
	Use:   "token",
	Short: "Generate tokens",
	Long: `Generate tokens for use with the CSI Driver when in proxy mode
The tokens are output as a Kubernetes Secret resource, so the results may
be piped directly to kubectl:

Example: karavictl generate token | kubectl apply -f -`,
	Run: func(cmd *cobra.Command, args []string) {
		addr, _ := cmd.Flags().GetString("addr")
		ns, _ := cmd.Flags().GetString("namespace")
		fromCfg, _ := cmd.Flags().GetString("from-config")
		ssecret, _ := cmd.Flags().GetString("shared-secret")
		cfg := token.GenerateConfig{
			Stdout:       os.Stdout,
			Addr:         addr,
			Namespace:    ns,
			FromConfig:   fromCfg,
			SharedSecret: strings.TrimSpace(ssecret),
		}
		if err := token.Generate(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "error generating token: %v\n", err)
			os.Exit(1)
		}
	},
}

func init() {
	generateCmd.AddCommand(generateTokenCmd)
	generateTokenCmd.Flags().String("addr", "grpc.gatekeeper.cluster:443", "host:port address")
	generateTokenCmd.Flags().String("namespace", "vxflexos", "Namespace of the CSI driver")
	generateTokenCmd.Flags().String("from-config", "", "File providing self-generated token information")
	generateTokenCmd.Flags().String("shared-secret", "", "Shared secret for token signing")
}
