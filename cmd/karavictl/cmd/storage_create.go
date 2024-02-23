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
	"io"
	"karavi-authorization/internal/token"
	"karavi-authorization/internal/web"
	"karavi-authorization/pb"
	"net/http"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
)

const (
	powerflex  = "powerflex"
	powermax   = "powermax"
	powerscale = "powerscale"
)

// Storage represents a map of storage system types.
type Storage map[string]SystemType

// SystemType represents a map of systems.
type SystemType map[string]System

// System represents the properties of a system.
type System struct {
	User     string `yaml:"User"`
	Password string `yaml:"Password"`
	Endpoint string `yaml:"Endpoint"`
	Insecure bool   `yaml:"Insecure"`
}

// SystemID wraps a system ID to be a quoted string because system IDs could be all numbers
// which will cause issues with yaml marshalers
type SystemID struct {
	Value string
}

func (id SystemID) String() string {
	return fmt.Sprintf("%q", strings.ReplaceAll(id.Value, `"`, ""))
}

// SupportedStorageTypes is the map of supported storage types for CSM Authorization
var SupportedStorageTypes = map[string]struct{}{
	powerflex:  {},
	powermax:   {},
	powerscale: {},
}

// NewStorageCreateCmd creates a new create command
func NewStorageCreateCmd() *cobra.Command {
	storageCreateCmd := &cobra.Command{
		Use:   "create",
		Short: "Create and register a storage system.",
		Long:  `Creates and registers a storage system.`,
		Run: func(cmd *cobra.Command, _ []string) {
			outFormat := "failed to create storage: %+v\n"

			errAndExit := func(err error) {
				fmt.Fprintf(cmd.ErrOrStderr(), "error: %+v\n", err)
				osExit(1)
			}

			// Convenience functions for ignoring errors whilst
			// getting flag values.
			flagStringValue := func(v string, err error) string {
				if err != nil {
					errAndExit(err)
				}
				return v
			}
			flagBoolValue := func(v bool, err error) bool {
				if err != nil {
					errAndExit(err)
				}
				return v
			}
			verifyInput := func(v string) string {
				inputText := flagStringValue(cmd.Flags().GetString(v))
				if strings.TrimSpace(inputText) == "" {
					errAndExit(fmt.Errorf("no input provided: %s", v))
				}
				return inputText
			}

			// Gather the inputs
			input := struct {
				Type          string
				Endpoint      string
				SystemID      string
				User          string
				Password      string
				ArrayInsecure bool
			}{
				Type:          verifyInput("type"),
				Endpoint:      verifyInput("endpoint"),
				SystemID:      flagStringValue(cmd.Flags().GetString("system-id")),
				User:          verifyInput("user"),
				Password:      flagStringValue(cmd.Flags().GetString("password")),
				ArrayInsecure: flagBoolValue(cmd.Flags().GetBool("array-insecure")),
			}

			addr := verifyInput("addr")

			insecure, err := cmd.Flags().GetBool("insecure")
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf(outFormat, err))
			}

			admTknFile, err := cmd.Flags().GetString("admin-token")
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
			}
			if admTknFile == "" {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), errors.New("specify token file"))
			}
			accessToken, refreshToken, err := ReadAccessAdminToken(admTknFile)
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
			}
			adminTknBody := token.AdminToken{
				Refresh: refreshToken,
				Access:  accessToken,
			}

			if err := doStorageCreateRequest(context.Background(), addr, input, insecure, cmd, adminTknBody); err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf(outFormat, err))
			}
		},
	}

	storageCreateCmd.Flags().StringP("type", "t", "", "Type of storage system")
	err := storageCreateCmd.MarkFlagRequired("type")
	if err != nil {
		reportErrorAndExit(JSONOutput, storageCreateCmd.ErrOrStderr(), err)
	}
	storageCreateCmd.Flags().StringP("endpoint", "e", "", "Endpoint of REST API gateway")
	err = storageCreateCmd.MarkFlagRequired("endpoint")
	if err != nil {
		reportErrorAndExit(JSONOutput, storageCreateCmd.ErrOrStderr(), err)
	}
	storageCreateCmd.Flags().StringP("user", "u", "", "Username")
	err = storageCreateCmd.MarkFlagRequired("user")
	if err != nil {
		reportErrorAndExit(JSONOutput, storageCreateCmd.ErrOrStderr(), err)
	}
	storageCreateCmd.Flags().StringP("system-id", "s", "", "System identifier")
	storageCreateCmd.Flags().StringP("password", "p", "", "Specify password, or omit to use stdin")
	storageCreateCmd.Flags().BoolP("array-insecure", "a", false, "Array insecure skip verify")

	return storageCreateCmd
}

func readPassword(w io.Writer, prompt string, p *string) {
	fmt.Fprintf(w, prompt)
	b, err := termReadPassword(int(syscall.Stdin))
	if err != nil {
		reportErrorAndExit(JSONOutput, w, err)
	}
	fmt.Fprintln(w)
	*p = string(b)
}

type input struct {
	Type          string
	Endpoint      string
	SystemID      string
	User          string
	Password      string
	ArrayInsecure bool
}

func doStorageCreateRequest(ctx context.Context, addr string, system input, insecure bool, cmd *cobra.Command, adminTknBody token.AdminToken) error {
	client, err := CreateHTTPClient(fmt.Sprintf("https://%s", addr), insecure)
	if err != nil {
		reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
	}

	body := &pb.StorageCreateRequest{
		StorageType: system.Type,
		Endpoint:    system.Endpoint,
		SystemId:    system.SystemID,
		UserName:    system.User,
		Password:    system.Password,
		Insecure:    system.ArrayInsecure,
	}
	headers := make(map[string]string)
	headers["Authorization"] = fmt.Sprintf("Bearer %s", adminTknBody.Access)

	err = client.Post(ctx, "/proxy/storage/", headers, nil, &body, nil)
	if err != nil {
		var jsonErr web.JSONError
		if errors.As(err, &jsonErr) {
			if jsonErr.Code == http.StatusUnauthorized {
				var adminTknResp pb.RefreshAdminTokenResponse

				headers["Authorization"] = fmt.Sprintf("Bearer %s", adminTknBody.Refresh)
				err = client.Post(ctx, "/proxy/refresh-admin", headers, nil, &adminTknBody, &adminTknResp)
				if err != nil {
					reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
				}
				// retry with refresh token
				headers["Authorization"] = fmt.Sprintf("Bearer %s", adminTknResp.AccessToken)
				err = client.Post(ctx, "/proxy/storage/", headers, nil, &body, nil)
				if err != nil {
					reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
				}
			} else {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
			}
		} else {
			reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
		}
	}

	return nil
}
