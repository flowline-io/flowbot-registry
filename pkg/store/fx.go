package store

import "go.uber.org/fx"

// Module provides the store adapter via fx dependency injection.
// Provides both *Adapter and the StoreQuerier interface for consumers
// that depend on the interface (e.g. web handlers).
var Module = fx.Module("store",
	fx.Provide(
		NewAdapter,
		func(a *Adapter) StoreQuerier { return a },
	),
)
