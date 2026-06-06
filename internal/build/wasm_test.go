package build

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/flowline-io/flowbot-registry/pkg/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWasmBuilderBuild(t *testing.T) {
	m := &manifest.Manifest{
		Name:    "community/my-wasm",
		Version: "1.0.0",
		Runtime: manifest.RuntimeWasm,
		Wasm:    &manifest.WasmConfig{Module: "./plugin.wasm"},
	}
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc main() {}"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n\ngo 1.26\n"), 0o644))

	b := NewWasmBuilder()
	artifacts, err := b.Build(context.Background(), dir, m)
	require.NoError(t, err)
	require.NotEmpty(t, artifacts)

	names := make(map[string]bool)
	for _, a := range artifacts {
		names[a.Name] = true
	}
	assert.True(t, names["plugin.wasm"])
}

func TestWasmBuilderNoMainGo(t *testing.T) {
	m := &manifest.Manifest{
		Name:    "community/no-main",
		Version: "1.0.0",
		Runtime: manifest.RuntimeWasm,
		Wasm:    &manifest.WasmConfig{Module: "./plugin.wasm"},
	}
	dir := t.TempDir()
	b := NewWasmBuilder()
	_, err := b.Build(context.Background(), dir, m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "main.go")
}

func TestWasmBuilderContextCancelled(t *testing.T) {
	m := &manifest.Manifest{
		Name:    "community/cancel",
		Version: "1.0.0",
		Runtime: manifest.RuntimeWasm,
		Wasm:    &manifest.WasmConfig{Module: "./plugin.wasm"},
	}
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc main() {}"), 0o644))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	b := NewWasmBuilder()
	_, err := b.Build(ctx, dir, m)
	require.Error(t, err)
}
