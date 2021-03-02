package main

import (
	"fmt"
	"log"
	"net/http"
)

func main() {
	pf := &PowerFlex{}
	log.Fatal(http.ListenAndServe(":8000", pf))
}

type PowerFlex struct{}

func (pf *PowerFlex) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/types/Volume/instances/":
		w.Write([]byte(`{"id":"000000000000001", "name": "TestVolume"}`))
	case "/api/instances/Volume::000000000000001":
		w.Write([]byte(`{"sizeInKb":10, "storagePoolId":"3df6b86600000000", "name": "TestVolume"}`))
	case "/api/login":
		w.Write([]byte("token"))
	case "/api/version":
		w.Write([]byte("3.5"))
	case "/api/types/StoragePool/instances":
		w.Write([]byte(`[{"protectionDomainId": "75b661b400000000", "mediaType": "HDD", "id": "3df6b86600000000", "name": "TestPool"}]`))
	default:
		http.Error(w, fmt.Sprintf("Unexpected api call to fake PowerFlex: %v", r.URL.Path), http.StatusBadRequest)
	}
}
