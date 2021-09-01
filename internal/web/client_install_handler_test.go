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
	r.Host = "127.0.0.1"

	q := r.URL.Query()
	q.Add("namespace", "powermax")
	q.Add("namespace", "orange")
	q.Add("proxy-port", "powermax:20001")
	r.URL.RawQuery = q.Encode()

	wantHost := fmt.Sprintf("--proxy-host %s", r.Host)
	imageAddr := "127.0.0.1/sidecar:latest"
	wantImageAddr := fmt.Sprintf("--image-addr %s", imageAddr)

	// the tokens are based on time, so we can't easily test for them.
	wantInsecureTkn := fmt.Sprintf("--insecure")
	wantRootCATkn := fmt.Sprintf("--root-certificate")

	oldSidecarProxyAddr := web.SidecarProxyAddr
	web.SidecarProxyAddr = "127.0.0.1/sidecar:latest"
	defer func() {
		web.SidecarProxyAddr = oldSidecarProxyAddr
	}()

	web.ClientInstallHandler("root-certificate.pem", false).ServeHTTP(w, r)
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
	if !bytes.Contains(b, []byte(wantInsecureTkn)) {
		t.Error("expected body to contain guest refresh token")
	}
	if !bytes.Contains(b, []byte(wantRootCATkn)) {
		t.Error("expected body to contain guest refresh token")
	}
	if !bytes.Contains(b, []byte("powermax,orange")) {
		t.Error("expected body to contain namepaces")
	}
	if !bytes.Contains(b, []byte("--proxy-port powermax=20001")) {
		t.Error("expected body to contain proxy-ports")
	}
}
