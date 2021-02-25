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
	"net/url"
	"os"
	"syscall"

	"github.com/dell/goscaleio"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"sigs.k8s.io/yaml"
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

var supportedStorageTypes = map[string]struct{}{
	"powerflex": {},
}

// storageCreateCmd represents the create command
var storageCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Create and register a storage system.",
	Long:  `Creates and registers a storage system.`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

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

		// Gather the inputs
		var input = struct {
			Type     string
			Endpoint string
			SystemID string
			User     string
			Password string
			Insecure bool
		}{
			Type:     flagStringValue(cmd.Flags().GetString("type")),
			Endpoint: flagStringValue(cmd.Flags().GetString("endpoint")),
			SystemID: flagStringValue(cmd.Flags().GetString("system-id")),
			User:     flagStringValue(cmd.Flags().GetString("user")),
			Password: flagStringValue(cmd.Flags().GetString("password")),
			Insecure: flagBoolValue(cmd.Flags().GetBool("insecure")),
		}

		// If the password was not provided...
		if pf := cmd.Flags().Lookup("password"); !pf.Changed {
			// Get password from stdin
			readPassword(cmd.ErrOrStderr(), int(syscall.Stdin), &input.Password)
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

		isDuplicate := func() bool {
			storType, ok := storage[input.Type]
			if !ok {
				storage[input.Type] = make(map[string]System)
				return false
			}
			_, ok = storType[input.SystemID]
			return ok
		}

		if isDuplicate() {
			fmt.Fprintf(cmd.ErrOrStderr(), "error: %s system with ID %s is already registered\n", input.Type, input.SystemID)
			osExit(1)
		}

		// Attempt to connect to the storage using the provided details.
		// TODO(ian): This logic should ideally be performed remotely, not
		// in the client.

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

		// Merge the new connection details and apply them.

		pfs := storage["powerflex"]
		if pfs == nil {
			pfs = make(map[string]System)
		}
		pfs[input.SystemID] = System{
			User:     input.User,
			Password: input.Password,
			Endpoint: input.Endpoint,
			Insecure: input.Insecure,
		}
		storage["powerflex"] = pfs
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
	},
}

func init() {
	storageCmd.AddCommand(storageCreateCmd)

	storageCreateCmd.Flags().StringP("type", "t", "powerflex", "Type of storage system")
	storageCreateCmd.Flags().StringP("endpoint", "e", "https://10.0.0.1", "Endpoint of REST API gateway")
	storageCreateCmd.Flags().StringP("system-id", "s", "systemid", "System identifier")
	storageCreateCmd.Flags().StringP("user", "u", "admin", "Username")
	storageCreateCmd.Flags().StringP("password", "p", "", "Specify password, or omit to use stdin")
	storageCreateCmd.Flags().BoolP("insecure", "i", false, "Insecure skip verify")
}

func readPassword(w io.Writer, in int, p *string) {
	fmt.Fprintf(w, "Enter password: ")
	b, err := term.ReadPassword(in)
	if err != nil {
		reportErrorAndExit(JSONOutput, w, err)
	}
	fmt.Fprintln(w)
	*p = string(b)
}
