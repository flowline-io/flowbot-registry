// Package main is the entry point for the flowbot-registry CLI
package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"sync"

	"github.com/spf13/cobra"
)

// cliLogHandler is a slog.Handler optimized for CLI output.
// It omits timestamps and simplifies the format for user-facing messages.
type cliLogHandler struct {
	mu     sync.Mutex
	writer io.Writer
	opts   slog.HandlerOptions
	attrs  []slog.Attr
}

func newCLILogHandler(w io.Writer, opts *slog.HandlerOptions) *cliLogHandler {
	h := &cliLogHandler{writer: w}
	if opts != nil {
		h.opts = *opts
	}
	return h
}

func (h *cliLogHandler) Enabled(_ context.Context, level slog.Level) bool {
	minLevel := slog.LevelInfo
	if h.opts.Level != nil {
		minLevel = h.opts.Level.Level()
	}
	return level >= minLevel
}

func (h *cliLogHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()

	var buf bytes.Buffer

	_, _ = fmt.Fprint(&buf, r.Message)
	for _, a := range h.attrs {
		_, _ = fmt.Fprintf(&buf, " %s=%s", a.Key, a.Value.String())
	}
	r.Attrs(func(a slog.Attr) bool {
		_, _ = fmt.Fprintf(&buf, " %s=%s", a.Key, a.Value.String())
		return true
	})
	_, _ = fmt.Fprint(&buf, "\n")

	_, err := h.writer.Write(buf.Bytes())
	return err
}

func (h *cliLogHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	newH := *h
	newH.attrs = append(append([]slog.Attr{}, h.attrs...), attrs...)
	return &newH
}

func (h *cliLogHandler) WithGroup(_ string) slog.Handler {
	return h
}

func main() {
	slog.SetDefault(slog.New(newCLILogHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))

	rootCmd := &cobra.Command{
		Use:           "flowbot",
		Short:         "Flowbot CLI tool",
		Long:          "Flowbot CLI for plugin management: publish, install, and search.",
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	pluginCmd := &cobra.Command{
		Use:   "plugin",
		Short: "Plugin management commands",
		Long:  "Scaffold, publish, install, and search plugins.",
	}
	pluginCmd.AddCommand(initCmd())
	pluginCmd.AddCommand(publishCmd())
	pluginCmd.AddCommand(registerCmd())
	pluginCmd.AddCommand(installCmd())
	pluginCmd.AddCommand(searchCmd())
	pluginCmd.AddCommand(loginCmd())
	rootCmd.AddCommand(pluginCmd)
	rootCmd.AddCommand(versionCmd())

	if err := rootCmd.Execute(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
