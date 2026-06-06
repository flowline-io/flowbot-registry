// Package main is the entry point for the flowbot-registry HTTP API server.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/flowline-io/flowbot-registry/internal/ent"
	"github.com/flowline-io/flowbot-registry/internal/handler"
	"github.com/flowline-io/flowbot-registry/internal/service"
	"github.com/flowline-io/flowbot-registry/internal/web"
	"github.com/flowline-io/flowbot-registry/pkg/jwt"
	"github.com/flowline-io/flowbot-registry/pkg/oci"
	"github.com/flowline-io/flowbot-registry/pkg/store"

	"github.com/gofiber/fiber/v3"
	"github.com/spf13/viper"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg := loadConfig()

	dsn := cfg.GetString("database.dsn")
	if dsn == "" {
		dsn = "postgres://flowbot:flowbot@localhost:5432/flowbot_registry?sslmode=disable"
	}

	slog.Info("connecting to database")

	entClient, err := ent.Open("postgres", dsn)
	if err != nil {
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer entClient.Close()

	slog.Info("running database migrations")
	ctx := context.Background()
	if err := entClient.Schema.Create(ctx); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	adapter := store.NewAdapter(entClient)

	jwtKeyPath := cfg.GetString("auth.jwt_private_key_path")
	if jwtKeyPath == "" {
		jwtKeyPath = "./private.pem"
	}

	jwtSvc, err := jwt.NewTokenService(
		jwtKeyPath,
		cfg.GetString("auth.jwt_issuer"),
		time.Duration(cfg.GetInt("auth.jwt_expiration"))*time.Second,
	)
	if err != nil {
		slog.Error("failed to initialize JWT service", "error", err, "key_path", jwtKeyPath)
		os.Exit(1)
	}

	registryURL := cfg.GetString("registry.url")
	if registryURL == "" {
		registryURL = "http://localhost:5000"
	}

	slog.Info("initializing services", "registry_url", registryURL)
	ociClient := oci.NewClient(registryURL)

	authSvc := service.NewAuthService(jwtSvc, adapter)
	pluginSvc := service.NewPluginService(adapter, ociClient, registryURL)

	app := fiber.New(fiber.Config{
		AppName: "flowbot-registry",
	})

	handler.RegisterRoutes(app, authSvc, pluginSvc)
	web.RegisterWebRoutes(app, adapter)

	listen := cfg.GetString("server.listen")
	if listen == "" {
		listen = ":8080"
	}

	go func() {
		slog.Info("server starting", "listen", listen)
		if err := app.Listen(listen); err != nil {
			slog.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	_, _ = fmt.Printf("flowbot-registry server started on %s\n", listen)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	slog.Info("shutting down", "signal", sig.String())

	if err := app.Shutdown(); err != nil {
		slog.Error("shutdown error", "error", err)
	}

	slog.Info("server stopped")
}

func loadConfig() *viper.Viper {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")

	v.SetDefault("server.listen", ":8080")
	v.SetDefault("auth.jwt_private_key_path", "./private.pem")
	v.SetDefault("auth.jwt_expiration", 3600)
	v.SetDefault("auth.jwt_issuer", "flowbot-registry")
	v.SetDefault("registry.url", "http://localhost:5000")

	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			slog.Error("config file error", "error", err)
		}
	}

	return v
}
