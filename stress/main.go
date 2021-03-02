package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	name := os.Getenv("NAME")
	if name == "vxflexos-node" {
		select {}
	}

	resp, err := http.Get("https://localhost:9000/api/version")
	if err != nil {
		log.Fatal(err)
	}

	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	resp.Body.Close()

	log.Println(data)

	ticker := time.NewTicker(5 * time.Second)
	for {
		select {
		case <-ticker.C:
			fmt.Println("hello")
		}
	}
}
