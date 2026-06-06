// Package main is the entry point for the flowbot-registry HTTP API server.
package main

import (
	"log/slog"
	"os"

	"go.uber.org/fx"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))
	fx.New(AllModules()).Run()
}
