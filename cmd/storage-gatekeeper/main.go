package main

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"expvar"
	"flag"
	"fmt"
	"io"
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
	"github.com/go-redis/redis"
	"google.golang.org/grpc"
)

type CtxKeyToken struct{}

var volReqCount *expvar.Map
var enf *quota.RedisEnforcement

func init() {
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	log.SetFlags(log.LstdFlags | log.Llongfile)

	// Export variables
	volReqCount = expvar.NewMap("volume-req-count")
	expvar.Publish("Goroutines", expvar.Func(func() interface{} {
		return fmt.Sprintf("%d", runtime.NumGoroutine())
	}))
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
		Addr:     "redis.karavi.svc.cluster.local:6379",
		Password: "",
		DB:       0,
	})
	defer func() {
		if err := rdb.Close(); err != nil {
			log.Printf("closing redis: %+v", err)
		}
	}()
	enf = quota.NewRedisEnforcement(context.Background(), rdb)

	log.Printf("Forwarding :443 -> %s://%s", tgt.Scheme, tgt.Host)
	proxy := httputil.NewSingleHostReverseProxy(tgt)

	mux := http.NewServeMux()
	mux.Handle("/policy/", enf.Handler())
	mux.Handle("/debug/", expvar.Handler())
	mux.Handle("/api/", apiMux(rdb, proxy))
	mux.Handle("/proxy/refresh-token/", refreshTokenHandler())
	mux.HandleFunc("/proxy/roles/", func(w http.ResponseWriter, r *http.Request) {
		r, err := http.NewRequest(http.MethodGet, "http://localhost:8181/v1/data/karavi/common/roles", nil)
		if err != nil {
			log.Fatal(err)
		}
		res, err := http.DefaultClient.Do(r)
		if err != nil {
			log.Fatal(err)
		}
		_, err = io.Copy(w, res.Body)
		if err != nil {
			log.Fatal(err)
		}
		defer res.Body.Close()
	})

	http.Handle("/", rootHandler(mux))

	log.Fatal(http.ListenAndServe(":8080", nil))
}

func writeError(w http.ResponseWriter, msg string, code int) error {
	w.WriteHeader(code)
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

func rootHandler(mux http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.URL.Path = cleanPath(r.URL.Path)
		log.Printf("Serving %s %s %v", r.RemoteAddr, r.Method, r.URL.Path)
		mux.ServeHTTP(w, r)
	})
}

func volumeDeleteHandler(proxy http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/action/removeVolume/") {
			// Proxy on if the request is not for removeVolume
			proxy.ServeHTTP(w, r)
			return
		}

		// Extract the volume ID from the request URI in order to get the
		// the name.
		// TODO(ian): have the CSI driver send both name and ID to remove
		// the need for us to figure it out.
		var id string
		z := strings.SplitN(r.URL.Path, "/", 5)
		if len(z) > 3 {
			id = z[3]
		}
		pvName, err := QueryNameByID(id)
		if err != nil {
			writeError(w, "query name by volid", http.StatusInternalServerError)
			return
		}

		// There is no storage pool information passed with this request, forcing
		// us to make extra queries to the array (get volume data, extract pool ID,
		// query pool name).
		// TODO(ian): have the CSI driver send the storage pool.
		// For now, we'll explicitly pass in the ID that we know is true.
		spName, err := QueryStoragePoolNameByID("StoragePool::8633480700000000")
		if err != nil {
			writeError(w, "failed to query pool name from id", http.StatusBadRequest)
			return
		}

		b, err := ioutil.ReadAll(r.Body)
		if err != nil {
			writeError(w, "failed to read body", http.StatusInternalServerError)
			return
		}
		defer r.Body.Close()

		var requestBody map[string]json.RawMessage
		err = json.NewDecoder(bytes.NewReader(b)).Decode(&requestBody)
		if err != nil {
			writeError(w, "decoding request body", http.StatusInternalServerError)
			return
		}
		// Request policy decision from OPA
		ans, err := decision.Can(func() decision.Query {
			return decision.Query{
				Policy: "/karavi/volumes/delete",
				Input: map[string]interface{}{
					"token": TokenFromRequest(r),
				},
			}
		})
		type resp struct {
			Result struct {
				Response struct {
					Allowed bool `json:"allowed"`
					Status  struct {
						Reason string `json:"reason"`
					} `json:"status"`
				} `json:"response"`
				Token struct {
					Group string `json:"group"`
				} `json:"token"`
			} `json:"result"`
		}
		var opaResp resp
		err = json.NewDecoder(bytes.NewReader(ans)).Decode(&opaResp)
		if err != nil {
			writeError(w, "decoding opa request body", http.StatusInternalServerError)
			return
		}
		log.Printf("OPA Response: %v", string(ans))
		if resp := opaResp.Result; !resp.Response.Allowed {
			switch {
			case resp.Token.Group == "":
				writeError(w, "invalid token", http.StatusUnauthorized)
			default:
				writeError(w, fmt.Sprintf("request denied: %v", resp.Response.Status.Reason), http.StatusBadRequest)
			}
			return
		}

		qr := quota.Request{
			StoragePoolID: spName,
			Group:         opaResp.Result.Token.Group,
			VolumeName:    pvName,
		}
		ok, err := enf.DeleteRequest(r.Context(), qr)
		if err != nil {
			writeError(w, "delete request failed", http.StatusInternalServerError)
			return
		}
		if !ok {
			writeError(w, "request denied", http.StatusForbidden)
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
	})
}

func volumeCreateHandler(proxy http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

		// Acquire requested capacity
		body := struct {
			VolumeSizeInKb string `json:"volumeSizeInKb"`
			StoragePoolID  string `json:"storagePoolId"`
		}{}
		err = json.NewDecoder(bytes.NewBuffer(b)).Decode(&body)
		if err != nil {
			writeError(w, "failed to extract cap data", http.StatusBadRequest)
			return
		}

		// TODO(ian): Cache this
		spName, err := QueryStoragePoolNameByID(body.StoragePoolID)
		if err != nil {
			writeError(w, "failed to query pool name from id", http.StatusBadRequest)
			return
		}
		log.Printf("Storagepool: %v -> %v", body.StoragePoolID, spName)
		log.Printf("Namespace: %s", r.Header.Get("X-CSI-PVCNamespace"))
		log.Printf("PVCName: %s", r.Header.Get("X-CSI-PVCName"))

		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			writeError(w, "failed to parse remote host", http.StatusInternalServerError)
			return
		}

		log.Printf("Cluster: %s, PVC: %s, PVCNamespace: %s, PVName: %s, Cap: %s",
			host, r.Header.Get("X-Csi-Pvcname"), r.Header.Get("X-Csi-Pvcnamespace"),
			r.Header.Get("X-Csi-Pvname"), body.VolumeSizeInKb)

		n, err := strconv.ParseInt(body.VolumeSizeInKb, 0, 64)
		if err != nil {
			writeError(w, "failed to parse capacity", http.StatusBadRequest)
			return
		}

		pvName := r.Header.Get("X-Csi-Pvname")
		volReqCount.Add(pvName, 1)

		// TODO Ask OPA to make a decision
		var requestBody map[string]json.RawMessage
		err = json.NewDecoder(bytes.NewReader(b)).Decode(&requestBody)
		if err != nil {
			writeError(w, "decoding request body", http.StatusInternalServerError)
			return
		}
		// Request policy decision from OPA
		ans, err := decision.Can(func() decision.Query {
			return decision.Query{
				Policy: "/karavi/volumes/create",
				Input: map[string]interface{}{
					"token":       TokenFromRequest(r),
					"request":     requestBody,
					"storagepool": spName,
				},
			}
		})
		type resp struct {
			Result struct {
				Response struct {
					Allowed bool `json:"allowed"`
					Status  struct {
						Reason string `json:"reason"`
					} `json:"status"`
				} `json:"response"`
				Token struct {
					Group string `json:"group"`
				} `json:"token"`
				Quota int64 `json:"quota"`
			} `json:"result"`
		}
		var opaResp resp
		err = json.NewDecoder(bytes.NewReader(ans)).Decode(&opaResp)
		if err != nil {
			writeError(w, "decoding opa request body", http.StatusInternalServerError)
			return
		}
		log.Printf("OPA Response: %v", string(ans))
		if resp := opaResp.Result; !resp.Response.Allowed {
			switch {
			case resp.Response.Status.Reason == "":
				writeError(w, "proxy is not configured", http.StatusInternalServerError)
			case resp.Token.Group == "":
				writeError(w, "invalid token", http.StatusUnauthorized)
			default:
				writeError(w, fmt.Sprintf("request denied: %v", resp.Response.Status.Reason), http.StatusBadRequest)
			}
			return
		}

		qr := quota.Request{
			StoragePoolID: spName,
			Group:         opaResp.Result.Token.Group,
			VolumeName:    pvName,
			Capacity:      fmt.Sprintf("%d", n),
		}

		ok, err := enf.ApproveRequest(r.Context(), qr, opaResp.Result.Quota)
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
	})
}

func apiMux(rdb *redis.Client, proxy http.Handler) http.Handler {
	mux := http.NewServeMux()
	// /api/instances/Volume::c3f5f42d00000004/action/removeVolume
	mux.Handle("/api/instances/", volumeDeleteHandler(proxy))
	// Override create volume API
	mux.Handle("/api/types/Volume/instances/", volumeCreateHandler(proxy))
	mux.HandleFunc("/api/login/", func(w http.ResponseWriter, r *http.Request) {
		log.Println("Proxy clients should not be logging in")
	})
	mux.Handle("/", proxy)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authz := r.Header.Get("Authorization")
		parts := strings.Split(authz, " ")
		if len(parts) != 2 {
			log.Println("invalid authz header")
			errorResponse(w, http.StatusBadRequest)
			return
		}
		scheme, token := parts[0], parts[1]

		switch scheme {
		case "Bearer":
			ctx := context.WithValue(r.Context(), CtxKeyToken{}, token)
			r = r.WithContext(ctx)
			setBasicAuth(r)

			// Request policy decision from OPA
			ans, err := decision.Can(func() decision.Query {
				return decision.Query{
					Policy: "/karavi/authz/url",
					Input: map[string]interface{}{
						"method": r.Method,
						"url":    r.URL.Path,
					},
				}
			})
			if err != nil {
				log.Printf("opa: %w", err)
				errorResponse(w, http.StatusInternalServerError)
				return
			}
			var resp struct {
				Result struct {
					Allow bool `json:"allow"`
				} `json:"result"`
			}
			err = json.NewDecoder(bytes.NewReader(ans)).Decode(&resp)
			if err != nil {
				log.Printf("decode json: %w", err)
				errorResponse(w, http.StatusInternalServerError)
				return
			}
			if !resp.Result.Allow {
				log.Println("Request denied")
				errorResponse(w, http.StatusNotFound)
				return
			}
		case "Basic":
			fallthrough
		default:
			// nothing to do
		}

		mux.ServeHTTP(w, r)
	})
}

func refreshTokenHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO(ian): Establish this connection as part of service initialization.
		conn, err := grpc.Dial("github-auth-provider.karavi.svc.cluster.local:50051",
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

func TokenFromRequest(r *http.Request) string {
	v := r.Context().Value(CtxKeyToken{})
	switch t := v.(type) {
	case string:
		return strings.TrimPrefix(t, "Bearer ")
	default:
		panic("TokenFromRequest: type assert to string failed")
	}
}

// QueryStoragePoolNameByID is a workaround to get the SP name from an SIO ID.
// The CSI driver figures this out, but doesn't send us both values so
// we need to repeat the work.
func QueryStoragePoolNameByID(key string) (string, error) {
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

	// TODO(ian): Does this need the prefix or not?
	key = strings.TrimPrefix(key, "StoragePool::")
	pool, err := c.FindStoragePool(key, "", "")
	if err != nil {
		return "", err
	}

	if pool == nil {
		return "", errors.New("No pool")
	}

	return pool.Name, nil
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
