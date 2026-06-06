package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/flowline-io/flowbot-registry/pkg/oci"
	"github.com/spf13/cobra"
)

var registerArgs struct {
	registryURL string
	storeURL    string
	apiKey      string
}

func registerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "register <namespace/name:version>",
		Short: "Retry metadata registration for an existing OCI image",
		Long: `Re-register an already-pushed OCI image with the store API.
Use when a previous publish succeeded at OCI push but failed at metadata registration.

Example:
  flowbot plugin register community/my-plugin:1.2.0`,
		Args: cobra.ExactArgs(1),
		RunE: runRegister,
	}

	cmd.Flags().StringVar(&registerArgs.registryURL, "registry", envOrFlag("FLOWBOT_REGISTRY_URL", "ghcr.io"), "OCI registry URL (auto-discovered from store by default)")
	cmd.Flags().StringVar(&registerArgs.storeURL, "store-url", envOrFlag("FLOWBOT_STORE_URL", "http://localhost:8128"), "Store API URL")
	cmd.Flags().StringVar(&registerArgs.apiKey, "api-key", envOrFlag("FLOWBOT_API_KEY", ""), "API key for store authentication")

	return cmd
}

func runRegister(cmd *cobra.Command, args []string) error {
	ns, name, version, err := parseRegisterRef(args[0])
	if err != nil {
		return err
	}

	// Resolve registry URL from store if not explicitly set.
	registerArgs.registryURL = resolveRegistryURLFromStore(cmd, registerArgs.storeURL, registerArgs.registryURL)

	ociRef := fmt.Sprintf("%s/%s/%s:%s", strings.TrimRight(oci.StripScheme(registerArgs.registryURL), "/"), ns, name, version)

	slog.Info("register: checking OCI manifest", "ref", ociRef)

	ociClient := oci.NewClient(registerArgs.registryURL)
	digest, err := ociClient.HeadManifest(context.Background(), ociRef)
	if err != nil {
		return fmt.Errorf("fetch OCI manifest: %w", err)
	}

	_, _ = fmt.Printf("  Fetching OCI manifest... OK (%s)\n", digest)

	slog.Info("register: registering metadata", "namespace", ns, "name", name, "version", version, "digest", digest)

	if err := registerPublishMetadata(registerArgs.storeURL, registerArgs.apiKey, ociRef, ns, name, version, digest.String()); err != nil {
		return fmt.Errorf("register metadata: %w", err)
	}

	_, _ = fmt.Println("  Registering metadata... OK")
	_, _ = fmt.Println("Plugin metadata registered.")

	return nil
}

// parseRegisterRef validates and splits a register ref into namespace, name, version.
func parseRegisterRef(ref string) (namespace, name, version string, err error) {
	if idx := strings.LastIndex(ref, ":"); idx >= 0 {
		version = ref[idx+1:]
		ref = ref[:idx]
	} else {
		return "", "", "", fmt.Errorf("version is required: %s (must be namespace/name:version)", ref)
	}

	parts := strings.SplitN(ref, "/", 2)
	if len(parts) != 2 {
		return "", "", "", fmt.Errorf("invalid ref %q: must be namespace/name:version", ref)
	}

	return parts[0], parts[1], version, nil
}
