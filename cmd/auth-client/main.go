package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"powerflex-reverse-proxy/pb"
	"strings"
	"time"

	"github.com/theckman/yacspin"
	"google.golang.org/grpc"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %+v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var gatekeeperAddress string
	flag.StringVar(&gatekeeperAddress, "gk-address", "grpc.gatekeeper.cluster:443", "Address/hostname and port of gatekeeper")

	flag.Parse()

	d, err := tls.Dial("tcp", gatekeeperAddress, &tls.Config{
		NextProtos:         []string{"h2"},
		InsecureSkipVerify: true,
	})
	if err != nil {
		log.Fatal(err)
	}

	conn, err := grpc.Dial(gatekeeperAddress,
		grpc.WithTimeout(10*time.Second),
		grpc.WithContextDialer(func(_ context.Context, _ string) (net.Conn, error) {
			return d, nil
		}),
		grpc.WithInsecure())
	if err != nil {
		return err
	}

	client := pb.NewAuthServiceClient(conn)

	stream, err := client.Login(context.Background(), &pb.LoginRequest{})
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
			fmt.Println(secrets)
			break
		}

	}

	return nil
}
