package web_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"karavi-authorization/internal/web"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientInstallHandler(t *testing.T) {
	w := httptest.NewRecorder()
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.Host = "10.0.0.1"
	wantHost := fmt.Sprintf("--proxy-host %s", r.Host)
	imageAddr := "10.0.0.1/sidecar:latest"
	wantImageAddr := fmt.Sprintf("--image-addr %s", imageAddr)

	web.ClientInstallHandler(imageAddr).ServeHTTP(w, r)
	b, err := ioutil.ReadAll(w.Body)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Contains(b, []byte(wantHost)) {
		t.Error("expected body to contain proxy host")
	}
	if !bytes.Contains(b, []byte(wantImageAddr)) {
		t.Error("expected body to contain proxy host")
	}
}