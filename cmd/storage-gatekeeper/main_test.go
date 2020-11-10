package main

import (
	"context"
	"net/http"
	"testing"
)

func TestQueryIdByKey(t *testing.T) {
	want := "k8s-f37cc1a11f"
	got, err := QueryNameByID("Volume::c3f5cd1c00000003")
	if err != nil {
		t.Fatal(err)
	}

	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestQueryStoragePoolNameByID(t *testing.T) {
	want := "bronze"
	got, err := QueryStoragePoolNameByID("8633480700000000")
	if err != nil {
		t.Fatal(err)
	}

	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

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
