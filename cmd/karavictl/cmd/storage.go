// Copyright © 2021-2022 Dell Inc., or its subsidiaries. All Rights Reserved.
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
	"crypto/x509"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"time"

	"karavi-authorization/pb"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// NewStorageCmd creates a new storage command
func NewStorageCmd() *cobra.Command {
	storageCmd := &cobra.Command{
		Use:   "storage",
		Short: "Manage storage systems",
		Long:  `Manages storage systems`,
		Run: func(cmd *cobra.Command, args []string) {
			if err := cmd.Usage(); err != nil {
				fmt.Fprintf(os.Stderr, "error: %+v\n", err)
			}
			os.Exit(1)
		},
	}

	storageCmd.PersistentFlags().StringP("admin-token", "f", "", "Path to admin token file; required")
	storageCmd.PersistentFlags().String("addr", "", "Address of the CSM Authorization Proxy Server; required")
	storageCmd.PersistentFlags().Bool("insecure", false, "Skip certificate validation of the CSM Authorization Proxy Server")

	err := storageCmd.MarkPersistentFlagRequired("admin-token")
	if err != nil {
		reportErrorAndExit(JSONOutput, storageCmd.ErrOrStderr(), err)
	}

	err = storageCmd.MarkPersistentFlagRequired("addr")
	if err != nil {
		reportErrorAndExit(JSONOutput, storageCmd.ErrOrStderr(), err)
	}

	storageCmd.AddCommand(NewStorageCreateCmd())
	storageCmd.AddCommand(NewStorageDeleteCmd())
	storageCmd.AddCommand(NewStorageGetCmd())
	storageCmd.AddCommand(NewStorageListCmd())
	storageCmd.AddCommand(NewStorageUpdateCmd())
	return storageCmd
}

func createStorageServiceClient(addr string, insecure bool) (pb.StorageServiceClient, io.Closer, error) {
	var conn *grpc.ClientConn
	var err error

	if insecure { // #nosec G402
		conn, err = grpc.Dial(addr,
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

	} else {
		certs, err := x509.SystemCertPool()
		if err != nil {
			return nil, nil, err
		}
		creds := credentials.NewClientTLSFromCert(certs, "")

		conn, err = grpc.Dial(addr,
			grpc.WithTransportCredentials(creds),
			grpc.WithTimeout(10*time.Second))
		if err != nil {
			log.Fatal(err)
		}
	}

	storageClient := pb.NewStorageServiceClient(conn)
	return storageClient, conn, nil
}
