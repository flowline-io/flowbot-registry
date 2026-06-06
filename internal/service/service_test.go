package service

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseScopes(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantLen   int
		wantFirst ScopeEntry
		wantError bool
	}{
		{
			name:    "single scope pull",
			input:   "repository:community/my-plugin:pull",
			wantLen: 1,
			wantFirst: ScopeEntry{
				Type:    "repository",
				Name:    "community/my-plugin",
				Actions: []string{"pull"},
			},
			wantError: false,
		},
		{
			name:    "single scope push and pull",
			input:   "repository:org/plugin:pull,push",
			wantLen: 1,
			wantFirst: ScopeEntry{
				Type:    "repository",
				Name:    "org/plugin",
				Actions: []string{"pull", "push"},
			},
			wantError: false,
		},
		{
			name:    "multiple scopes",
			input:   "repository:ns/a:pull repository:ns/b:pull,push",
			wantLen: 2,
			wantFirst: ScopeEntry{
				Type:    "repository",
				Name:    "ns/a",
				Actions: []string{"pull"},
			},
			wantError: false,
		},
		{
			name:      "empty scope",
			input:     "",
			wantLen:   0,
			wantError: false,
		},
		{
			name:      "invalid format missing actions",
			input:     "repository:name",
			wantError: true,
		},
		{
			name:      "invalid format no type",
			input:     "name:pull",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			scopes, err := ParseScopes(tt.input)
			if tt.wantError {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Len(t, scopes, tt.wantLen)
				if tt.wantLen > 0 {
					assert.Equal(t, tt.wantFirst.Type, scopes[0].Type)
					assert.Equal(t, tt.wantFirst.Name, scopes[0].Name)
					assert.Equal(t, tt.wantFirst.Actions, scopes[0].Actions)
				}
			}
		})
	}
}
