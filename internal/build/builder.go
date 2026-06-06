// Package build orchestrates cross-compilation of plugin source code into deployable binaries.
package build

import (
	"context"

	"github.com/flowline-io/flowbot-registry/pkg/manifest"
)

// Artifact represents a built file ready for OCI packaging.
type Artifact struct {
	Name    string // filename: "plugin.yaml", "plugin-server", "plugin.wasm"
	Content []byte // file contents
}

// Builder compiles plugin source code into artifacts.
type Builder interface {
	// Build compiles the plugin in dir and returns the built artifacts.
	Build(ctx context.Context, dir string, m *manifest.Manifest) ([]Artifact, error)
}
