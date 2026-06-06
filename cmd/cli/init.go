package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/flowline-io/flowbot-registry/pkg/manifest"
	"github.com/spf13/cobra"
)

var initArgs struct {
	runtime string
}

func initCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init <namespace/name>",
		Short: "Scaffold a new plugin project",
		Long: `Scaffold a new plugin project directory with plugin.yaml, go.mod, and skeleton source files.

The argument must be a fully qualified name in <namespace>/<name> format.

Example:
  flowbot plugin init community/my-plugin
  flowbot plugin init my-org/github-stars`,
		Args: cobra.ExactArgs(1),
		RunE: runInitCmd,
	}
	cmd.Flags().StringVar(&initArgs.runtime, "runtime", "grpc", "Plugin runtime type: grpc or wasm")
	return cmd
}

func runInitCmd(_ *cobra.Command, args []string) error {
	fullName := args[0]
	ns, name, err := parseInitName(fullName)
	if err != nil {
		return err
	}

	runtime := initArgs.runtime
	if runtime != "grpc" && runtime != "wasm" {
		return fmt.Errorf("invalid runtime %q: must be grpc or wasm", runtime)
	}

	slog.Info("init: scaffolding plugin", "dir", name, "namespace", ns, "runtime", runtime)

	if err := runInit(name, ns, name, runtime); err != nil {
		return fmt.Errorf("generate files: %w", err)
	}

	_, _ = fmt.Println()
	_, _ = fmt.Printf("Next steps:\n  cd %s\n  <edit code>\n  flowbot plugin publish\n", name)
	return nil
}

// runInit generates and writes scaffold files for a plugin project.
func runInit(targetDir, namespace, name, runtime string) error {
	rt := manifest.RuntimeKind(runtime)
	if rt != manifest.RuntimeGRPC && rt != manifest.RuntimeWasm {
		return fmt.Errorf("invalid runtime %q: must be grpc or wasm", runtime)
	}
	if _, err := os.Stat(targetDir); err == nil {
		return fmt.Errorf("directory %q already exists", targetDir)
	}
	files, err := manifest.InitFileSet(namespace, name, rt)
	if err != nil {
		return err
	}
	for _, f := range files {
		fullPath := filepath.Join(targetDir, f.Path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			return fmt.Errorf("create dir for %s: %w", f.Path, err)
		}
		if err := os.WriteFile(fullPath, f.Content, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", f.Path, err)
		}
	}
	return nil
}

// parseInitName validates and splits a full plugin name into namespace and name.
func parseInitName(fullName string) (namespace, name string, err error) {
	if fullName == "" {
		return "", "", fmt.Errorf("plugin name is required in format <namespace>/<name>")
	}
	parts := strings.SplitN(fullName, "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid plugin name %q: must be in format <namespace>/<name>", fullName)
	}
	if parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid plugin name %q: namespace and name must not be empty", fullName)
	}
	return parts[0], parts[1], nil
}
