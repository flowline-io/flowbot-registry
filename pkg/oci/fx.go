package oci

import (
	"github.com/spf13/viper"
	"go.uber.org/fx"
)

// Module provides the OCI registry client via fx dependency injection.
var Module = fx.Module("oci",
	fx.Provide(func(v *viper.Viper) *Client {
		url := v.GetString("registry.url")
		if url == "" {
			url = "http://localhost:5000"
		}
		return NewClient(url)
	}),
)
