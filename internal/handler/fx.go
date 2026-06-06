package handler

import "go.uber.org/fx"

// Module registers HTTP API routes via fx dependency injection.
var Module = fx.Module("handler",
	fx.Invoke(RegisterRoutes),
)
