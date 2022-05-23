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
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"karavi-authorization/pb"
	"net/url"
	"os"
	"strings"
	"syscall"

	pscale "github.com/dell/goisilon"
	pmax "github.com/dell/gopowermax"
	"github.com/dell/gopowermax/types/v90"
	"github.com/dell/goscaleio"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
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
	User     string `yaml:"user"`
	Password string `yaml:"password"`
	Endpoint string `yaml:"endpoint"`
	Insecure bool   `yaml:"insecure"`
}

// SystemID wraps a system ID to be a quoted string because system IDs could be all numbers
// which will cause issues with yaml marshalers
type SystemID struct {
	Value string
}

func (id SystemID) String() string {
	return fmt.Sprintf("%q", strings.ReplaceAll(id.Value, `"`, ""))
}

var supportedStorageTypes = map[string]struct{}{
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
		Run: func(cmd *cobra.Command, args []string) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

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
			var input = struct {
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

			addr, err := cmd.Flags().GetString("addr")
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf(outFormat, err))
			}
			insecure, err := cmd.Flags().GetBool("insecure")
			if err != nil {
				reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf(outFormat, err))
			}

			if addr != "" {
				// if addr flag is specified, make a grpc request
				if err := doStorageCreateRequest(addr, input, insecure); err != nil {
					reportErrorAndExit(JSONOutput, cmd.ErrOrStderr(), fmt.Errorf(outFormat, err))
				}
			} else {

				// Parse the URL and prepare for a password prompt.
				urlWithUser, err := url.Parse(input.Endpoint)
				if err != nil {
					errAndExit(err)
				}

				urlWithUser.Scheme = "https"
				urlWithUser.User = url.User(input.User)

				// If the password was not provided...
				prompt := fmt.Sprintf("Enter password for %v: ", urlWithUser)
				// If the password was not provided...
				if pf := cmd.Flags().Lookup("password"); !pf.Changed {
					// Get password from stdin
					readPassword(cmd.ErrOrStderr(), prompt, &input.Password)
				}

				// Sanitize the endpoint
				epURL, err := url.Parse(input.Endpoint)
				if err != nil {
					errAndExit(err)
				}
				epURL.Scheme = "https"

				// Get the current list of registered storage systems
				k3sCmd := execCommandContext(ctx, K3sPath, "kubectl", "get",
					"--namespace=karavi",
					"--output=json",
					"secret/karavi-storage-secret")

				b, err := k3sCmd.Output()
				if err != nil {
					errAndExit(err)
				}
				base64Systems := struct {
					Data map[string]string
				}{}
				if err := json.Unmarshal(b, &base64Systems); err != nil {
					errAndExit(err)
				}
				decodedSystems, err := base64.StdEncoding.DecodeString(base64Systems.Data["storage-systems.yaml"])
				if err != nil {
					errAndExit(err)
				}

				var listData map[string]Storage
				if err := yaml.Unmarshal(decodedSystems, &listData); err != nil {
					errAndExit(err)
				}
				if listData == nil || listData["storage"] == nil {
					listData = make(map[string]Storage)
					listData["storage"] = make(Storage)
				}
				var storage = listData["storage"]
				// Check that we are not duplicating, no errors, etc.

				if _, ok := supportedStorageTypes[input.Type]; !ok {
					errAndExit(fmt.Errorf("unsupported type: %s", input.Type))
				}

				sysIDs := strings.Split(input.SystemID, ",")
				isDuplicate := func() (string, bool) {
					storType, ok := storage[input.Type]
					if !ok {
						storage[input.Type] = make(map[string]System)
						return "", false
					}
					for _, id := range sysIDs {
						if _, ok = storType[fmt.Sprintf(id)]; ok {
							return id, true
						}
					}
					return "", false
				}

				if id, result := isDuplicate(); result {
					fmt.Fprintf(cmd.ErrOrStderr(), "error: %s system with ID %s is already registered\n", input.Type, id)
					osExit(1)
				}

				// Attempt to connect to the storage using the provided details.
				// TODO(ian): This logic should ideally be performed remotely, not
				// in the client.

				var tempStorage SystemType

				switch input.Type {
				case powerflex:
					tempStorage = storage[powerflex]
					if tempStorage == nil {
						tempStorage = make(map[string]System)
					}

					sioClient, err := goscaleio.NewClientWithArgs(epURL.String(), "", true, false)
					if err != nil {
						errAndExit(err)
					}

					_, err = sioClient.Authenticate(&goscaleio.ConfigConnect{
						Username: input.User,
						Password: input.Password,
					})
					if err != nil {
						errAndExit(err)
					}

					resp, err := sioClient.FindSystem(input.SystemID, "", "")
					if err != nil {
						errAndExit(err)
					}
					if resp.System.ID != input.SystemID {
						fmt.Fprintf(cmd.ErrOrStderr(), "system id %q not found", input.SystemID)
						osExit(1)
					}

					storageID := strings.Trim(SystemID{Value: input.SystemID}.String(), "\"")
					tempStorage[storageID] = System{
						User:     input.User,
						Password: input.Password,
						Endpoint: input.Endpoint,
						Insecure: input.ArrayInsecure,
					}

				case powermax:
					tempStorage = storage[powermax]
					if tempStorage == nil {
						tempStorage = make(map[string]System)
					}

					pmClient, err := pmax.NewClientWithArgs(epURL.String(), "", "karavi-auth", true, false)
					if err != nil {
						errAndExit(err)
					}

					configConnect := &pmax.ConfigConnect{
						Endpoint: input.Endpoint,
						Version:  "",
						Username: input.User,
						Password: input.Password,
					}
					err = pmClient.Authenticate(ctx, configConnect)
					if err != nil {
						errAndExit(err)
					}

					var powermaxSymmetrix []*types.Symmetrix

					symmetrixIDList, err := pmClient.GetSymmetrixIDList(ctx)
					if err != nil {
						errAndExit(err)
					}
					for _, s := range symmetrixIDList.SymmetrixIDs {
						symmetrix, err := pmClient.GetSymmetrixByID(ctx, s)
						if err != nil {
							errAndExit(err)
						}
						if strings.Contains(symmetrix.Model, "PowerMax") || strings.Contains(symmetrix.Model, "VMAX") {
							powermaxSymmetrix = append(powermaxSymmetrix, symmetrix)
						}
					}

					createStorageFunc := func(id string) {
						tempStorage[id] = System{
							User:     input.User,
							Password: input.Password,
							Endpoint: input.Endpoint,
							Insecure: input.ArrayInsecure,
						}
					}

					for _, p := range powermaxSymmetrix {
						storageID := strings.Trim(SystemID{Value: p.SymmetrixID}.String(), "\"")
						if input.SystemID != "" {
							if len(sysIDs) > 0 {
								if contains(p.SymmetrixID, sysIDs) {
									createStorageFunc(storageID)
								}
								continue
							}
						}
						createStorageFunc(storageID)
					}

				case powerscale:
					tempStorage = storage[powerscale]
					if tempStorage == nil {
						tempStorage = make(map[string]System)
					}

					psClient, err := pscale.NewClientWithArgs(context.Background(), epURL.String(), input.ArrayInsecure, 1, input.User, "Administrators", input.Password, "", "777", 0)
					if err != nil {
						errAndExit(err)
					}

					clusterConfig, err := psClient.GetClusterConfig(context.Background())
					if err != nil {
						errAndExit(err)
					}

					if clusterConfig.Name != input.SystemID {
						fmt.Fprintf(cmd.ErrOrStderr(), "cluster name %q not found", input.SystemID)
						osExit(1)
					}

					tempStorage[input.SystemID] = System{
						User:     input.User,
						Password: input.Password,
						Endpoint: input.Endpoint,
						Insecure: input.ArrayInsecure,
					}

				default:
					errAndExit(fmt.Errorf("invalid storage array type given"))
				}

				// Merge the new connection details and apply them.

				storage[input.Type] = tempStorage
				listData["storage"] = storage

				b, err = yaml.Marshal(&listData)
				if err != nil {
					errAndExit(err)
				}

				tmpFile, err := ioutil.TempFile("", "karavi")
				if err != nil {
					errAndExit(err)
				}
				defer func() {
					if err := tmpFile.Close(); err != nil {
						fmt.Fprintf(os.Stderr, "error: %+v\n", err)
					}
					if err := os.Remove(tmpFile.Name()); err != nil {
						fmt.Fprintf(os.Stderr, "error: %+v\n", err)
					}
				}()
				_, err = tmpFile.WriteString(string(b))
				if err != nil {
					errAndExit(err)
				}

				crtCmd := execCommandContext(ctx, K3sPath, "kubectl", "create",
					"--namespace=karavi",
					"secret", "generic", "karavi-storage-secret",
					fmt.Sprintf("--from-file=storage-systems.yaml=%s", tmpFile.Name()),
					"--output=yaml",
					"--dry-run=client")
				appCmd := execCommandContext(ctx, K3sPath, "kubectl", "apply", "-f", "-")

				if err := pipeCommands(crtCmd, appCmd); err != nil {
					errAndExit(err)
				}

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

func contains(s string, slice []string) bool {
	for _, v := range slice {
		if s == v {
			return true
		}
	}
	return false
}

type input struct {
	Type          string
	Endpoint      string
	SystemID      string
	User          string
	Password      string
	ArrayInsecure bool
}

func doStorageCreateRequest(addr string, system input, grpcInsecure bool) error {

	client, conn, err := CreateStorageServiceClient(addr, grpcInsecure)
	if err != nil {
		return err
	}
	defer conn.Close()

	req := &pb.StorageCreateRequest{
		StorageType: system.Type,
		Endpoint:    system.Endpoint,
		SystemId:    system.SystemID,
		UserName:    system.User,
		Password:    system.Password,
		Insecure:    system.ArrayInsecure,
	}

	_, err = client.Create(context.Background(), req)
	if err != nil {
		return err
	}

	return nil
}
