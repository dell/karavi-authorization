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

package decision

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"
)

type Query struct {
	Host   string
	Policy string                 `json:"-"`
	Input  map[string]interface{} `json:"input"`
}

func Can(fn func() Query) ([]byte, error) {
	// TODO(ian): Need a context here for tracing.
	// Query:
	//
	//{
	//  "input": {
	//    "capacity": 100,
	//    "cluster": "devops1",
	//    "pool": "mypool",
	//    "pv_name": "pv-123",
	//    "pvc_namespace": "apps"
	//  }
	//}
	// curl -v -d @query-create-volume.json localhost:8181/v1/data/dell/policy/allow

	var b bytes.Buffer
	q := fn()
	err := json.NewEncoder(&b).Encode(&q)
	if err != nil {
		return nil, err
	}

	u, err := url.Parse("http://" + q.Host + "/v1/data" + q.Policy)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, u.String(), &b)
	if err != nil {
		return nil, err
	}

	http.DefaultClient.Timeout = 10 * time.Second
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	respBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return respBytes, nil
}
