package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"powerflex-reverse-proxy/internal/decision"
	"strconv"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
)

func init() {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
}

func writeError(w http.ResponseWriter, msg string, code int) error {
	w.WriteHeader(http.StatusForbidden)
	errBody := struct {
		Code       int    `json:"errorCode"`
		StatusCode int    `json:"httpStatusCode"`
		Message    string `json:"message"`
	}{
		Code:       code,
		StatusCode: code,
		Message:    msg,
	}
	err := json.NewEncoder(w).Encode(&errBody)
	if err != nil {
		log.Println("Failed to encode error response", err)
	}
	return nil
}

func main() {
	tgt, err := url.Parse("https://10.247.78.66")
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Forwarding :443 -> %s://%s", tgt.Scheme, tgt.Host)

	proxy := httputil.NewSingleHostReverseProxy(tgt)
	mux := http.NewServeMux()

	// Override handling of volume creation
	mux.HandleFunc("/api/types/Volume/instances", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			proxy.ServeHTTP(w, r)
			return
		}

		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			writeError(w, "failed to read body", http.StatusInternalServerError)
			return
		}
		defer r.Body.Close()

		log.Printf("CreateVolume Request Body: %v", string(b))

		// Acquire requested capacity
		capReq := struct {
			VolumeSizeInKb string `json:"volumeSizeInKb"`
		}{}
		err = json.NewDecoder(bytes.NewBuffer(b)).Decode(&capReq)
		if err != nil {
			writeError(w, "failed to extract cap data", http.StatusBadRequest)
			return
		}
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			writeError(w, "failed to parse remote host", http.StatusInternalServerError)
			return
		}

		log.Printf("Cluster: %s, PVC: %s, PVCNamespace: %s, PVName: %s, Cap: %s",
			host, r.Header.Get("X-Csi-Pvcname"), r.Header.Get("X-Csi-Pvcnamespace"),
			r.Header.Get("X-Csi-Pvname"), capReq.VolumeSizeInKb)

		n, err := strconv.ParseInt(capReq.VolumeSizeInKb, 0, 64)
		if err != nil {
			writeError(w, "failed to parse capacity", http.StatusBadRequest)
			return
		}

		if true {
			// Reset the original request
			r.Body.Close()
			r.Body = ioutil.NopCloser(bytes.NewBuffer(b))
			proxy.ServeHTTP(w, r)
			return
		}

		// Request policy decision from OPA
		ans, err := decision.Can(func() decision.Query {
			return decision.Query{
				Policy: "/dell/create_volume/allow",
				Input: map[string]interface{}{
					"cluster":       host,
					"requested_cap": n,
					"pool":          "mypool",
					"pv_name":       r.Header.Get("X-CSI-PVName"),
					"namespace":     r.Header.Get("X-CSI-PVCNamespace"),
				},
			}
		})
		var resp struct {
			Result struct {
				Result         bool    `json:"result"`
				ProvisionalCap float64 `json:"provisional_cap"`
			} `json:"result"`
		}
		err = json.NewDecoder(bytes.NewBuffer(ans)).Decode(&resp)
		if err != nil {
			writeError(w, "error decoding response", http.StatusInternalServerError)
			return
		}
		if err != nil || !resp.Result.Result {
			writeError(w, "forbidden: exceeded capacity", http.StatusForbidden)
			return
		}

		// At this point the request has been allowed and will forward it
		// on to the proxy.

		// Reset the original request
		r.Body.Close()
		r.Body = ioutil.NopCloser(bytes.NewBuffer(b))

		// We want to capture the responsecode, so pass in a custom
		// ResponseWriter.
		sw := &statusWriter{
			ResponseWriter: w,
		}
		proxy.ServeHTTP(sw, r)

		log.Printf("Resp: Code: %d", sw.status)

		// Hack to increment the used_cap in the OPA data so that
		// we can test eventual denial of the policy without having
		// to implement DeleteVolume.
		if sw.status == http.StatusOK {
			prov_cap := resp.Result.ProvisionalCap
			go func() {
				uri := fmt.Sprintf("/v1/data/dell/quotas/tenants/%s/namespaces/%s",
					host, r.Header.Get("X-CSI-PVCNamespace"))

				req, err := http.NewRequest(http.MethodPatch, "http://localhost:8181"+uri,
					strings.NewReader(fmt.Sprintf(`[{ "op": "replace", "path": "used_cap", "value": %.0f }]`, prov_cap)))
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
				case http.StatusNoContent:
					log.Println("OPA data was updated")
				default:
					log.Println("OPA data failed to be updated:", sc)
					err := json.NewEncoder(os.Stdout).Encode(resp.Body)
					if err != nil {
						log.Printf("error: %+v", err)
						return
					}
					resp.Body.Close()
				}
			}()
		}
	})

	//mux.HandleFunc("/api/login", func(w http.ResponseWriter, r *http.Request) {
	//	log.Println("Proxy clients should not be logging in")
	//})
	mux.Handle("/", proxy)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %v", r.RemoteAddr, r.Method, r.URL.Path)
		authz := r.Header.Get("Authorization")
		log.Println("Authorization", authz)
		parts := strings.Split(authz, " ")
		if len(parts) != 2 {
			log.Println("invalid authz header")
			errorResponse(w, http.StatusBadRequest)
			return
		}
		scheme, token := parts[0], parts[1]

		switch scheme {
		case "Bearer":
			jwtToken, err := jwt.Parse(token, func(t *jwt.Token) (interface{}, error) {
				return []byte("secret"), nil
			})
			if err != nil {
				log.Printf("parsing token: %+v", err)
				errorResponse(w, http.StatusUnauthorized)
				return
			}

			if claims, ok := jwtToken.Claims.(jwt.StandardClaims); ok && jwtToken.Valid {
				log.Printf("time.Now() == %v, ExpiresAt == %v", time.Now().Unix(), claims.ExpiresAt)
				if time.Now().After(time.Unix(claims.ExpiresAt, 0)) {
					log.Println("Expired token")
					errorResponse(w, http.StatusUnauthorized)
					return
				}
			}

			setBasicAuth(r)
		case "Basic":
			fallthrough
		default:
			// nothing to do
		}

		mux.ServeHTTP(w, r)
	})
	//log.Fatal(http.ListenAndServeTLS(":443", "cert.pem", "key.pem", nil))
	log.Fatal(http.ListenAndServe(":8080", nil))
}

type statusWriter struct {
	http.ResponseWriter
	status int
	length int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = 200
	}
	n, err := w.ResponseWriter.Write(b)
	w.length += n
	return n, err
}

func setBasicAuth(req *http.Request) {
	log.Println("Setting basic auth")
	authReq, err := http.NewRequest(http.MethodGet, "https://10.247.78.66/api/login", nil)
	if err != nil {
		panic(err)
	}
	authReq.SetBasicAuth("admin", "Password123")
	resp, err := http.DefaultClient.Do(authReq)
	if err != nil {
		panic(err)
	}
	if resp.StatusCode > 299 {
		panic(err)
	}
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()
	b = bytes.Trim(b, "\"")

	req.SetBasicAuth("", string(b))
}

func errorResponse(w io.Writer, httpCode int) {
	resp := struct {
		Message        string `json:"message"`
		HttpStatusCode int    `json:"httpStatusCode"`
		ErrorCode      int    `json:"errorCode"`
	}{
		Message:        http.StatusText(httpCode),
		HttpStatusCode: httpCode,
		ErrorCode:      0,
	}
	err := json.NewEncoder(w).Encode(&resp)
	if err != nil {
		switch t := w.(type) {
		case http.ResponseWriter:
			http.Error(t, "encoding failure", http.StatusInternalServerError)
		default:
			log.Printf("encoding failed: %+v", err)
		}
		return
	}
}
