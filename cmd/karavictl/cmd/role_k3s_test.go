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
	"bytes"
	"io"
	"io/ioutil"
	"os"
	"testing"
)

// This test case is intended to run as a subprocess.
func TestK3sRoleSubprocess(t *testing.T) {
	if v := os.Getenv("WANT_GO_TEST_SUBPROCESS"); v != "1" {
		t.Skip("not being run as a subprocess")
	}

	for i, arg := range os.Args {
		if arg == "--" {
			os.Args = os.Args[i+1:]
			break
		}
	}
	defer os.Exit(0)

	returnFile := "testdata/common-configmap.json"

	// k3s commands may be for access roles using 'common' configmap or for storage using the 'karavi-storage-secret' secret
	for _, arg := range os.Args {
		if arg == "secret/karavi-storage-secret" {
			returnFile = "testdata/kubectl_get_secret_storage.json"
		}
	}

	// k3s kubectl [get,create,apply]
	switch os.Args[2] {
	case "get":
		b, err := ioutil.ReadFile(returnFile)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = io.Copy(os.Stdout, bytes.NewReader(b)); err != nil {
			t.Fatal(err)
		}
	case "create":
		// exit code 0
	case "apply":
		// exit code 0
	}
}
