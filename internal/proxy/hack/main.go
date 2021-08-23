package main

import (
	"context"
	"fmt"
	"log"

	pscale "github.com/dell/goisilon"
)

func main() {
	c, err := pscale.NewClientWithArgs(context.Background(), "https://10.247.100.207:8080", true, 1, "root", "Administrators", "dangerous", "/ifs/data/csi", "777")
	if err != nil {
		log.Fatal(fmt.Errorf("powerscale authentication failed: %+v", err))
	}
	//c.GetClusterConfig()
	log.Printf("%+v", c)
}
