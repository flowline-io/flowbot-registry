// Package main is the entry point for the flowbot-registry CLI
package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))

	rootCmd := &cobra.Command{
		Use:   "flowbot",
		Short: "Flowbot CLI tool",
		Long:  "Flowbot CLI for plugin management: publish, install, and search.",
	}

	pluginCmd := &cobra.Command{
		Use:   "plugin",
		Short: "Plugin management commands",
		Long:  "Scaffold, publish, install, and search plugins.",
	}
	pluginCmd.AddCommand(initCmd())
	pluginCmd.AddCommand(publishCmd())
	pluginCmd.AddCommand(installCmd())
	pluginCmd.AddCommand(searchCmd())
	rootCmd.AddCommand(pluginCmd)

	if err := rootCmd.Execute(); err != nil {
		slog.Error("command failed", "error", err)
		_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
