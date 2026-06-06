package web

import "go.uber.org/fx"

// Module registers web UI routes via fx dependency injection.
var Module = fx.Module("web",
	fx.Invoke(RegisterWebRoutes),
)
