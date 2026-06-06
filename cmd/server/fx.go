package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gofiber/fiber/v3"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/spf13/viper"
	"go.uber.org/fx"

	"github.com/flowline-io/flowbot-registry/internal/ent"
	"github.com/flowline-io/flowbot-registry/internal/handler"
	"github.com/flowline-io/flowbot-registry/internal/service"
	"github.com/flowline-io/flowbot-registry/internal/web"
	"github.com/flowline-io/flowbot-registry/pkg/jwt"
	"github.com/flowline-io/flowbot-registry/pkg/oci"
	"github.com/flowline-io/flowbot-registry/pkg/store"
)

// AllModules returns all fx options needed to start the server.
func AllModules() fx.Option {
	return fx.Options(
		fx.Provide(
			newViper,
			newEntClient,
			newFiberApp,
			newRegistryURL,
		),
		fx.Invoke(runMigrations),
		store.Module,
		jwt.Module,
		oci.Module,
		service.Module,
		handler.Module,
		web.Module,
	)
}

func newViper() *viper.Viper {
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")

	v.SetDefault("server.listen", ":8128")
	v.SetDefault("auth.jwt_private_key_path", "./private.pem")
	v.SetDefault("auth.jwt_expiration", 3600)
	v.SetDefault("auth.jwt_issuer", "flowbot-registry")
	v.SetDefault("registry.url", "http://localhost:5000")

	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			slog.Error("config file error", "error", err)
		}
	}

	return v
}

func newRegistryURL(v *viper.Viper) string {
	url := v.GetString("registry.url")
	if url == "" {
		url = "http://localhost:5000"
	}
	return url
}

func newEntClient(v *viper.Viper, lc fx.Lifecycle) (*ent.Client, error) {
	sql.Register("postgres", stdlib.GetDefaultDriver())

	dsn := v.GetString("database.dsn")
	if dsn == "" {
		return nil, fmt.Errorf("database.dsn is not configured")
	}

	slog.Info("connecting to database")
	client, err := ent.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	lc.Append(fx.Hook{
		OnStop: func(_ context.Context) error {
			return client.Close()
		},
	})

	return client, nil
}

func runMigrations(client *ent.Client) error {
	slog.Info("running database migrations")
	return client.Schema.Create(context.Background())
}

func newFiberApp(v *viper.Viper, lc fx.Lifecycle) *fiber.App {
	app := fiber.New(fiber.Config{
		AppName: "flowbot-registry",
	})

	listen := v.GetString("server.listen")
	if listen == "" {
		listen = ":8128"
	}

	lc.Append(fx.Hook{
		OnStart: func(_ context.Context) error {
			go func() {
				slog.Info("server starting", "listen", listen)
				if err := app.Listen(listen); err != nil {
					slog.Error("server error", "error", err)
				}
			}()
			_, _ = fmt.Printf("flowbot-registry server started on %s\n", listen)
			return nil
		},
		OnStop: func(_ context.Context) error {
			slog.Info("shutting down")
			if err := app.Shutdown(); err != nil {
				slog.Error("shutdown error", "error", err)
			}
			slog.Info("server stopped")
			return nil
		},
	})

	return app
}
