package jwt

import (
	"time"

	"github.com/spf13/viper"
	"go.uber.org/fx"
)

// Module provides the jwt token services via fx dependency injection.
var Module = fx.Module("jwt",
	fx.Provide(
		func(v *viper.Viper) (*TokenService, error) {
			return NewTokenService(
				v.GetString("auth.jwt_private_key_path"),
				v.GetString("auth.jwt_issuer"),
				time.Duration(v.GetInt("auth.jwt_expiration"))*time.Second,
			)
		},
		func(v *viper.Viper) (*UserTokenService, error) {
			exp := v.GetInt("auth.access_token_expiration")
			if exp == 0 {
				exp = 3600
			}
			return NewUserTokenService(
				v.GetString("auth.jwt_private_key_path"),
				v.GetString("auth.jwt_issuer"),
				time.Duration(exp)*time.Second,
			)
		},
	),
)
