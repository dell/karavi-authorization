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
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"powerflex-reverse-proxy/internal/decision"
	"powerflex-reverse-proxy/internal/github"
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
	mux.Handle("/proxy/auth/login/", &github.Handler{})
	mux.Handle("/api/", apiMux(proxy))
	mux.HandleFunc("/proxy/refresh-token/", func(w http.ResponseWriter, r *http.Request) {
		// Verify refresh token
		// Check if refresh token has been revoked! (possibly in a goroutine?)
		//   - we can perform this check via OPA as it should already have
		//     the external data in-memory.
		// Verify access token (should be expired)
		// Sign new access token
		// Return new access token

		// To create a new refresh token, the user will have to re-authn.
		// This will cause a new refresh token to be

		log.Println("Refreshing token!")
		type tokenPair struct {
			RefreshToken string `json:"refreshToken"`
			AccessToken  string `json:"accessToken"`
		}
		var input tokenPair
		err := json.NewDecoder(r.Body).Decode(&input)
		if err != nil {
			log.Printf("decoding token pair: %+v", err)
			http.Error(w, "decoding token pair", http.StatusInternalServerError)
			return
		}

		var refreshClaims jwt.StandardClaims
		refresh, err := jwt.ParseWithClaims(input.RefreshToken, &refreshClaims, func(t *jwt.Token) (interface{}, error) {
			return []byte("secret"), nil
		})
		if err != nil {
			log.Printf("parsing refresh token: %+v", err)
			http.Error(w, "parsing refresh token", http.StatusInternalServerError)
			return
		}
		// TODO(ian): Check revoked status on refresh token.
		// TODO(ian): Inc the refresh count on refresh token vs generating a new one?
		_ = refresh

		var accessClaims jwt.StandardClaims
		access, err := jwt.ParseWithClaims(input.AccessToken, &accessClaims, func(t *jwt.Token) (interface{}, error) {
			return []byte("secret"), nil
		})
		if access.Valid {
			log.Println("access token was valid")
			return
		} else if ve, ok := err.(*jwt.ValidationError); ok {
			switch {
			case ve.Errors&jwt.ValidationErrorExpired != 0:
				log.Println("access token is expired, but that's ok...continue!")
			default:
				log.Printf("parsing access token: %+v", err)
				http.Error(w, "parsing access token", http.StatusInternalServerError)
				return
			}
		}
		_ = access

		claims := jwt.StandardClaims{
			Audience:  accessClaims.Audience,
			ExpiresAt: time.Now().Add(30 * time.Second).Unix(),
		}
		newRefresh := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		newAccess := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

		newRefreshStr, err := newRefresh.SignedString([]byte("secret"))
		if err != nil {
			log.Printf("signing refresh: %+v", err)
			http.Error(w, "signing refresh token", http.StatusInternalServerError)
			return
		}
		newAccessStr, err := newAccess.SignedString([]byte("secret"))
		if err != nil {
			log.Printf("signing access token: %+v", err)
			http.Error(w, "signing access token", http.StatusInternalServerError)
			return
		}

		var output tokenPair
		output.RefreshToken = newRefreshStr
		output.AccessToken = newAccessStr
		err = json.NewEncoder(w).Encode(&output)
		if err != nil {
			log.Printf("encoding token pair: %+v", err)
			http.Error(w, "encoding token pair", http.StatusInternalServerError)
			return
		}
	})

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = cleanPath(r.URL.Path)
		log.Printf("%s %s %v", r.RemoteAddr, r.Method, r.URL.Path)
		mux.ServeHTTP(w, r)
	})

	//log.Fatal(http.ListenAndServeTLS(":443", "cert.pem", "key.pem", nil))
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func apiMux(proxy http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/types/Volume/instances/", func(w http.ResponseWriter, r *http.Request) {
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
	mux.HandleFunc("/api/login/", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Proxy clients should not be logging in")
	})
	mux.Handle("/", proxy)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

		log.Println("apiMux is serving", r.URL.Path)
		mux.ServeHTTP(w, r)
	})
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

func errorResponse(w http.ResponseWriter, httpCode int) {
	resp := struct {
		Message        string `json:"message"`
		HttpStatusCode int    `json:"httpStatusCode"`
		ErrorCode      int    `json:"errorCode"`
	}{
		Message:        http.StatusText(httpCode),
		HttpStatusCode: httpCode,
		ErrorCode:      0,
	}
	w.WriteHeader(httpCode)
	err := json.NewEncoder(w).Encode(&resp)
	if err != nil {
		log.Printf("encoding failed: %+v", err)
		http.Error(w, "encoding failure", http.StatusInternalServerError)
		return
	}
}

func cleanPath(pth string) string {
	pth = path.Clean("/" + pth)
	if pth[len(pth)-1] != '/' {
		pth = pth + "/"
	}
	return pth
}

func splitPath(pth string) (head, tail string) {
	pth = cleanPath(pth)
	parts := strings.SplitN(pth[1:], "/", 2)
	if len(parts) < 2 {
		parts = append(parts, "/")
	}
	return parts[0], cleanPath(parts[1])
}

func stripPrefix(s string, h http.Handler) (string, http.Handler) {
	return s, http.StripPrefix(strings.TrimSuffix(s, "/"), h)
}
