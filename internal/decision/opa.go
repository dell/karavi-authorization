package decision

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"
)

type Query struct {
	Policy string                 `json:"-"`
	Input  map[string]interface{} `json:"input"`
}

func Can(fn func() Query) (bool, error) {
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
		return false, err
	}

	u, err := url.Parse("http://localhost:8181/v1/data" + q.Policy)
	if err != nil {
		return false, err
	}
	req, err := http.NewRequest(http.MethodPost, u.String(), &b)
	if err != nil {
		return false, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false, err
	}

	m := map[string]bool{}
	json.NewDecoder(resp.Body).Decode(&m)
	return m["result"], nil
}
