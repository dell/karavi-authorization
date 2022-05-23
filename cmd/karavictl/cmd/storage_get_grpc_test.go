// Copyright © 2022 Dell Inc., or its subsidiaries. All Rights Reserved.
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
	"context"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"karavi-authorization/pb"
	"os"
	"strings"
	"testing"

	"google.golang.org/grpc"
)

func TestStorageGetGrpc(t *testing.T) {
	afterFn := func() {
		CreateStorageServiceClient = createStorageServiceClient
		JSONOutput = jsonOutput
		osExit = os.Exit
	}

	t.Run("it requests getting a storage", func(t *testing.T) {
		defer afterFn()

		getStorageFn := func(context.Context, *pb.StorageGetRequest, ...grpc.CallOption) (*pb.StorageGetResponse, error) {
			storage := `{"User":"admin","Password":"(omitted)","Endpoint":"https://10.0.0.1","Insecure":false}`
			return &pb.StorageGetResponse{Storage: []byte(storage)}, nil
		}

		CreateStorageServiceClient = func(_ string, _ bool) (pb.StorageServiceClient, io.Closer, error) {
			return &fakeStorageServiceClient{GetStorageFn: getStorageFn}, ioutil.NopCloser(nil), nil
		}
		osExit = func(code int) {
		}

		var gotOutput bytes.Buffer

		cmd := NewRootCmd()
		cmd.SetOutput(&gotOutput)
		cmd.SetArgs([]string{"storage", "get", "--addr", "https://storage-service.com", "--system-id", "11e4e7d35817bd0f", "--type", "powerflex", "--insecure"})
		cmd.Execute()

		got := strings.ReplaceAll(gotOutput.String(), "\n", "")
		got = strings.ReplaceAll(got, " ", "")

		want := `{"Endpoint":"https://10.0.0.1","Insecure":false,"Password":"(omitted)","User":"admin"}`
		if want != got {
			t.Errorf("want %s, got \n%s", want, got)
		}
	})
	t.Run("it requires a valid storage server connection", func(t *testing.T) {
		defer afterFn()
		CreateStorageServiceClient = func(_ string, _ bool) (pb.StorageServiceClient, io.Closer, error) {
			return nil, ioutil.NopCloser(nil), errors.New("test error")
		}
		var gotCode int
		done := make(chan struct{})
		osExit = func(code int) {
			gotCode = code
			done <- struct{}{}
			done <- struct{}{} // we can't let this function return
		}
		var gotOutput bytes.Buffer

		cmd := NewRootCmd()
		cmd.SetErr(&gotOutput)
		cmd.SetArgs([]string{"storage", "get", "--addr", "https://storage-service.com", "--system-id", "11e4e7d35817bd0f", "--type", "powerflex", "--insecure"})
		go cmd.Execute()
		<-done

		wantCode := 1
		if gotCode != wantCode {
			t.Errorf("got exit code %d, want %d", gotCode, wantCode)
		}
		var gotErr CommandError
		if err := json.NewDecoder(&gotOutput).Decode(&gotErr); err != nil {
			t.Fatal(err)
		}
		wantErrMsg := "test error"
		if gotErr.ErrorMsg != wantErrMsg {
			t.Errorf("got err %q, want %q", gotErr.ErrorMsg, wantErrMsg)
		}
	})
	t.Run("it handles server errors", func(t *testing.T) {
		defer afterFn()
		CreateStorageServiceClient = func(_ string, _ bool) (pb.StorageServiceClient, io.Closer, error) {
			return &fakeStorageServiceClient{
				GetStorageFn: func(context.Context, *pb.StorageGetRequest, ...grpc.CallOption) (*pb.StorageGetResponse, error) {
					return nil, errors.New("test error")
				},
			}, ioutil.NopCloser(nil), nil
		}
		var gotCode int
		done := make(chan struct{})
		osExit = func(code int) {
			gotCode = code
			done <- struct{}{}
			done <- struct{}{} // we can't let this function return
		}
		var gotOutput bytes.Buffer

		rootCmd := NewRootCmd()
		rootCmd.SetErr(&gotOutput)
		rootCmd.SetArgs([]string{"storage", "get", "--addr", "https://storage-service.com", "--system-id", "testing123", "--type", "powerflex", "--insecure"})

		go rootCmd.Execute()
		<-done

		wantCode := 1
		if gotCode != wantCode {
			t.Errorf("got exit code %d, want %d", gotCode, wantCode)
		}
		var gotErr CommandError
		if err := json.NewDecoder(&gotOutput).Decode(&gotErr); err != nil {
			t.Fatal(err)
		}
		wantErrMsg := "test error"
		if gotErr.ErrorMsg != wantErrMsg {
			t.Errorf("got err %q, want %q", gotErr.ErrorMsg, wantErrMsg)
		}
	})
}
