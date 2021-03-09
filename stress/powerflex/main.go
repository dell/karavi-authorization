package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strings"
)

func main() {
	log.Fatal(http.ListenAndServeTLS(":8000", "./cert.pem", "./key.pem", &PowerFlex{}))
}

var (
	volumeInstancePath        = regexp.MustCompile(`/api/instances/Volume::[a-f0-9]+/$`)
	volumeInstancePathNoSlash = regexp.MustCompile(`/api/instances/Volume::[a-f0-9]+`)
	addMappedSdcPath          = regexp.MustCompile(`/api/instances/Volume::[a-f0-9]+/action/addMappedSdc/`)
	removeMappedSdcPath       = regexp.MustCompile(`/api/instances/Volume::[a-f0-9]+/action/removeMappedSdc/`)
	removeVolumePath          = regexp.MustCompile(`/api/instances/Volume::[a-f0-9]+/action/removeVolume/`)
)

type PowerFlex struct{}

func (pf *PowerFlex) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/api/types/Volume/instances/":
		body := struct {
			VolumeSizeInKb string `json:"volumeSizeInKb"`
			StoragePoolID  string `json:"storagePoolId"`
			Name           string `json:"name"`
		}{}
		err := json.NewDecoder(r.Body).Decode(&body)
		if err != nil {
			panic(err)
		}
		//log.Println(fmt.Sprintf(`{"id":"%s", "name": "%s"}`, body.Name, body.Name))
		w.Write([]byte(fmt.Sprintf(`{"id":"%s", "name": "%s"}`, body.Name, body.Name)))
	case volumeInstancePath.FindString(r.URL.Path):
		var id string
		z := strings.SplitN(r.URL.Path, "/", 5)
		if len(z) > 3 {
			id = z[3]
		}
		id = strings.TrimPrefix(id, "Volume::")
		//log.Println(fmt.Sprintf(`{"id":"%s", "name": "%s", "storagePoolId": "dcc71b0500000000"}`, id, id))
		w.Write([]byte(fmt.Sprintf(`{"id":"%s", "name": "%s", "storagePoolId": "dcc71b0500000000"}`, id, id)))
	case volumeInstancePathNoSlash.FindString(r.URL.Path):
		var id string
		z := strings.SplitN(r.URL.Path, "/", 5)
		if len(z) > 3 {
			id = z[3]
		}
		id = strings.TrimPrefix(id, "Volume::")
		w.Write([]byte(fmt.Sprintf(`{"id":"%s", "name": "%s", "storagePoolId": "dcc71b0500000000"}`, id, id)))
	case "/api/login":
		w.Write([]byte("token"))
	case "/api/version/":
		w.Write([]byte("3.5"))
	case "/api/version":
		w.Write([]byte("3.5"))
	case "/api/types/StoragePool/instances/":
		w.Write([]byte(`[{"protectionDomainId": "ed1efbd300000000", "mediaType": "HDD", "id": "dcc71b0500000000", "name": "mypool"}]`))
	case "/api/types/StoragePool/instances":
		w.Write([]byte(`[{"protectionDomainId": "ed1efbd300000000", "mediaType": "HDD", "id": "dcc71b0500000000", "name": "mypool"}]`))
	case "/api/types/System/instances":
		data, err := ioutil.ReadFile("stress/powerflex/testdata/system_instances.json")
		if err != nil {
			panic(err)
		}
		w.Write(data)
	case "/api/types/Volume/instances/action/queryIdByKey/":
		resp := struct {
			Name string `json:"name"`
		}{}
		err := json.NewDecoder(r.Body).Decode(&resp)
		if err != nil {
			panic(err)
		}
		w.Write([]byte(fmt.Sprintf(`{"id":"%s"}`, resp.Name)))
	case "/api/instances/System::7045c4cc20dffc0f/relationships/Sdc/":
		data, err := ioutil.ReadFile("stress/powerflex/testdata/system_relationship_sdc.json")
		if err != nil {
			panic(err)
		}
		w.Write(data)
	case addMappedSdcPath.FindString(r.URL.Path):
		w.Write([]byte(`{}`))
	case removeVolumePath.FindString(r.URL.Path):
		w.Write([]byte(`{}`))
	case removeMappedSdcPath.FindString(r.URL.Path):
		w.Write([]byte(`{}`))
	case "/api/instances/System::7045c4cc20dffc0f/relationships/ProtectionDomain":
		data, err := ioutil.ReadFile("stress/powerflex/testdata/system_pd_relationship.json")
		if err != nil {
			panic(err)
		}
		w.Write(data)
	case "/api/instances/ProtectionDomain::ed1efbd300000000/relationships/StoragePool":
		data, err := ioutil.ReadFile("stress/powerflex/testdata/pd_relationship_sp.json")
		if err != nil {
			panic(err)
		}
		w.Write(data)
	case "/api/instances/StoragePool::dcc71b0500000000/relationships/Statistics":
		data, err := ioutil.ReadFile("stress/powerflex/testdata/sp_stats.json")
		if err != nil {
			panic(err)
		}
		w.Write(data)
	default:
		log.Printf("invalid path: %s", r.URL.Path)
	}
}
