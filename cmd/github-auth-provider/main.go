package main

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"powerflex-reverse-proxy/pb"
	"time"

	"github.com/dgrijalva/jwt-go"
	"google.golang.org/grpc"
)

const (
	DefaultListenAddr = ":50051"
	ClientID          = "b016f6f31210082e52c2"
	Scope             = "user"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %+v\n", err)
		os.Exit(1)
	}
}

func run() error {
	l, err := net.Listen("tcp", DefaultListenAddr)
	if err != nil {
		return err
	}
	defer func() {
		if err := l.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "closing listener: %+v\n", err)
		}
	}()

	as := defaultAuthService{}
	gs := grpc.NewServer()
	pb.RegisterAuthServiceServer(gs, &as)

	// TODO(ian): Support graceful shutdown.
	log.Println("Serving on", DefaultListenAddr)
	return gs.Serve(l)
}

type defaultAuthService struct{}

func (d *defaultAuthService) Login(req *pb.LoginRequest, stream pb.AuthService_LoginServer) error {
	httpClient := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: true,
			},
		},
		Timeout: 30 * time.Second,
	}
	u, err := url.Parse("https://github.com/login/device/code")
	if err != nil {
		return fmt.Errorf("parsing url: %w", err)
	}

	qp := u.Query()
	qp.Add("client_id", ClientID)
	qp.Add("scope", Scope)
	u.RawQuery = qp.Encode()

	postCodeReq, err := http.NewRequest(http.MethodPost, u.String(), nil)
	if err != nil {
		return fmt.Errorf("creating new request: %w", err)
	}
	postCodeReq.Header.Set("Accept", "application/json")

	postCodeResp, err := httpClient.Do(postCodeReq)
	if err != nil {
		return fmt.Errorf("making request: %w", err)
	}

	if sc := postCodeResp.StatusCode; sc != http.StatusOK {
		return fmt.Errorf("something went wrong, got code %d", sc)
	}

	ghResp := struct {
		DeviceCode      string `json:"device_code"`
		UserCode        string `json:"user_code"`
		VerificationURI string `json:"verification_uri"`
		ExpiresIn       int    `json:"expires_in"`
		Interval        int    `json:"interval"`
	}{}

	err = json.NewDecoder(postCodeResp.Body).Decode(&ghResp)
	if err != nil {
		return err
	}
	defer postCodeResp.Body.Close()

	var stat pb.LoginStatus

	msg := fmt.Sprintf("Browse to %s and enter code %s to authenticate.", ghResp.VerificationURI, ghResp.UserCode)
	stat.AuthURL = msg
	err = stream.Send(&stat)
	if err != nil {
		return err
	}

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
			return err
		}
		qp := atURL.Query()
		qp.Add("client_id", ClientID)
		qp.Add("device_code", ghResp.DeviceCode)
		qp.Add("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
		atURL.RawQuery = qp.Encode()

		atReq, err := http.NewRequest(http.MethodPost, atURL.String(), nil)
		if err != nil {
			return err
		}
		atReq.Header.Set("Accept", "application/json")

		atResp, err := httpClient.Do(atReq)
		if err != nil {
			return err
		}

		err = json.NewDecoder(atResp.Body).Decode(&atBody)
		if err != nil {
			return err
		}
		err = atResp.Body.Close()
		if err != nil {
			return err
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
		return err
	}

	userReq.Header.Set("Authorization", "token "+atBody.AccessToken)
	userResp, err := httpClient.Do(userReq)
	if err != nil {
		return err
	}

	getUser := struct {
		Login string `json:"login"`
	}{}
	err = json.NewDecoder(userResp.Body).Decode(&getUser)
	if err != nil {
		return err
	}
	userResp.Body.Close()

	accessToken, err := signToken(30*time.Second, getUser.Login, "secret")
	if err != nil {
		return err
	}
	refreshToken, err := signToken(7*24*time.Hour, getUser.Login, "secret")
	if err != nil {
		return err
	}

	accessTokenEnc := base64.StdEncoding.EncodeToString([]byte(accessToken))
	refreshTokenEnc := base64.StdEncoding.EncodeToString([]byte(refreshToken))

	stat.SecretYAML = fmt.Sprintf(`
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

	if err := stream.Send(&stat); err != nil {
		return err
	}

	return nil
}

func (d *defaultAuthService) Refresh(ctx context.Context, req *pb.RefreshRequest) (*pb.RefreshResponse, error) {
	panic("not implemented") // TODO: Implement
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
