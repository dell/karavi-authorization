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
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

// deleteRoleBindingCmd represents the rolebinding command
var deleteRoleBindingCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete a rolebinding between role and tenant",
	Long:  `Deletes a rolebinding between role and tenant`,
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

		tenant, err := cmd.Flags().GetString("tenant")
		if err != nil {
			log.Fatal(err)
		}
		role, err := cmd.Flags().GetString("role")
		if err != nil {
			log.Fatal(err)
		}

		_, err = tenantClient.UnbindRole(context.Background(), &pb.UnbindRoleRequest{
			TenantName: tenant,
			RoleName:   role,
		})
		if err != nil {
			log.Fatal(err)
		}

		fmt.Printf("Deleted a role binding between %q and %q\n", tenant, role)
	},
}

func init() {
	rolebindingCmd.AddCommand(deleteRoleBindingCmd)

	deleteRoleBindingCmd.Flags().StringP("tenant", "t", "", "Tenant name")
	deleteRoleBindingCmd.Flags().StringP("role", "r", "", "Role name")
}
