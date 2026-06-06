package jwt

import (
	"time"

	"github.com/spf13/viper"
	"go.uber.org/fx"
)

// Module provides the jwt token service via fx dependency injection.
var Module = fx.Module("jwt",
	fx.Provide(func(v *viper.Viper) (*TokenService, error) {
		return NewTokenService(
			v.GetString("auth.jwt_private_key_path"),
			v.GetString("auth.jwt_issuer"),
			time.Duration(v.GetInt("auth.jwt_expiration"))*time.Second,
		)
	}),
)
