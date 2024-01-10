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
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/spf13/cobra"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// NewTenantCmd creates a new tenant command
func NewTenantCmd() *cobra.Command {
	tenantCmd := &cobra.Command{
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

	tenantCmd.PersistentFlags().StringP("admin-token", "f", "", "Path to admin token file; required")
	tenantCmd.PersistentFlags().String("addr", "", "Address of the CSM Authorization Proxy Server; required")
	tenantCmd.PersistentFlags().Bool("insecure", false, "Skip certificate validation of the CSM Authorization Proxy Server")

	err := tenantCmd.MarkPersistentFlagRequired("admin-token")
	if err != nil {
		reportErrorAndExit(JSONOutput, tenantCmd.ErrOrStderr(), err)
	}

	err = tenantCmd.MarkPersistentFlagRequired("addr")
	if err != nil {
		reportErrorAndExit(JSONOutput, tenantCmd.ErrOrStderr(), err)
	}

	tenantCmd.AddCommand(NewTenantCreateCmd())
	tenantCmd.AddCommand(NewTenantDeleteCmd())
	tenantCmd.AddCommand(NewTenantGetCmd())
	tenantCmd.AddCommand(NewTenantListCmd())
	tenantCmd.AddCommand(NewTenantRevokeCmd())
	tenantCmd.AddCommand(NewTenantUpdateCmd())
	return tenantCmd
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

func jsonOutput(w io.Writer, v interface{}) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	err := enc.Encode(&v)
	if err != nil {
		return err
	}
	return nil
}

func output(w io.Writer, v interface{}) error {
	_, err := fmt.Fprint(w, v)
	if err != nil {
		return err
	}
	return nil
}

// jsonOutput() omits boolean flag on false value while encoding
func jsonOutputEmitEmpty(w io.Writer, m protoreflect.ProtoMessage) error {
	enc := protojson.MarshalOptions{Multiline: true, EmitUnpopulated: true, Indent: ""}
	data := enc.Format(m)
	fmt.Fprintln(w, data)
	return nil
}
