// Package main is the entry point for the flowbot-registry CLI
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/flowline-io/flowbot-registry/pkg/manifest"
	"github.com/spf13/cobra"
)

var publishArgs struct {
	registryURL string
	storeURL    string
	apiKey      string
}

func publishCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "publish",
		Short: "Publish a plugin to the registry",
		Long: `Packages the current directory as an OCI artifact and publishes it to the
plugin registry.

The current directory must contain a plugin.yaml file and may include
a plugin.wasm binary and README.md.`,
		RunE: runPublish,
	}

	cmd.Flags().StringVar(&publishArgs.registryURL, "registry-url", "http://localhost:5000", "OCI registry URL")
	cmd.Flags().StringVar(&publishArgs.storeURL, "store-url", "http://localhost:8128", "Store API URL")
	cmd.Flags().StringVar(&publishArgs.apiKey, "api-key", "", "API key for authentication")

	return cmd
}

func runPublish(_ *cobra.Command, _ []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	slog.Info("publish: reading plugin.yaml", "dir", cwd)

	manifestPath := filepath.Join(cwd, "plugin.yaml")
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("read plugin.yaml: %w (run 'flowbot plugin init' first)", err)
	}

	m, err := manifest.ParseManifest(raw)
	if err != nil {
		return fmt.Errorf("parse plugin.yaml: %w", err)
	}

	_, _ = fmt.Printf("Publishing plugin: %s v%s\n", m.Name, m.Version)

	namespace, name := parsePluginName(m.Name)

	files, err := collectArtifacts(cwd)
	if err != nil {
		return fmt.Errorf("collect artifacts: %w", err)
	}

	slog.Info("publish: collected artifacts",
		"namespace", namespace, "name", name,
		"version", m.Version, "file_count", len(files),
	)

	ociRef := fmt.Sprintf("%s/%s/%s:%s", publishArgs.registryURL, namespace, name, m.Version)

	digest, err := pushOCIArtifact(context.Background(), ociRef, files, publishArgs.apiKey, publishArgs.storeURL)
	if err != nil {
		slog.Error("publish: push failed", "error", err, "ref", ociRef)
		return fmt.Errorf("push OCI artifact: %w", err)
	}

	slog.Info("publish: artifact pushed", "digest", digest)

	if err := registerPublishMetadata(publishArgs.storeURL, publishArgs.apiKey, namespace, name, m.Version, digest); err != nil {
		slog.Error("publish: register failed", "error", err)
		return fmt.Errorf("register publish metadata: %w", err)
	}

	_, _ = fmt.Println("Plugin published successfully.")

	return nil
}

func parsePluginName(fullName string) (string, string) {
	parts := strings.SplitN(fullName, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "default", fullName
}

func collectArtifacts(dir string) ([]ArtifactFile, error) {
	var files []ArtifactFile

	manifestData, err := os.ReadFile(filepath.Join(dir, "plugin.yaml"))
	if err != nil {
		return nil, fmt.Errorf("read plugin.yaml: %w", err)
	}
	files = append(files, ArtifactFile{
		Name:    "plugin.yaml",
		Content: manifestData,
	})

	if data, err := os.ReadFile(filepath.Join(dir, "README.md")); err == nil {
		files = append(files, ArtifactFile{
			Name:    "README.md",
			Content: data,
		})
	}

	if data, err := os.ReadFile(filepath.Join(dir, "plugin.wasm")); err == nil {
		files = append(files, ArtifactFile{
			Name:    "plugin.wasm",
			Content: data,
		})
	}

	if entries, err := os.ReadDir(dir); err == nil {
		for _, entry := range entries {
			if entry.IsDir() || strings.HasSuffix(entry.Name(), ".yaml") ||
				strings.HasSuffix(entry.Name(), ".md") || strings.HasSuffix(entry.Name(), ".wasm") {
				continue
			}
			if data, err := os.ReadFile(filepath.Join(dir, entry.Name())); err == nil && len(data) > 0 {
				files = append(files, ArtifactFile{
					Name:    entry.Name(),
					Content: data,
				})
			}
		}
	}

	return files, nil
}

// ArtifactFile represents a file to include in the OCI artifact.
type ArtifactFile struct {
	Name    string
	Content []byte
}

func pushOCIArtifact(_ context.Context, ref string, files []ArtifactFile, _ string, _ string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "flowbot-publish-*")
	if err != nil {
		return "", fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	for _, f := range files {
		if err := os.WriteFile(filepath.Join(tmpDir, f.Name), f.Content, 0o644); err != nil {
			return "", fmt.Errorf("write temp file %s: %w", f.Name, err)
		}
	}

	slog.Debug("publish: temp artifact prepared", "dir", tmpDir, "ref", ref)

	return "", fmt.Errorf("%w: OCI push via oras-go not yet wired (target: %s)", errNotImplemented, ref)
}

func registerPublishMetadata(storeURL string, apiKey string, namespace string, name string, version string, digest string) error {
	_ = storeURL
	_ = apiKey
	_ = namespace
	_ = name
	_ = version
	_ = digest

	slog.Debug("publish: metadata registration skipped (not implemented)",
		"store_url", storeURL, "namespace", namespace, "name", name,
	)

	return fmt.Errorf("%w: store publish API registration not yet wired", errNotImplemented)
}
