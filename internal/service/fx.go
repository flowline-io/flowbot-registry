package service

import (
	"time"

	"github.com/spf13/viper"
	"go.uber.org/fx"

	"github.com/flowline-io/flowbot-registry/pkg/jwt"
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
		func(a *store.Adapter, jwtSvc *jwt.UserTokenService, v *viper.Viper) *UserService {
			exp := v.GetInt("auth.refresh_token_expiration")
			if exp == 0 {
				exp = 604800
			}
			return NewUserService(a, jwtSvc, time.Duration(exp)*time.Second)
		},
	),
)
