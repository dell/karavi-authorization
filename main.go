package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"powerflex-reverse-proxy/internal/decision"
	"strconv"
	"strings"
)

func init() {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
}

func main() {
	tgt, err := url.Parse("https://10.247.78.66")
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Forwarding :443 -> %s://%s", tgt.Scheme, tgt.Host)

	proxy := httputil.NewSingleHostReverseProxy(tgt)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/types/Volume/instances", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			proxy.ServeHTTP(w, r)
			return
		}
		log.Println("Creating a volume!")

		if false {
			w.WriteHeader(http.StatusForbidden)
			errBody := struct {
				Code       int    `json:"errorCode"`
				StatusCode int    `json:"httpStatusCode"`
				Message    string `json:"message"`
			}{
				Code:       http.StatusForbidden,
				StatusCode: http.StatusForbidden,
				Message:    "not enough quota left",
			}
			err := json.NewEncoder(w).Encode(&errBody)
			if err != nil {
				log.Println("Failed to encode error response", err)
			}
			return
		}

		log.Printf("r = %+v\n", r)

		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "failed to read body", http.StatusInternalServerError)
			return
		}
		log.Printf("Body: %v", string(b))

		log.Println("Validation time!")
		capReq := struct {
			VolumeSizeInKb string `json:"volumeSizeInKb"`
		}{}
		err = json.NewDecoder(bytes.NewBuffer(b)).Decode(&capReq)
		if err != nil {
			http.Error(w, "failed to extract cap data", http.StatusBadRequest)
			return
		}
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			http.Error(w, "failed to parse remote host", http.StatusInternalServerError)
			return
		}

		log.Printf("Cluster: %s, PVC: %s, PVCNamespace: %s, PVName: %s, Cap: %s",
			host, r.Header.Get("X-Csi-Pvcname"), r.Header.Get("X-Csi-Pvcnamespace"),
			r.Header.Get("X-Csi-Pvname"), capReq.VolumeSizeInKb)

		n, err := strconv.ParseInt(capReq.VolumeSizeInKb, 0, 64)
		if err != nil {
			http.Error(w, "failed to parse capacity", http.StatusBadRequest)
			return
		}

		ans, err := decision.Can(func() decision.Query {
			return decision.Query{
				Policy: "/dell/policy/allow",
				Input: map[string]interface{}{
					"cluster":       host,
					"capacity":      n,
					"pool":          "mypool",
					"pv_name":       r.Header.Get("X-CSI-PVName"),
					"pvc_namespace": r.Header.Get("X-CSI-PVCNamespace"),
				},
			}
		})
		if err != nil || !ans {
			log.Println("Forbidden")
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}

		go func() {
			log.Println("Updating the local data in OPA")
			uri := fmt.Sprintf("/v1/data/dell/quotas/tenants/%s/namespaces/%s",
				host, r.Header.Get("X-CSI-PVCNamespace"))

			req, err := http.NewRequest(http.MethodPatch, "http://localhost:8181"+uri,
				strings.NewReader(fmt.Sprintf(`[{ "op": "add", "path": "capacity_quota_in_kb", "value": %d }]`, 0)))
			if err != nil {
				log.Printf("error: %+v", err)
				return
			}

			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				log.Printf("error: %+v", err)
				return
			}

			switch sc := resp.StatusCode; sc {
			case http.StatusAccepted:
				log.Println("Update was successful")
			default:
				log.Println("Failed with response code", sc)
				err := json.NewEncoder(os.Stdout).Encode(resp.Body)
				if err != nil {
					log.Printf("error: %+v", err)
					return
				}
				resp.Body.Close()
			}
		}()

		// Reset the original request
		r.Body.Close()
		r.Body = ioutil.NopCloser(bytes.NewBuffer(b))

		// We want to capture the response, so first pass in a recorder
		// then we'll use it for the real response later.
		rec := httptest.NewRecorder()
		proxy.ServeHTTP(rec, r)

		log.Printf("Resp: Code: %d", rec.Code)

		// Now let's use it for the real response...
		for k, v := range rec.HeaderMap {
			w.Header()[k] = v
		}
		w.WriteHeader(rec.Code)
		rec.Body.WriteTo(w)
	})
	mux.Handle("/", proxy)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %v", r.Method, r.URL.Path)
		mux.ServeHTTP(w, r)
	})
	log.Fatal(http.ListenAndServeTLS(":443", "cert.pem", "key.pem", nil))
}
