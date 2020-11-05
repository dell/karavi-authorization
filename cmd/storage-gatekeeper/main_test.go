package main

import "testing"

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
