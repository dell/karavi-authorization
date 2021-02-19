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
	"karavi-authorization/pb"
	"log"
	"net"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

// tenantCreateCmd represents the tenant command
var tenantCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a tenant resource within Karavi",
	Long:  `Creates a tenant resource within Karavi`,
	Run: func(cmd *cobra.Command, args []string) {
		conn, err := grpc.Dial("grpc.tenants.cluster:443",
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

		t, err := tenantClient.CreateTenant(context.Background(), &pb.CreateTenantRequest{})
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("t = %+v\n", t)
	},
}

func init() {
	tenantCmd.AddCommand(tenantCreateCmd)
}
