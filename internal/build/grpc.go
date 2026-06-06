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

// GrpcBuilder compiles gRPC plugins for linux/amd64.
type GrpcBuilder struct{}

// NewGrpcBuilder creates a new GrpcBuilder.
func NewGrpcBuilder() *GrpcBuilder {
	return &GrpcBuilder{}
}

// Build compiles a gRPC plugin targeting linux/amd64.
func (*GrpcBuilder) Build(ctx context.Context, dir string, m *manifest.Manifest) ([]Artifact, error) {
	cmdDir := filepath.Join(dir, "cmd", "server")
	if _, err := os.Stat(filepath.Join(cmdDir, "main.go")); os.IsNotExist(err) {
		return nil, fmt.Errorf("cmd/server/main.go not found in %s", dir)
	}

	outputName := "plugin-server"
	if m.GRPC != nil && m.GRPC.Binary != "" {
		outputName = filepath.Base(m.GRPC.Binary)
	}
	outputPath := filepath.Join(dir, outputName)

	slog.Info("build: compiling gRPC plugin",
		"os", "linux", "arch", "amd64", "dir", cmdDir, "output", outputPath,
	)

	args := []string{"build", "-o", outputPath}
	if m.GRPC != nil {
		args = append(args, m.GRPC.Args...)
	}
	args = append(args, "./cmd/server")

	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"CGO_ENABLED=0",
		"GOOS=linux",
		"GOARCH=amd64",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("go build failed: %w\n%s", err, string(output))
	}

	content, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("read built binary %s: %w", outputPath, err)
	}

	return []Artifact{
		{Name: outputName, Content: content},
	}, nil
}
