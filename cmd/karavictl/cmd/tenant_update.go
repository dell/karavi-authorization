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
	"crypto/tls"
	"fmt"
	"karavi-authorization/pb"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

// tenantUpdateCmd represents the update command
var tenantUpdateCmd = &cobra.Command{
	Use:   "update",
	Short: "Update a tenant resource within Karavi",
	Long:  `Updates a tenant resource within Karavi`,
	Run: func(cmd *cobra.Command, args []string) {
		conn, err := grpc.Dial("localhost:443",
			grpc.WithAuthority("grpc.tenants.cluster"),
			grpc.WithTimeout(10*time.Second),
			grpc.WithContextDialer(func(_ context.Context, addr string) (net.Conn, error) {
				return tls.Dial("tcp", addr, &tls.Config{
					NextProtos:         []string{"h2"},
					InsecureSkipVerify: true,
				})
			}),
			grpc.WithInsecure())
		if err != nil {
			log.Fatal(err)
		}
		defer conn.Close()

		tenantClient := pb.NewTenantServiceClient(conn)

		name, err := cmd.Flags().GetString("name")
		if err != nil {
			log.Fatal(err)
		}
		if strings.TrimSpace(name) == "" {
			fmt.Fprint(cmd.ErrOrStderr(), "error: invalid tenant name")
			os.Exit(1)
		}

		roles, err := cmd.Flags().GetString("roles")
		if err != nil {
			log.Fatal(err)
		}

		t, err := tenantClient.UpdateTenant(context.Background(), &pb.UpdateTenantRequest{
			Tenant: &pb.Tenant{
				Name:  name,
				Roles: roles,
			},
		})
		if err != nil {
			log.Fatal(err)
		}
		fmt.Fprintf(cmd.ErrOrStderr(), "Updated tenant %q\n", t.Name)
	},
}

func init() {
	tenantCmd.AddCommand(tenantUpdateCmd)

	tenantUpdateCmd.Flags().StringP("name", "n", "", "Tenant name")
	tenantUpdateCmd.Flags().StringP("roles", "r", "", "Comma-separated list of roles, e.g. \"role1,role2\"")
}
