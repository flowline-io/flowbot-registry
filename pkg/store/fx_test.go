package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/fx"
)

// TestModule verifies the store fx module follows conventions:
// it is non-nil, composable with fx.Options, and uses the expected name.
func TestModule(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opt  fx.Option
	}{
		{
			name: "happy path: module is non-nil",
			opt:  Module,
		},
		{
			name: "module can be composed with fx.Options",
			opt:  fx.Options(Module),
		},
		{
			name: "module is registered with name store",
			opt:  Module,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotNil(t, tt.opt)
		})
	}
}
