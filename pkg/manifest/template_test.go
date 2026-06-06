package manifest

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGenerateManifestTemplate(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		plugin    string
		runtime   RuntimeKind
		wantErr   bool
		wantField string
	}{
		{
			name:      "grpc template contains runtime grpc",
			namespace: "community",
			plugin:    "my-plugin",
			runtime:   RuntimeGRPC,
			wantErr:   false,
			wantField: "grpc",
		},
		{
			name:      "wasm template contains runtime wasm",
			namespace: "community",
			plugin:    "my-wasm",
			runtime:   RuntimeWasm,
			wantErr:   false,
			wantField: "wasm",
		},
		{
			name:      "invalid runtime returns error",
			namespace: "community",
			plugin:    "bad",
			runtime:   "invalid",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GenerateManifestYAML(tt.namespace, tt.plugin, tt.runtime)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Contains(t, string(got), "name: "+tt.namespace+"/"+tt.plugin)
			assert.Contains(t, string(got), "runtime: "+string(tt.runtime))
			assert.Contains(t, string(got), tt.wantField)
		})
	}
}

func TestGenerateGoMod(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		plugin    string
		runtime   RuntimeKind
		check     string
	}{
		{
			name:      "go.mod contains module path",
			namespace: "community",
			plugin:    "my-plugin",
			runtime:   RuntimeGRPC,
			check:     "module github.com/community/my-plugin",
		},
		{
			name:      "go.mod declares go version",
			namespace: "org",
			plugin:    "test-plugin",
			runtime:   RuntimeGRPC,
			check:     "go 1.",
		},
		{
			name:      "go.mod requires go-plugin sdk for grpc",
			namespace: "org",
			plugin:    "grpc-plugin",
			runtime:   RuntimeGRPC,
			check:     "module",
		},
		{
			name:      "wasm go.mod has no require line",
			namespace: "org",
			plugin:    "wasm-plugin",
			runtime:   RuntimeWasm,
			check:     "module github.com/org/wasm-plugin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateGoMod(tt.namespace, tt.plugin, tt.runtime)
			assert.Contains(t, string(got), tt.check)
			if tt.runtime == RuntimeWasm {
				assert.NotContains(t, string(got), "require")
			}
		})
	}
}

func TestGenerateMainGo(t *testing.T) {
	tests := []struct {
		name    string
		runtime RuntimeKind
		check   string
	}{
		{
			name:    "grpc main.go has ServeModule",
			runtime: RuntimeGRPC,
			check:   "ServeModule",
		},
		{
			name:    "wasm main.go exports alloc",
			runtime: RuntimeWasm,
			check:   "alloc",
		},
		{
			name:    "wasm main.go exports free",
			runtime: RuntimeWasm,
			check:   "free",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateMainGo(tt.runtime)
			assert.Contains(t, string(got), tt.check)
		})
	}
}

func TestPluginNameFromRef(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantNs   string
		wantName string
	}{
		{
			name:     "full namespace/name",
			input:    "community/my-plugin",
			wantNs:   "community",
			wantName: "my-plugin",
		},
		{
			name:     "bare name defaults to default namespace",
			input:    "my-plugin",
			wantNs:   "default",
			wantName: "my-plugin",
		},
		{
			name:     "org with hyphen",
			input:    "my-org/github-stars",
			wantNs:   "my-org",
			wantName: "github-stars",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns, name := PluginNameFromRef(tt.input)
			assert.Equal(t, tt.wantNs, ns)
			assert.Equal(t, tt.wantName, name)
		})
	}
}

func TestInitFileSet(t *testing.T) {
	tests := []struct {
		name      string
		namespace string
		plugin    string
		runtime   RuntimeKind
	}{
		{
			name:      "grpc creates 4 files",
			namespace: "community",
			plugin:    "my-plugin",
			runtime:   RuntimeGRPC,
		},
		{
			name:      "wasm creates 3 files",
			namespace: "community",
			plugin:    "my-wasm",
			runtime:   RuntimeWasm,
		},
		{
			name:      "wasm has plugin.yaml and main.go but no cmd dir",
			namespace: "org",
			plugin:    "wasm-plugin",
			runtime:   RuntimeWasm,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			files, err := InitFileSet(tt.namespace, tt.plugin, tt.runtime)
			require.NoError(t, err)

			names := make(map[string]bool)
			for _, f := range files {
				names[filepath.ToSlash(f.Path)] = true
			}

			assert.True(t, names["plugin.yaml"])
			assert.True(t, names["go.mod"])
			assert.True(t, names["main.go"])

			if tt.runtime == RuntimeGRPC {
				assert.True(t, names["cmd/server/main.go"])
			}
		})
	}
}
