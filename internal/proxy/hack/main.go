package main

import (
	"context"
	"fmt"
	"log"

	pscale "github.com/dell/goisilon"
)

func main() {
	c, err := pscale.NewClientWithArgs(context.Background(), "https://lglw6195:8080", false, 1, "admin", "Administrators", "Is1l0n", "/ifs/data/csm")
	if err != nil {
		log.Fatal(fmt.Errorf("powerscale authentication failed: %+v", err))
	}
	//c.GetClusterConfig()
	log.Printf("%+v", c)
}
