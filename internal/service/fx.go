package service

import (
	"github.com/spf13/viper"
	"go.uber.org/fx"

	"github.com/flowline-io/flowbot-registry/pkg/oci"
	"github.com/flowline-io/flowbot-registry/pkg/store"
)

// Module provides the service layer via fx dependency injection.
var Module = fx.Module("service",
	fx.Provide(
		NewAuthService,
		func(a *store.Adapter, ociClient *oci.Client, v *viper.Viper) *PluginService {
			url := v.GetString("registry.url")
			if url == "" {
				url = "http://localhost:5000"
			}
			return NewPluginService(a, ociClient, url)
		},
	),
)
