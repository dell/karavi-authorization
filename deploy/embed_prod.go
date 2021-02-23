// +build prod

package main

import "embed"

// This file is used for building production binaries and so will embed
// the real bundle file.
var (
	//go:embed "dist/karavi-airgap-install.tar.gz"
	embedBundleTar embed.FS
)
