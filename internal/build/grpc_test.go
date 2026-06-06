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

func TestGrpcBuilderBuild(t *testing.T) {
	m := &manifest.Manifest{
		Name:    "community/my-plugin",
		Version: "1.0.0",
		Runtime: manifest.RuntimeGRPC,
		GRPC:    &manifest.GRPCConfig{Binary: "./plugin-server"},
	}

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "cmd", "server"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "cmd", "server", "main.go"), []byte("package main\nfunc main() {}"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n\ngo 1.26\n"), 0o644))

	b := NewGrpcBuilder()
	artifacts, err := b.Build(context.Background(), dir, m)
	require.NoError(t, err)
	require.NotEmpty(t, artifacts)

	var names []string
	for _, a := range artifacts {
		names = append(names, a.Name)
	}
	assert.Contains(t, names, "plugin-server")
}

func TestGrpcBuilderNoCmdServer(t *testing.T) {
	m := &manifest.Manifest{
		Name:    "community/no-cmd",
		Version: "1.0.0",
		Runtime: manifest.RuntimeGRPC,
		GRPC:    &manifest.GRPCConfig{Binary: "./plugin-server"},
	}
	dir := t.TempDir()
	b := NewGrpcBuilder()
	_, err := b.Build(context.Background(), dir, m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cmd/server")
}

func TestGrpcBuilderContextCancelled(t *testing.T) {
	m := &manifest.Manifest{
		Name:    "community/cancel",
		Version: "1.0.0",
		Runtime: manifest.RuntimeGRPC,
		GRPC:    &manifest.GRPCConfig{Binary: "./plugin-server"},
	}
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "cmd", "server"), 0o755))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	b := NewGrpcBuilder()
	_, err := b.Build(ctx, dir, m)
	require.Error(t, err)
}
