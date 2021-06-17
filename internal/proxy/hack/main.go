package main

import (
	"context"
	"fmt"

	"github.com/dell/goisilon"
)

func main() {
	client, err := goisilon.NewClientWithArgs(
		context.Background(),
		"https://10.247.96.195:8080",
		true,
		1,
		"admin",
		"admin",
		"Is1l0n",
		"/ifs/csm-aaron")
	if err != nil {
		panic(err)
	}

	vols, err := client.GetVolumes(context.Background())
	if err != nil {
		panic(err)
	}

	printVols(vols)
}

func printVols(vols []goisilon.Volume) {
	for _, v := range vols {
		fmt.Printf("%+v", v.AttributeMap)
	}
}
