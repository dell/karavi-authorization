package token

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"karavi-authorization/pb"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/theckman/yacspin"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v2"
)

type GenerateConfig struct {
	Stdout       io.Writer
	Addr         string
	Namespace    string
	FromConfig   string
	SharedSecret string
}

func Generate(cfg GenerateConfig) error {
	conn, err := grpc.Dial(cfg.Addr,
		grpc.WithTimeout(10*time.Second),
		grpc.WithContextDialer(func(_ context.Context, addr string) (net.Conn, error) {
			return tls.Dial("tcp", addr, &tls.Config{
				NextProtos:         []string{"h2"},
				InsecureSkipVerify: true,
			})
		}),
		grpc.WithInsecure())
	if err != nil {
		return err
	}

	client := pb.NewAuthServiceClient(conn)

	switch {
	case cfg.FromConfig != "":
		return usingSelfSigned(client, cfg)
	}
	return usingGitHub(client, cfg)
}

func usingSelfSigned(client pb.AuthServiceClient, cfg GenerateConfig) error {
	p, err := filepath.Abs(cfg.FromConfig)
	if err != nil {
		return err
	}
	if _, err := os.Stat(p); err != nil {
		return err
	}
	f, err := os.Open(p)
	if err != nil {
		return err
	}
	var v struct {
		Sub   string `yaml:"sub"`
		Role  string `yaml:"role"`
		Group string `yaml:"group"`
	}
	err = yaml.NewDecoder(f).Decode(&v)
	if err != nil {
		return err
	}

	// Create the claims
	claims := struct {
		jwt.StandardClaims
		Role  string `json:"role"`
		Group string `json:"group"`
	}{
		StandardClaims: jwt.StandardClaims{
			Issuer:    "com.dell.karavi",
			ExpiresAt: time.Now().Add(30 * time.Second).Unix(),
			Audience:  "karavi",
			Subject:   v.Sub,
		},
		Role:  v.Role,
		Group: v.Group,
	}
	// Sign for an access token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	accessToken, err := token.SignedString([]byte(cfg.SharedSecret))
	if err != nil {
		return err
	}
	// Sign for a refresh token
	claims.ExpiresAt = time.Now().Add(365 * 24 * time.Hour).Unix()
	token = jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	refreshToken, err := token.SignedString([]byte(cfg.SharedSecret))
	if err != nil {
		return err
	}

	accessTokenEnc := base64.StdEncoding.EncodeToString([]byte(accessToken))
	refreshTokenEnc := base64.StdEncoding.EncodeToString([]byte(refreshToken))

	fmt.Printf(`
apiVersion: v1
kind: Secret
metadata:
  name: proxy-authz-tokens
type: Opaque
data:
  access: %s
  refresh: %s
`, accessTokenEnc, refreshTokenEnc)

	return nil
}

func usingGitHub(client pb.AuthServiceClient, cfg GenerateConfig) error {
	stream, err := client.Login(context.Background(), &pb.LoginRequest{Namespace: cfg.Namespace})
	if err != nil {
		return err
	}

	var spinner *yacspin.Spinner
	var msgPrinted bool
	for {
		stat, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// Print the message if we have it, and not already printed.
		if msg := stat.AuthURL; msg != "" && !msgPrinted {
			cfg := yacspin.Config{
				Message:       fmt.Sprintf(" %s", strings.ReplaceAll(msg, "\n", "")),
				Frequency:     100 * time.Millisecond,
				CharSet:       yacspin.CharSets[23],
				Prefix:        " ",
				StopCharacter: "✓",
				StopMessage:   " Authenticated!",
				StopColors:    []string{"fgGreen"},
				Writer:        os.Stderr,
			}
			var err error
			if spinner, err = yacspin.New(cfg); err != nil {
				return err
			}
			if err = spinner.Start(); err != nil {
				return err
			}
			msgPrinted = true
		}

		if secrets := stat.SecretYAML; secrets != "" {
			err := spinner.Stop()
			if err != nil {
				return err
			}
			fmt.Fprintln(cfg.Stdout, secrets)
			break
		}

	}

	return nil
}
