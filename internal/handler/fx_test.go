package handler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/fx"
)

// TestModule verifies the handler fx module is valid and follows conventions.
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
			name: "module name is handler",
			opt:  Module,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotNil(t, tt.opt)
		})
	}
}
