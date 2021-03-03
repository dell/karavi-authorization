package main

import (
	"log"
	"net/http"
)

func main() {
	log.Fatal(http.ListenAndServeTLS(":8000", "./cert.pem", "./key.pem", &PowerFlex{}))
}

type PowerFlex struct{}

func (pf *PowerFlex) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	//log.Println(r.URL.Path)
	switch r.URL.Path {
	case "/api/types/Volume/instances/":
		w.Write([]byte(`{"id":"000000000000001", "name": "TestVolume"}`))
	case "/api/instances/Volume::000000000000001":
		w.Write([]byte(`{"sizeInKb":10, "storagePoolId":"3df6b86600000000", "name": "TestVolume"}`))
	case "/api/login":
		w.Write([]byte("token"))
	case "/api/version/":
		w.Write([]byte("3.5"))
	case "/api/version":
		w.Write([]byte("3.5"))
	case "/api/types/StoragePool/instances/":
		w.Write([]byte(`[{"protectionDomainId": "75b661b400000000", "mediaType": "HDD", "id": "3df6b86600000000", "name": "TestPool"}]`))
	case "/api/types/StoragePool/instances":
		w.Write([]byte(`[{"protectionDomainId": "75b661b400000000", "mediaType": "HDD", "id": "3df6b86600000000", "name": "TestPool"}]`))
	case "/api/types/System/instances":
		w.Write([]byte(`[{"id": "7045c4cc20dffc0f"}]`))
	default:
		log.Printf("invalid path: %s", r.URL.Path)
	}
}
