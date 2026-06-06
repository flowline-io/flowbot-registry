package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
)

// TestAllModules verifies the full fx dependency graph resolves.
func TestAllModules(t *testing.T) {
	t.Parallel()

	app := fxtest.New(t, AllModules())
	app.RequireStart()
	app.RequireStop()
}

// TestAllModulesOption verifies AllModules returns a valid fx.Option.
func TestAllModulesOption(t *testing.T) {
	t.Parallel()

	assert.NotNil(t, AllModules())
	_ = fx.Options(AllModules())
}
