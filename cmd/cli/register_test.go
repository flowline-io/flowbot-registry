package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRegisterRef(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantNs   string
		wantName string
		wantVer  string
		wantErr  bool
	}{
		{
			name:     "full ref with version",
			input:    "community/my-plugin:1.2.0",
			wantNs:   "community",
			wantName: "my-plugin",
			wantVer:  "1.2.0",
			wantErr:  false,
		},
		{
			name:    "ref without version fails",
			input:   "community/my-plugin",
			wantErr: true,
		},
		{
			name:    "bare name fails",
			input:   "my-plugin:1.0.0",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns, name, ver, err := parseRegisterRef(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantNs, ns)
			assert.Equal(t, tt.wantName, name)
			assert.Equal(t, tt.wantVer, ver)
		})
	}
}
