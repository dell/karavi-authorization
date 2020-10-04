package github

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/dgrijalva/jwt-go"
)

const (
	ClientID = "b016f6f31210082e52c2"
	Scope    = "user"
)

type Handler struct {
	once       sync.Once
	mux        *http.ServeMux
	httpClient *http.Client
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.once.Do(func() {
		h.httpClient = &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{
					InsecureSkipVerify: true,
				},
			},
			Timeout: 30 * time.Second,
		}

		h.initRoutes()
	})
	h.mux.ServeHTTP(w, r)
}

func (h *Handler) initRoutes() {
	h.mux = http.NewServeMux()
	h.mux.HandleFunc("/proxy/auth/login/", h.login)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	fw, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
	}
	defer fw.Flush()
	addSSEHeaders(w)

	u, err := url.Parse("https://github.com/login/device/code")
	if err != nil {
		log.Fatal(err)
	}

	qp := u.Query()
	qp.Add("client_id", ClientID)
	qp.Add("scope", Scope)
	u.RawQuery = qp.Encode()

	req, err := http.NewRequest(http.MethodPost, u.String(), nil)
	if err != nil {
		log.Fatal(err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := h.httpClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}

	if sc := resp.StatusCode; sc != http.StatusOK {
		log.Fatalf("something went wrong, got code %d", sc)
	}

	ghResp := struct {
		DeviceCode      string `json:"device_code"`
		UserCode        string `json:"user_code"`
		VerificationURI string `json:"verification_uri"`
		ExpiresIn       int    `json:"expires_in"`
		Interval        int    `json:"interval"`
	}{}

	err = json.NewDecoder(resp.Body).Decode(&ghResp)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	msg := fmt.Sprintf("msg: Browse to %s and enter code %s to authenticate.", ghResp.VerificationURI, ghResp.UserCode)
	_, err = fmt.Fprintf(w, "%s\n\n", msg)
	if err != nil {
		log.Fatal(err)
	}

	fw.Flush()

	atBody := struct {
		Error       string `json:"error"`
		ErrorDesc   string `json:"error_description"`
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
		Scope       string `json:"scope"`
	}{}

	for {
		// Poll the URL at the appropriate interval
		// for a max time of ~15 minutes.
		atURL, err := url.Parse("https://github.com/login/oauth/access_token")
		if err != nil {
			log.Fatal(err)
		}
		qp := atURL.Query()
		qp.Add("client_id", ClientID)
		qp.Add("device_code", ghResp.DeviceCode)
		qp.Add("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
		atURL.RawQuery = qp.Encode()

		atReq, err := http.NewRequest(http.MethodPost, atURL.String(), nil)
		if err != nil {
			log.Fatal(err)
		}
		atReq.Header.Set("Accept", "application/json")

		atResp, err := h.httpClient.Do(atReq)
		if err != nil {
			log.Fatal(err)
		}

		err = json.NewDecoder(atResp.Body).Decode(&atBody)
		if err != nil {
			log.Fatal(err)
		}
		err = atResp.Body.Close()
		if err != nil {
			log.Fatal(err)
		}

		if atBody.Error != "" {
			atBody.Error = ""
			atBody.ErrorDesc = ""
			time.Sleep(time.Duration(ghResp.Interval) * time.Second)
			continue
		}
		break
	}

	userReq, err := http.NewRequest(http.MethodGet, "https://api.github.com/user", nil)
	if err != nil {
		log.Fatal(err)
	}

	userReq.Header.Set("Authorization", "token "+atBody.AccessToken)
	userResp, err := http.DefaultClient.Do(userReq)
	if err != nil {
		log.Fatal(err)
	}

	getUser := struct {
		Login string `json:"login"`
	}{}
	err = json.NewDecoder(userResp.Body).Decode(&getUser)
	if err != nil {
		log.Fatal(err)
	}
	userResp.Body.Close()

	accessToken, err := signToken(30*time.Second, getUser.Login, "secret")
	if err != nil {
		log.Fatal(err)
	}
	refreshToken, err := signToken(7*24*time.Hour, getUser.Login, "secret")
	if err != nil {
		log.Fatal(err)
	}

	accessTokenEnc := base64.StdEncoding.EncodeToString([]byte(accessToken))
	refreshTokenEnc := base64.StdEncoding.EncodeToString([]byte(refreshToken))

	_, err = fmt.Fprintf(w, `secret: 
apiVersion: v1
kind: Secret
metadata:
  name: tokens
  namespace: vxflexos
type: Opaque
data:
  access: %s
  refresh: %s


`, accessTokenEnc, refreshTokenEnc)
	if err != nil {
		log.Fatal(err)
	}
	fw.Flush()
}

func addSSEHeaders(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")
}

func signToken(t time.Duration, aud, secret string) (string, error) {
	claims := jwt.StandardClaims{
		Issuer:    "com.dell.storage-gatekeeper",
		ExpiresAt: time.Now().Add(t).Unix(),
		Audience:  aud,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}
