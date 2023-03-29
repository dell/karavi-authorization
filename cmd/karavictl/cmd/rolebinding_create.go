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
	"karavi-authorization/internal/proxy"
	"strings"

	"github.com/spf13/cobra"
)

// NewCreateRoleBindingCmd creates a new rolebinding command
func NewCreateRoleBindingCmd() *cobra.Command {
	createRoleBindingCmd := &cobra.Command{
		Use:   "create",
		Short: "Create a rolebinding between role and tenant",
		Long:  `Creates a rolebinding between role and tenant`,
		Run: func(cmd *cobra.Command, args []string) {
			addr, err := cmd.Flags().GetString("addr")
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
			}

			insecure, err := cmd.Flags().GetBool("insecure")
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
			}

			tenant, err := cmd.Flags().GetString("tenant")
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
			}
			role, err := cmd.Flags().GetString("role")
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
			}

			if strings.TrimSpace(tenant) == "" {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), errors.New("no tenant input provided"))
			}

			if strings.TrimSpace(role) == "" {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), errors.New("no role input provided"))
			}

			client, err := CreateHttpClient(fmt.Sprintf("https://%s", addr), insecure)
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
			}

			body := proxy.BindRoleBody{
				Tenant: tenant,
				Role:   role,
			}
			err = client.Post(context.Background(), "/proxy/tenant/bind", nil, nil, &body, nil)
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), err)
			}
		},
	}

	createRoleBindingCmd.Flags().StringP("tenant", "t", "", "Tenant name")
	err := createRoleBindingCmd.MarkFlagRequired("tenant")
	if err != nil {
		reportErrorAndExit(JSONOutput, createRoleBindingCmd.ErrOrStderr(), err)
	}
	createRoleBindingCmd.Flags().StringP("role", "r", "", "Role name")
	err = createRoleBindingCmd.MarkFlagRequired("role")
	if err != nil {
		reportErrorAndExit(JSONOutput, createRoleBindingCmd.ErrOrStderr(), err)
	}

	return createRoleBindingCmd
}
