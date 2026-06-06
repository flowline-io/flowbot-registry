package build

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/flowline-io/flowbot-registry/pkg/manifest"
)

// WasmBuilder compiles Wasm/WASI plugins using tinygo.
type WasmBuilder struct{}

// NewWasmBuilder creates a new WasmBuilder.
func NewWasmBuilder() *WasmBuilder {
	return &WasmBuilder{}
}

// Build compiles a Wasm plugin targeting wasi/wasm.
func (*WasmBuilder) Build(ctx context.Context, dir string, m *manifest.Manifest) ([]Artifact, error) {
	if _, err := os.Stat(filepath.Join(dir, "main.go")); os.IsNotExist(err) {
		return nil, fmt.Errorf("main.go not found in %s", dir)
	}

	outputName := "plugin.wasm"
	if m.Wasm != nil && m.Wasm.Module != "" {
		outputName = filepath.Base(m.Wasm.Module)
	}
	outputPath := filepath.Join(dir, outputName)

	slog.Info("build: compiling Wasm plugin",
		"target", "wasi", "dir", dir, "output", outputPath,
	)

	cmd := exec.CommandContext(ctx, "tinygo", "build",
		"-target=wasi",
		"-no-debug",
		"-o", outputPath,
		".",
	)
	cmd.Dir = dir
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("tinygo build failed: %w\n%s", err, string(output))
	}

	content, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("read built wasm %s: %w", outputPath, err)
	}

	return []Artifact{
		{Name: outputName, Content: content},
	}, nil
}
