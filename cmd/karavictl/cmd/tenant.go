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
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"karavi-authorization/pb"
	"log"
	"net"
	"os"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

// tenantCmd represents the tenant command
var tenantCmd = &cobra.Command{
	Use:              "tenant",
	TraverseChildren: true,
	Short:            "Manage tenants",
	Long:             `Management for tenants`,
	Run: func(cmd *cobra.Command, args []string) {
		if err := cmd.Usage(); err != nil {
			fmt.Fprintf(os.Stderr, "error: %+v\n", err)
		}
		os.Exit(1)
	},
}

func init() {
	rootCmd.AddCommand(tenantCmd)

	tenantCmd.PersistentFlags().String("addr", "localhost:443", "Address of the server")
	tenantCmd.PersistentFlags().Bool("insecure", false, "For insecure connections")
}

// CommandError wraps errors for reporting.
type CommandError struct {
	ErrorMsg string
}

// ErrorReporter represents a reporting function that can report in a specific format.
type ErrorReporter func(io.Writer, interface{}) error

func reportErrorAndExit(er ErrorReporter, w io.Writer, err error) {
	v := &CommandError{ErrorMsg: err.Error()}
	reporterErr := er(w, v)
	if reporterErr != nil {
		log.Fatal(reporterErr)
	}
	osExit(1)
}

func createTenantServiceClient(addr string, insecure bool) (pb.TenantServiceClient, io.Closer, error) {
	var conn *grpc.ClientConn
	var err error

	if insecure {
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

	tenantClient := pb.NewTenantServiceClient(conn)
	return tenantClient, conn, nil
}

func jsonOutput(w io.Writer, v interface{}) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(&v); err != nil {
		return err
	}
	return nil
}
