package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"errors"
	"expvar"
	"flag"
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
	"powerflex-reverse-proxy/internal/quota"
	"powerflex-reverse-proxy/pb"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/dell/goscaleio"
	"github.com/dgrijalva/jwt-go"
	"github.com/go-redis/redis"
	"google.golang.org/grpc"
)

var volReqCount *expvar.Map

func init() {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	log.SetFlags(log.LstdFlags | log.Llongfile)

	// Export variables
	volReqCount = expvar.NewMap("volume-req-count")
	expvar.Publish("Goroutines", expvar.Func(func() interface{} {
		return fmt.Sprintf("%d", runtime.NumGoroutine())
	}))
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
		return err
	}
	return nil
}

func main() {
	var (
		powerFlexAddress        string
		defaultPowerFlexAddress = "https://10.247.78.66"
	)

	e := strings.TrimSpace(os.Getenv("POWERFLEX_ADDRESS"))
	if len(e) >= 1 {
		defaultPowerFlexAddress = e
	}

	fs := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	fs.StringVar(&powerFlexAddress, "pfa", defaultPowerFlexAddress, "PowerFlex address and port <host>:<port>")
	err := fs.Parse(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}

	tgt, err := url.Parse(powerFlexAddress)
	if err != nil {
		log.Fatal(err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr:     "redis.default.svc.cluster.local:6379",
		Password: "",
		DB:       0,
	})
	defer func() {
		if err := rdb.Close(); err != nil {
			log.Printf("closing redis: %+v", err)
		}
	}()

	log.Printf("Forwarding :443 -> %s://%s", tgt.Scheme, tgt.Host)

	proxy := httputil.NewSingleHostReverseProxy(tgt)

	mux := http.NewServeMux()
	mux.Handle("/debug/", expvar.Handler())
	mux.Handle("/api/", apiMux(rdb, proxy))
	mux.Handle("/proxy/refresh-token/", refreshTokenHandler())

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = cleanPath(r.URL.Path)
		log.Printf("%s %s %v", r.RemoteAddr, r.Method, r.URL.Path)
		mux.ServeHTTP(w, r)
	})

	log.Fatal(http.ListenAndServe(":8080", nil))
}

// QueryNameByID is a workaround to get the PV name from an SIO ID.
// The CSI driver figures this out, but doesn't send us both values so
// we need to repeat the work.
func QueryNameByID(key string) (string, error) {
	c, err := goscaleio.NewClientWithArgs("https://10.247.78.66", "", false, false)
	if err != nil {
		return "", err
	}
	_, err = c.Authenticate(&goscaleio.ConfigConnect{
		Username: "admin",
		Password: "Password123",
	})
	if err != nil {
		return "", err
	}

	key = strings.TrimPrefix(key, "Volume::")
	vols, err := c.GetVolume("", key, "", "", false)
	if err != nil {
		return "", err
	}

	if len(vols) == 0 {
		return "", errors.New("No volume")
	}

	return vols[0].Name, nil
}

func apiMux(rdb *redis.Client, proxy http.Handler) http.Handler {
	mux := http.NewServeMux()
	// /api/instances/Volume::c3f5f42d00000004/action/removeVolume
	mux.HandleFunc("/api/instances/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/action/removeVolume/"):
			var id string
			z := strings.SplitN(r.URL.Path, "/", 5)
			if len(z) > 3 {
				id = z[3]
			}
			log.Println("BIBBY Trying to handle delete for", id)
			pvName, err := QueryNameByID(id)
			if err != nil {
				writeError(w, "ff", http.StatusInternalServerError)
				return
			}
			log.Println("BIBBY The PV name is", pvName)

			// TODO Ask OPA to make a decision
			enf := quota.NewRedisEnforcement(r.Context(), rdb)
			qr := quota.Request{
				StoragePool: "pool1",
				TenantID:    "tenant",
				VolumeName:  pvName,
			}
			ok, err := enf.DeleteRequest(r.Context(), qr)
			if err != nil {
				writeError(w, "failed to approve request", http.StatusInternalServerError)
				return
			}
			if !ok {
				writeError(w, "request denied", http.StatusInsufficientStorage)
				return
			}

			sw := &statusWriter{
				ResponseWriter: w,
			}
			proxy.ServeHTTP(sw, r)

			log.Printf("Resp: Code: %d", sw.status)
			switch sw.status {
			case http.StatusOK:
				log.Println("Publish deleted")
				ok, err := enf.PublishDeleted(r.Context(), qr)
				if err != nil {
					log.Printf("publish failed: %+v", err)
					return
				}
				log.Println("Result of publish:", ok)
			default:
				log.Println("Non 200 response, nothing to publish")
			}
			return
		default:
			proxy.ServeHTTP(w, r)
		}
	})
	// Override create volume API
	mux.HandleFunc("/api/types/Volume/instances/", func(w http.ResponseWriter, r *http.Request) {
		// StripPrefix won't work here, because we are proxying the request and must
		// leave the request path intact.
		if r.URL.Path != "/api/types/Volume/instances/" {
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

		pvName := r.Header.Get("X-Csi-Pvname")
		volReqCount.Add(pvName, 1)

		// TODO Ask OPA to make a decision
		enf := quota.NewRedisEnforcement(r.Context(), rdb)
		qr := quota.Request{
			StoragePool: "pool1",
			TenantID:    "tenant",
			VolumeName:  pvName,
			Capacity:    fmt.Sprintf("%d", n),
		}
		ok, err := enf.ApproveRequest(r.Context(), qr, 100_000_000)
		if err != nil {
			writeError(w, "failed to approve request", http.StatusInternalServerError)
			return
		}
		if !ok {
			writeError(w, "request denied", http.StatusInsufficientStorage)
			return
		}

		// Reset the original request
		r.Body.Close()
		r.Body = ioutil.NopCloser(bytes.NewBuffer(b))
		sw := &statusWriter{
			ResponseWriter: w,
		}

		proxy.ServeHTTP(sw, r)

		log.Printf("Resp: Code: %d", sw.status)
		switch sw.status {
		case http.StatusOK:
			log.Println("Publish created")
			ok, err := enf.PublishCreated(r.Context(), qr)
			if err != nil {
				log.Printf("publish failed: %+v", err)
				return
			}
			log.Println("Result of publish:", ok)
		default:
			log.Println("Non 200 response, nothing to publish")
		}

		if true {
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
		sw = &statusWriter{
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

func refreshTokenHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO(ian): Establish this connection as part of service initialization.
		conn, err := grpc.Dial("github-auth-provider.default.svc.cluster.local:50051",
			grpc.WithTimeout(10*time.Second),
			grpc.WithInsecure())
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer conn.Close()

		client := pb.NewAuthServiceClient(conn)

		log.Println("Refreshing token!")
		type tokenPair struct {
			RefreshToken string `json:"refreshToken,omitempty"`
			AccessToken  string `json:"accessToken"`
		}
		var input tokenPair
		err = json.NewDecoder(r.Body).Decode(&input)
		if err != nil {
			log.Printf("decoding token pair: %+v", err)
			http.Error(w, "decoding token pair", http.StatusInternalServerError)
			return
		}

		refreshResp, err := client.Refresh(r.Context(), &pb.RefreshRequest{
			AccessToken:  input.AccessToken,
			RefreshToken: input.RefreshToken,
		})
		if err != nil {
			log.Printf("%+v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var output tokenPair
		output.AccessToken = refreshResp.AccessToken
		err = json.NewEncoder(w).Encode(&output)
		if err != nil {
			log.Printf("encoding token pair: %+v", err)
			http.Error(w, "encoding token pair", http.StatusInternalServerError)
			return
		}
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
