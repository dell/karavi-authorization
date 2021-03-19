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
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"karavi-authorization/pb"
	"log"
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
}

// CommandError wraps errors for reporting.
type CommandError struct {
	ErrorMsg string
}

// ErrorReporter represents a reporting function that can report in a specific format.
type ErrorReporter func(io.Writer, interface{}) error

func reportErrorAndExit(er ErrorReporter, w io.Writer, err error) {
	v := &CommandError{ErrorMsg: err.Error()}
	er(w, v)
	osExit(1)
}

func createTenantServiceClient(addr string) (pb.TenantServiceClient, io.Closer, error) {
	// TODO: Pass in an insecure flag for the self-signed case.
	// TODO: It may not be feasible to require "grpc.hostname", since it will require
	//       an extra DNS entry. I tested this successfully when adding it to /etc/hosts
	//       though.
	//       Perhaps instead we could try taking advantage of the fact that a gRPC call
	//       makes a request where the path begins with the proto namespace of the service
	//       itself.  E.g. /karavi/TenantService/... => Path /karavi.
	certs, err := x509.SystemCertPool()
	if err != nil {
		return nil, nil, err
	}
	creds := credentials.NewClientTLSFromCert(certs, "")

	conn, err := grpc.Dial(addr,
		grpc.WithTransportCredentials(creds),
		grpc.WithTimeout(10*time.Second))
	if err != nil {
		log.Fatal(err)
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
