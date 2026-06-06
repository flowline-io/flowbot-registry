package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/flowline-io/flowbot-registry/internal/build"
	"github.com/flowline-io/flowbot-registry/pkg/manifest"
	"github.com/flowline-io/flowbot-registry/pkg/oci"
	"github.com/flowline-io/flowbot-registry/pkg/sign"

	"github.com/google/go-containerregistry/pkg/v1"
	"github.com/spf13/cobra"
)

var publishArgs struct {
	registryURL string
	storeURL    string
	apiKey      string
	keyPath     string
	noSign      bool
}

func publishCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "publish",
		Short: "Publish a plugin to the registry",
		Long: `Cross-compile, package as OCI artifact, sign with Cosign, and publish to the registry.

The current directory must contain a plugin.yaml file.

Requires:
  - go (for gRPC plugins) or tinygo (for Wasm plugins)
  - Cosign private key via --key flag (unless --no-sign)`,
		RunE: runPublish,
	}

	cmd.Flags().StringVar(&publishArgs.registryURL, "registry", envOrFlag("FLOWBOT_REGISTRY_URL", "ghcr.io"), "OCI registry URL")
	cmd.Flags().StringVar(&publishArgs.storeURL, "store-url", envOrFlag("FLOWBOT_STORE_URL", "http://localhost:8128"), "Store API URL")
	cmd.Flags().StringVar(&publishArgs.apiKey, "api-key", envOrFlag("FLOWBOT_API_KEY", ""), "API key for store authentication")
	cmd.Flags().StringVar(&publishArgs.keyPath, "key", envOrFlag("COSIGN_KEY_PATH", ""), "Cosign private key path")
	cmd.Flags().BoolVar(&publishArgs.noSign, "no-sign", false, "Skip Cosign signing")

	return cmd
}

// envOrFlag returns the environment variable value if set, otherwise the fallback.
func envOrFlag(envKey, fallback string) string {
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	return fallback
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

	namespace, name := manifest.PluginNameFromRef(m.Name)

	_, _ = fmt.Printf("Publishing plugin: %s/%s v%s\n", namespace, name, m.Version)
	_, _ = fmt.Println()

	ociRef := fmt.Sprintf("%s/%s/%s:%s", strings.TrimRight(publishArgs.registryURL, "/"), namespace, name, m.Version)
	ociClient := oci.NewClient(publishArgs.registryURL)

	existingDigest, err := buildAndPushArtifact(cwd, raw, m, ociRef, ociClient)
	if err != nil {
		return err
	}

	// Sign with Cosign (unless --no-sign)
	if !publishArgs.noSign {
		_, _ = fmt.Println("  [3/4] Signing with Cosign...")
		if publishArgs.keyPath == "" {
			return fmt.Errorf("--key flag is required for signing (or use --no-sign to skip)")
		}

		signer, signErr := sign.NewSigner(publishArgs.keyPath, os.Getenv("COSIGN_PASSWORD"))
		if signErr != nil {
			return fmt.Errorf("load signing key: %w", signErr)
		}

		result, signErr := signer.Sign(fmt.Sprintf("%s@%s", ociRef, existingDigest))
		if signErr != nil {
			return fmt.Errorf("sign image: %w", signErr)
		}

		if pushSigErr := ociClient.PushSignature(context.Background(), ociRef, result.Payload, result.Signature); pushSigErr != nil {
			return fmt.Errorf("push signature: %w", pushSigErr)
		}
		_, _ = fmt.Println("  Signed OK")
	} else {
		_, _ = fmt.Println("  [3/4] Skipping signing (--no-sign)")
	}

	// Register metadata
	_, _ = fmt.Println("  [4/4] Registering metadata...")
	if regErr := registerPublishMetadata(publishArgs.storeURL, publishArgs.apiKey, ociRef, namespace, name, m.Version, existingDigest.String()); regErr != nil {
		return fmt.Errorf("register publish metadata: %w", regErr)
	}

	_, _ = fmt.Println()
	_, _ = fmt.Printf("Published: %s\n", ociRef)
	_, _ = fmt.Printf("Digest: %s\n", existingDigest)

	return nil
}

// buildAndPushArtifact checks for an existing image, builds the plugin, and pushes the OCI artifact.
// Returns the image digest, reusing the existing one on idempotent publish.
func buildAndPushArtifact(cwd string, raw []byte, m *manifest.Manifest, ociRef string, ociClient *oci.Client) (v1.Hash, error) {
	// Check idempotency
	existingDigest, err := ociClient.HeadManifest(context.Background(), ociRef)
	if err == nil {
		slog.Info("publish: image already exists", "ref", ociRef, "digest", existingDigest)
		_, _ = fmt.Printf("  Image already exists: %s@%s\n", ociRef, existingDigest)
		return existingDigest, nil
	}

	// Build
	_, _ = fmt.Println("  [1/4] Building plugin...")
	var builder build.Builder
	switch m.Runtime {
	case manifest.RuntimeGRPC:
		builder = build.NewGrpcBuilder()
	case manifest.RuntimeWasm:
		builder = build.NewWasmBuilder()
	default:
		return v1.Hash{}, fmt.Errorf("unknown runtime %q", m.Runtime)
	}

	artifacts, buildErr := builder.Build(context.Background(), cwd, m)
	if buildErr != nil {
		return v1.Hash{}, fmt.Errorf("build plugin: %w", buildErr)
	}

	var size int
	for _, a := range artifacts {
		size += len(a.Content)
	}
	_, _ = fmt.Printf("  Built %d artifact(s), %s\n", len(artifacts), formatSize(size))

	// Collect artifacts for OCI push
	ociFiles := []oci.ArtifactFile{
		{Name: "plugin.yaml", Content: raw},
	}
	for _, a := range artifacts {
		ociFiles = append(ociFiles, oci.ArtifactFile{Name: a.Name, Content: a.Content})
	}
	if readmeData, readErr := os.ReadFile(filepath.Join(cwd, "README.md")); readErr == nil {
		ociFiles = append(ociFiles, oci.ArtifactFile{Name: "README.md", Content: readmeData})
	}

	// Push OCI image
	_, _ = fmt.Println("  [2/4] Pushing OCI image...")
	existingDigest, pushErr := ociClient.PushArtifact(context.Background(), ociRef, ociFiles)
	if pushErr != nil {
		return v1.Hash{}, fmt.Errorf("push OCI artifact: %w", pushErr)
	}
	_, _ = fmt.Printf("  Pushed: %s\n", existingDigest)

	return existingDigest, nil
}

func formatSize(bytesCount int) string {
	const unit = 1024
	if bytesCount < unit {
		return fmt.Sprintf("%d B", bytesCount)
	}
	div, exp := unit, 0
	for n := bytesCount / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytesCount)/float64(div), "KMGTPE"[exp])
}

func registerPublishMetadata(storeURL string, apiKey string, ociRef string, namespace string, name string, version string, digest string) error {
	apiURL := fmt.Sprintf("%s/api/v1/plugins/%s/%s/publish",
		strings.TrimRight(storeURL, "/"), namespace, name)

	slog.Debug("publish: registering metadata", "url", apiURL, "version", version, "digest", digest)

	body := map[string]string{
		"version":       version,
		"oci_digest":    digest,
		"oci_image_ref": ociRef,
	}

	resp, err := doJSONPost(apiURL, body, apiKey)
	if err != nil {
		return fmt.Errorf("register metadata: %w", err)
	}
	defer resp.Body.Close()

	return nil
}
