package main

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/flowline-io/flowbot-registry/version"
)

func TestVersionCmd(t *testing.T) {
	tests := []struct {
		name       string
		versionVal string
		wantOutput string
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:       "default version",
			versionVal: "v0.92.0",
			wantOutput: "flowbot version v0.92.0\n",
			wantErr:    false,
		},
		{
			name:       "version from ldflags",
			versionVal: "v1.2.3",
			wantOutput: "flowbot version v1.2.3\n",
			wantErr:    false,
		},
		{
			name:       "dev version",
			versionVal: "dev",
			wantOutput: "flowbot version dev\n",
			wantErr:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			old := version.Buildtags
			version.Buildtags = tt.versionVal
			t.Cleanup(func() { version.Buildtags = old })

			cmd := versionCmd()
			var buf bytes.Buffer
			cmd.SetOut(&buf)
			cmd.SetErr(&buf)

			err := cmd.Execute()
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrMsg)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantOutput, buf.String())
			}
		})
	}
}
