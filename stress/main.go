package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
)

func main() {
	req, err := newCreateVolumeRequest(context.Background(), "123", "2000")
	if err != nil {
		log.Fatal(err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}

	log.Println(string(body))
}

func newCreateVolumeRequest(ctx context.Context, storagePoolID string, volumeSizeinKb string) (*http.Request, error) {
	body := struct {
		VolumeSize     int64
		VolumeSizeInKb string `json:"volumeSizeInKb"`
		StoragePoolID  string `json:"storagePoolId"`
	}{
		VolumeSize:     2000,
		VolumeSizeInKb: volumeSizeinKb,
		StoragePoolID:  storagePoolID,
	}
	data, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	payload := bytes.NewBuffer(data)

	r, err := http.NewRequestWithContext(ctx, http.MethodPost, "/api/types/Volume/instances/", payload)
	if err != nil {
		return nil, err
	}
	return r, nil
}
