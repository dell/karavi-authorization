package main

import (
	"context"
	"fmt"
	"log"

	pscale "github.com/dell/goisilon"
)

func main() {
	c, err := pscale.NewClientWithArgs(context.Background(), "https://10.247.96.195:8080", true, 0, "admin", "Administrators", "Is1l0n", "")
	if err != nil {
		log.Fatal(err)
	}

	v, err := c.GetVolumeWithIsiPath(context.Background(), "/ifs/csm-aaron", "", "")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%+v\n", v.AttributeMap)
}
