package main

import (
	"context"
	"net/http"
	"testing"
)

func TestTokenFromRequest(t *testing.T) {
	want := "123"
	r, err := http.NewRequest(http.MethodGet, "http://10.0.0.1/", nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.WithValue(r.Context(), CtxKeyToken{}, "123")
	r = r.WithContext(ctx)

	if got := TokenFromRequest(r); got != want {
		t.Errorf("TokenFromRequest: got %q, want %q", got, want)
	}
}
