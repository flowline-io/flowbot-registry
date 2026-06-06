package manifest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseManifest(t *testing.T) {
	tests := []struct {
		name      string
		input     []byte
		wantError bool
		wantName  string
		wantVer   string
	}{
		{
			name: "valid manifest",
			input: []byte(`name: my-plugin
version: "1.0.0"
description: A test plugin
author: test-author
runtime: grpc
grpc:
  binary: ./plugin
`),
			wantError: false,
			wantName:  "my-plugin",
			wantVer:   "1.0.0",
		},
		{
			name: "missing name field",
			input: []byte(`version: "1.0.0"
description: no name here
`),
			wantError: true,
		},
		{
			name:      "missing version field",
			input:     []byte(`name: my-plugin`),
			wantError: true,
		},
		{
			name:      "invalid yaml",
			input:     []byte(`{{invalid yaml`),
			wantError: true,
		},
		{
			name: "valid grpc manifest with full schema",
			input: []byte(`name: community/my-plugin
version: "1.0.0"
description: A gRPC plugin
author: dev@example.com
runtime: grpc
provides:
  module: true
  abilities:
    - capability: bookmark
      operations: [list, get]
grpc:
  binary: ./plugin-server
  args: ["--port", "0"]
config_schema:
  type: object
  properties:
    api_key:
      type: string
`),
			wantError: false,
			wantName:  "community/my-plugin",
			wantVer:   "1.0.0",
		},
		{
			name: "valid wasm manifest with permissions",
			input: []byte(`name: community/my-wasm
version: "0.1.0"
runtime: wasm
provides:
  module: true
wasm:
  module: ./plugin.wasm
  permissions:
    memory:
      max: "64MB"
    execution:
      timeout: "30s"
`),
			wantError: false,
			wantName:  "community/my-wasm",
			wantVer:   "0.1.0",
		},
		{
			name: "invalid runtime kind",
			input: []byte(`name: my-plugin
version: "1.0.0"
runtime: invalid
`),
			wantError: true,
		},
		{
			name: "grpc runtime without grpc config",
			input: []byte(`name: my-plugin
version: "1.0.0"
runtime: grpc
`),
			wantError: true,
		},
		{
			name: "wasm runtime without wasm config",
			input: []byte(`name: my-plugin
version: "1.0.0"
runtime: wasm
`),
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := ParseManifest(tt.input)
			if tt.wantError {
				require.Error(t, err)
				assert.Nil(t, m)
			} else {
				require.NoError(t, err)
				require.NotNil(t, m)
				assert.Equal(t, tt.wantName, m.Name)
				assert.Equal(t, tt.wantVer, m.Version)
			}
		})
	}
}
