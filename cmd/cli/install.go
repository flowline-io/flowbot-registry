package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var installArgs struct {
	storeURL string
	destPath string
}

func installCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install [namespace/name:version]",
		Short: "Install a plugin from the registry",
		Long: `Downloads and installs a plugin from the plugin registry.

Example:
  flowbot plugin install my-org/my-plugin:1.2.0
  flowbot plugin install my-org/my-plugin`,
		Args: cobra.ExactArgs(1),
		RunE: runInstall,
	}

	cmd.Flags().StringVar(&installArgs.storeURL, "store-url", "http://localhost:8080", "Store API URL")
	cmd.Flags().StringVar(&installArgs.destPath, "dest", "/data/flowbot/plugins", "Destination directory for installed plugins")

	return cmd
}

func runInstall(_ *cobra.Command, args []string) error {
	ref := args[0]
	namespace, name, version := parseInstallRef(ref)

	slog.Info("install: started",
		"namespace", namespace, "name", name, "version", version,
	)

	_, _ = fmt.Printf("Installing plugin: %s/%s v%s\n", namespace, name, version)

	pv, err := fetchPluginVersionFromStore(installArgs.storeURL, namespace, name, version)
	if err != nil {
		slog.Error("install: fetch version failed", "error", err)
		return fmt.Errorf("fetch plugin version: %w", err)
	}

	_, _ = fmt.Printf("OCI ref: %s\n", pv.OciImageRef)
	_, _ = fmt.Printf("OCI digest: %s\n", pv.OciDigest)

	slog.Debug("install: resolved plugin version",
		"image_ref", pv.OciImageRef, "digest", pv.OciDigest,
	)

	destDir := filepath.Join(installArgs.destPath, namespace, name, version)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return fmt.Errorf("create destination directory: %w", err)
	}

	slog.Debug("install: pulling artifact", "ref", pv.OciImageRef, "dest", destDir)

	err = pullOCIArtifact(context.Background(), pv.OciImageRef, destDir)
	if err != nil {
		slog.Error("install: pull failed", "error", err, "ref", pv.OciImageRef)
		return fmt.Errorf("pull OCI artifact: %w", err)
	}

	manifestData, err := os.ReadFile(filepath.Join(destDir, "plugin.yaml"))
	if err != nil {
		return fmt.Errorf("read installed plugin.yaml: %w", err)
	}

	_, _ = fmt.Println("Installation successful.")
	_, _ = fmt.Printf("Plugin: %s/%s v%s\n", namespace, name, version)
	_, _ = fmt.Printf("Location: %s\n", destDir)
	_, _ = fmt.Printf("Manifest:\n%s\n", string(manifestData))

	slog.Info("install: success",
		"namespace", namespace, "name", name, "version", version,
		"dest", destDir,
	)

	return nil
}

func parseInstallRef(ref string) (string, string, string) {
	var namespace, name, version string

	version = "latest"

	if idx := strings.LastIndex(ref, ":"); idx >= 0 {
		version = ref[idx+1:]
		ref = ref[:idx]
	}

	if idx := strings.Index(ref, "/"); idx >= 0 {
		namespace = ref[:idx]
		name = ref[idx+1:]
	} else {
		namespace = "default"
		name = ref
	}

	return namespace, name, version
}

type pluginVersionInfo struct {
	OciImageRef string `json:"oci_image_ref"`
	OciDigest   string `json:"oci_digest"`
	Version     string `json:"version"`
}

func fetchPluginVersionFromStore(storeURL string, namespace string, name string, version string) (*pluginVersionInfo, error) {
	apiURL := fmt.Sprintf("%s/api/v1/plugins/%s/%s/versions/%s",
		strings.TrimRight(storeURL, "/"), namespace, name, version)

	slog.Debug("install: fetching plugin version", "url", apiURL)

	return doJSONGet[pluginVersionInfo](apiURL)
}

func pullOCIArtifact(ctx context.Context, ref string, destDir string) error {
	_ = ctx
	_, _ = fmt.Printf("Pulling artifact: %s -> %s\n", ref, destDir)
	return fmt.Errorf("%w: OCI pull via oras-go not yet wired (target: %s)", errNotImplemented, ref)
}
