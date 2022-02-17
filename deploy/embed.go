//go:build !prod
// +build !prod

package main

import "embed"

// This file is used for every day development and testing purposes due to
// the large file size of the intended bundle.  Here you can see that we're
// embedded a very small file.
var (
	//go:embed "testdata/fake-bundle.tar.gz"
	embedBundleTar embed.FS
)
