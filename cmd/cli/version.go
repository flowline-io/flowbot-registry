package main

import (
	"fmt"
	"log/slog"

	"github.com/spf13/cobra"

	"github.com/flowline-io/flowbot-registry/version"
)

func versionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the CLI version",
		Long:  "Print the version of the flowbot CLI.",
		RunE:  runVersion,
	}

	return cmd
}

func runVersion(cmd *cobra.Command, _ []string) error {
	slog.Debug("version: printing", "version", version.Buildtags)
	_, err := fmt.Fprintf(cmd.OutOrStdout(), "flowbot version %s\n", version.Buildtags)
	if err != nil {
		return fmt.Errorf("print version: %w", err)
	}
	return nil
}
