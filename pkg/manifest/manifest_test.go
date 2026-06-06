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
