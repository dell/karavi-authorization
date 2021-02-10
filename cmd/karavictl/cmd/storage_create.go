/*
Copyright © 2021 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"os/exec"

	"github.com/dell/goscaleio"
	"github.com/spf13/cobra"
	"sigs.k8s.io/yaml"
)

// createCmd represents the create command
var createCmd = &cobra.Command{
	Use:   "create",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		flagStringValue := func(v string, err error) string {
			if err != nil {
				log.Fatal(err)
			}
			return v
		}
		flagBoolValue := func(v bool, err error) bool {
			if err != nil {
				log.Fatal(err)
			}
			return v
		}

		// Gather the inputs
		var input = struct {
			Type     string
			Endpoint string
			SystemID string
			User     string
			Pass     string
			Insecure bool
		}{
			Type:     flagStringValue(cmd.Flags().GetString("type")),
			Endpoint: flagStringValue(cmd.Flags().GetString("endpoint")),
			SystemID: flagStringValue(cmd.Flags().GetString("system-id")),
			User:     flagStringValue(cmd.Flags().GetString("user")),
			Pass:     flagStringValue(cmd.Flags().GetString("pass")),
			Insecure: flagBoolValue(cmd.Flags().GetBool("insecure")),
		}

		epURL, err := url.Parse(input.Endpoint)
		if err != nil {
			log.Fatal(err)
		}
		epURL.Scheme = "https"

		// Get the current resource

		// TODO(ian): Allow override for testing
		k3sCmd := exec.CommandContext(ctx, "k3s", "kubectl", "get",
			"--namespace=karavi",
			"--output=json",
			"secret/karavi-storage-secret")

		b, err := k3sCmd.Output()
		if err != nil {
			log.Fatal(err)
		}

		base64Systems := struct {
			Data map[string]string
		}{}
		if err := json.Unmarshal(b, &base64Systems); err != nil {
			log.Fatal(err)
		}
		decodedSystems, err := base64.StdEncoding.DecodeString(base64Systems.Data["storage-systems.yaml"])
		if err != nil {
			log.Fatal(err)
		}

		var storage map[string]map[string]map[string]System
		if err := yaml.Unmarshal(decodedSystems, &storage); err != nil {
			log.Fatal(err)
		}

		if storage == nil {
			storage = make(map[string]map[string]map[string]System)
			storage["storage"] = make(map[string]map[string]System)
		}

		// Check that we are not duplicating, no errors, etc.

		fn := func() bool {
			storType, ok := storage["storage"][input.Type]
			if !ok {
				storage["storage"][input.Type] = make(map[string]System)
				return false
			}
			_, ok = storType[input.SystemID]
			return ok
		}

		if fn() {
			log.Fatal("duplicate")
		}

		// Attempt to connect to the storage using the provided details.

		sioClient, err := goscaleio.NewClientWithArgs(epURL.String(), "", true, false)
		if err != nil {
			log.Fatal(err)
		}

		_, err = sioClient.Authenticate(&goscaleio.ConfigConnect{
			Username: input.User,
			Password: input.Pass,
		})
		if err != nil {
			log.Fatal(err)
		}

		_, err = sioClient.FindSystem(input.SystemID, "", "")
		if err != nil {
			log.Fatal(err)
		}

		// Merge the new connection details and apply them.

		types := storage["storage"]
		pfs := types["powerflex"]
		if pfs == nil {
			pfs = make(map[string]System)
		}
		pfs[input.SystemID] = System{
			User:     input.User,
			Pass:     input.Pass,
			Endpoint: input.Endpoint,
			Insecure: input.Insecure,
		}
		types["powerflex"] = pfs

		b, err = yaml.Marshal(&storage)
		if err != nil {
			log.Fatal(err)
		}

		tmpFile, err := ioutil.TempFile("", "karavi")
		if err != nil {
			log.Fatal(err)
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
			log.Fatal(err)
		}

		// kg create -n karavi secret generic karavi-secret
		// 	--from-file=/home/ian/csi-drivers/karavi-authorization/storage-systems.yaml
		//  --from-file=/home/ian/csi-drivers/karavi-authorization/config.yaml -o yaml --dry-run=client | kg apply -f -

		// TODO(ian): Allow override for testing
		crtCmd := exec.CommandContext(ctx, "k3s", "kubectl", "create",
			"--namespace=karavi",
			"secret", "generic", "karavi-storage-secret",
			fmt.Sprintf("--from-file=storage-systems.yaml=%s", tmpFile.Name()),
			"--output=yaml",
			"--dry-run=client")
		// TODO(ian): Allow override for testing
		appCmd := exec.CommandContext(ctx, "k3s", "kubectl", "apply", "-f", "-")

		appCmd.Stdin, err = crtCmd.StdoutPipe()
		if err != nil {
			log.Fatal(err)
		}
		appCmd.Stdout = os.Stdout

		err = appCmd.Start()
		if err != nil {
			log.Fatal(err)
		}
		err = crtCmd.Run()
		if err != nil {
			log.Fatal(err)
		}
		err = appCmd.Wait()
		if err != nil {
			log.Fatal(err)
		}
	},
}

type System struct {
	User     string `yaml:"user"`
	Pass     string `yaml:"pass"`
	Endpoint string `yaml:"endpoint"`
	Insecure bool   `yaml:"insecure"`
}

func init() {
	storageCmd.AddCommand(createCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// createCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// createCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
	createCmd.Flags().StringP("type", "t", "powerflex", "Type of storage system")
	createCmd.Flags().StringP("endpoint", "e", "https://10.0.0.1", "Endpoint of REST API gateway")
	createCmd.Flags().StringP("system-id", "s", "systemid", "System identifier")
	createCmd.Flags().StringP("user", "u", "admin", "Username")
	createCmd.Flags().StringP("pass", "p", "****", "Password")
	createCmd.Flags().BoolP("insecure", "i", false, "Insecure skip verify")
}
