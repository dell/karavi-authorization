package main

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"powerflex-reverse-proxy/pb"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/go-redis/redis/v8"
	"google.golang.org/grpc"
)

func init() {
	log.SetFlags(log.LstdFlags | log.Llongfile)
}

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
	refreshTokenSHA256 := sha256.Sum256([]byte(refreshToken))
	refreshTokenSHAEnc := base64.StdEncoding.EncodeToString(refreshTokenSHA256[:])

	// TODO(ian): Send a hash of the refresh token to Redis
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
	_, err = rdb.HSet(stream.Context(),
		"tenant:github:"+getUser.Login,
		"refresh_sha", refreshTokenSHAEnc,
		"refresh_isa", time.Now().Unix(),
		"refresh_count", 0).Result()
	if err != nil {
		return err
	}

	stat.SecretYAML = fmt.Sprintf(`
apiVersion: v1
kind: Secret
metadata:
  name: proxy-authz-tokens
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

	refreshToken := req.RefreshToken
	accessToken := req.AccessToken

	var refreshClaims jwt.StandardClaims
	_, err := jwt.ParseWithClaims(refreshToken, &refreshClaims, func(t *jwt.Token) (interface{}, error) {
		return []byte("secret"), nil
	})
	if err != nil {
		log.Printf("parsing refresh token %q: %+v", refreshToken, err)
		return nil, err
	}

	// Check if the tenant is being denied.
	ok, err := rdb.SIsMember(ctx, "tenant:deny", refreshClaims.Audience).Result()
	if err != nil {
		log.Printf("%+v", err)
		return nil, err
	}
	if ok {
		log.Printf("user denied", err)
		return nil, errors.New("user has been denied")
	}

	var accessClaims jwt.StandardClaims
	access, err := jwt.ParseWithClaims(accessToken, &accessClaims, func(t *jwt.Token) (interface{}, error) {
		return []byte("secret"), nil
	})
	if access.Valid {
		return nil, errors.New("access token was valid")
	} else if ve, ok := err.(*jwt.ValidationError); ok {
		switch {
		case ve.Errors&jwt.ValidationErrorExpired != 0:
			log.Println("Refreshing expired token for", accessClaims.Audience)
		default:
			log.Printf("%+v", err)
			return nil, err
		}
	}

	_, err = rdb.HIncrBy(ctx,
		"tenant:github:"+accessClaims.Audience,
		"refresh_count",
		1).Result()
	if err != nil {
		log.Printf("%+v", err)
		return nil, err
	}

	claims := jwt.StandardClaims{
		Audience:  accessClaims.Audience,
		ExpiresAt: time.Now().Add(30 * time.Second).Unix(),
	}
	newAccess := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	newAccessStr, err := newAccess.SignedString([]byte("secret"))
	if err != nil {
		log.Printf("%+v", err)
		return nil, err
	}

	return &pb.RefreshResponse{
		AccessToken: newAccessStr,
	}, nil
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
