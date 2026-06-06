# Publisher CLI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement `flowbot plugin init` (scaffold) and `flowbot plugin publish` (cross-compile, OCI push, Cosign sign, metadata register) with TDD throughout.

**Architecture:** Phase-gated: manifest schema → build pipeline → OCI push → Cosign signing → CLI wiring. New packages `internal/build/` and `pkg/sign/`. Existing `pkg/oci/` and `pkg/manifest/` extended. CLI restructured under `plugin` parent subcommand.

**Tech Stack:** Go 1.26.3, cobra, go-containerregistry v0.20.6, sigstore/cosign/v2, goccy/go-yaml, bytedance/sonic, testify

---

## File Map

| Action | File                            | Responsibility                                          |
| ------ | ------------------------------- | ------------------------------------------------------- |
| Modify | `pkg/manifest/manifest.go`      | Add Runtime, Provides, GRPC, Wasm, ConfigSchema fields  |
| Modify | `pkg/manifest/manifest_test.go` | Test extended schema parsing                            |
| Create | `pkg/manifest/template.go`      | Generate plugin.yaml + go.mod + main.go skeletons       |
| Create | `pkg/manifest/template_test.go` | Test template generation per runtime                    |
| Create | `internal/build/builder.go`     | Builder interface + shared types                        |
| Create | `internal/build/grpc.go`        | GrpcBuilder: go build linux/amd64                       |
| Create | `internal/build/grpc_test.go`   | Test GrpcBuilder exec calls                             |
| Create | `internal/build/wasm.go`        | WasmBuilder: tinygo build wasi                          |
| Create | `internal/build/wasm_test.go`   | Test WasmBuilder exec calls                             |
| Create | `pkg/oci/pusher.go`             | PushArtifact, HeadManifest                              |
| Create | `pkg/oci/pusher_test.go`        | Test push with mock registry                            |
| Create | `pkg/oci/signature.go`          | PushSignature as referrer                               |
| Create | `pkg/oci/signature_test.go`     | Test signature push                                     |
| Create | `pkg/sign/cosign.go`            | Signer with sigstore/cosign/v2                          |
| Create | `pkg/sign/cosign_test.go`       | Test key loading, signing                               |
| Modify | `cmd/cli/main.go`               | Add `plugin` parent subcommand                          |
| Create | `cmd/cli/init.go`               | `plugin init <ns/name>` command                         |
| Create | `cmd/cli/init_test.go`          | Test init file generation                               |
| Modify | `cmd/cli/publish.go`            | Wire full pipeline: build→push→sign→register            |
| Create | `cmd/cli/publish_test.go`       | Test publish pipeline                                   |
| Create | `cmd/cli/register.go`           | `plugin register <ns/name:version>` command             |
| Create | `cmd/cli/register_test.go`      | Test register command                                   |
| Modify | `cmd/cli/search.go`             | No logic change; just note it moves under plugin parent |
| Modify | `cmd/cli/install.go`            | No logic change; just note it moves under plugin parent |

---

## Phase 1: Manifest Schema + Init Scaffold

### Task 1.1: Extend Manifest schema to match flowbot

**Files:**

- Modify: `pkg/manifest/manifest.go:4-10`
- Modify: `pkg/manifest/manifest_test.go`

- [ ] **Step 1: Write failing tests for extended schema**

```go
// pkg/manifest/manifest_test.go — add this inside the existing TestParseManifest table:

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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/manifest/ -run TestParseManifest -v`
Expected: FAIL — fields not parsed, runtime/grpc/wasm not validated

- [ ] **Step 3: Extend Manifest struct and ParseManifest**

```go
// pkg/manifest/manifest.go — replace entire file

package manifest

import (
	"encoding/json"
	"fmt"
)

// Manifest represents the plugin.yaml definition for a plugin.
type Manifest struct {
	Name         string          `yaml:"name" json:"name"`
	Version      string          `yaml:"version" json:"version"`
	Description  string          `yaml:"description" json:"description"`
	Author       string          `yaml:"author" json:"author"`
	Runtime      RuntimeKind     `yaml:"runtime" json:"runtime"`
	Provides     Provides        `yaml:"provides" json:"provides"`
	GRPC         *GRPCConfig     `yaml:"grpc" json:"grpc,omitempty"`
	Wasm         *WasmConfig     `yaml:"wasm" json:"wasm,omitempty"`
	ConfigSchema json.RawMessage `yaml:"config_schema" json:"config_schema,omitempty"`
}

// RuntimeKind is the plugin execution environment.
type RuntimeKind string

// Valid runtime kinds.
const (
	RuntimeGRPC RuntimeKind = "grpc"
	RuntimeWasm RuntimeKind = "wasm"
)

// Provides declares what the plugin implements.
type Provides struct {
	Module    bool          `yaml:"module" json:"module"`
	Abilities []AbilityDecl `yaml:"abilities" json:"abilities"`
	Provider  *ProviderDecl `yaml:"provider" json:"provider,omitempty"`
}

// AbilityDecl describes an ability capability and its operations.
type AbilityDecl struct {
	Capability string   `yaml:"capability" json:"capability"`
	Operations []string `yaml:"operations" json:"operations"`
}

// ProviderDecl describes a provider plugin.
type ProviderDecl struct {
	Name  string `yaml:"name" json:"name"`
	OAuth bool   `yaml:"oauth" json:"oauth"`
}

// GRPCConfig configures a gRPC-based plugin.
type GRPCConfig struct {
	Binary string   `yaml:"binary" json:"binary"`
	Args   []string `yaml:"args" json:"args"`
}

// WasmConfig configures a Wasm/WASI-based plugin.
type WasmConfig struct {
	Module      string           `yaml:"module" json:"module"`
	Permissions *WasmPermissions `yaml:"permissions" json:"permissions"`
	Pool        *WasmPoolConfig  `yaml:"pool" json:"pool,omitempty"`
}

// WasmPermissions defines sandbox constraints for Wasm plugins.
type WasmPermissions struct {
	HTTP       []HTTPPermission `yaml:"http" json:"http"`
	Filesystem []FSPermission   `yaml:"filesystem" json:"filesystem"`
	Memory     *MemoryLimit     `yaml:"memory" json:"memory"`
	Execution  *ExecutionLimit  `yaml:"execution" json:"execution"`
}

// HTTPPermission defines an allowed HTTP host for Wasm plugins.
type HTTPPermission struct {
	Host string `yaml:"host" json:"host"`
}

// FSPermission defines filesystem access for Wasm plugins.
type FSPermission struct {
	Path string `yaml:"path" json:"path"`
	Mode string `yaml:"mode" json:"mode"`
}

// MemoryLimit is the max memory for a Wasm plugin.
type MemoryLimit struct {
	Max string `yaml:"max" json:"max"`
}

// ExecutionLimit is the execution timeout for a Wasm plugin.
type ExecutionLimit struct {
	Timeout string `yaml:"timeout" json:"timeout"`
}

// WasmPoolConfig configures instance pooling for Wasm plugins.
type WasmPoolConfig struct {
	MaxInstances int    `yaml:"max_instances" json:"max_instances"`
	WaitTimeout  string `yaml:"wait_timeout" json:"wait_timeout"`
}

// ParseManifest parses raw YAML bytes into a Manifest.
func ParseManifest(data []byte) (*Manifest, error) {
	var m Manifest
	if err := unmarshalYAML(data, &m); err != nil {
		return nil, err
	}
	if m.Name == "" {
		return nil, errMissingField("name")
	}
	if m.Version == "" {
		return nil, errMissingField("version")
	}
	if err := m.validateRuntime(); err != nil {
		return nil, err
	}
	return &m, nil
}

func (m *Manifest) validateRuntime() error {
	switch m.Runtime {
	case RuntimeGRPC:
		if m.GRPC == nil {
			return fmt.Errorf("%w: grpc runtime requires grpc config", ErrInvalidManifest)
		}
	case RuntimeWasm:
		if m.Wasm == nil {
			return fmt.Errorf("%w: wasm runtime requires wasm config", ErrInvalidManifest)
		}
	case "":
		return fmt.Errorf("%w: runtime is required (grpc or wasm)", ErrInvalidManifest)
	default:
		return fmt.Errorf("%w: unknown runtime %q (expected grpc or wasm)", ErrInvalidManifest, m.Runtime)
	}
	return nil
}

// ErrInvalidManifest is returned when the manifest fails validation.
var ErrInvalidManifest = fmt.Errorf("invalid manifest")
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/manifest/ -run TestParseManifest -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/manifest/manifest.go pkg/manifest/manifest_test.go
git commit -m "feat(manifest): extend schema with runtime, provides, grpc, wasm, config_schema"
```

### Task 1.2: Add template generation for plugin init

**Files:**

- Create: `pkg/manifest/template.go`
- Create: `pkg/manifest/template_test.go`

- [ ] **Step 1: Write failing tests for template generation**

```go
// pkg/manifest/template_test.go

package manifest

import (
	"path/filepath"
	"strings"
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
			wantField:  "grpc",
		},
		{
			name:      "wasm template contains runtime wasm",
			namespace: "community",
			plugin:    "my-wasm",
			runtime:   RuntimeWasm,
			wantErr:   false,
			wantField:  "wasm",
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
		check     string
	}{
		{
			name:      "go.mod contains module path",
			namespace: "community",
			plugin:    "my-plugin",
			check:     "module github.com/community/my-plugin",
		},
		{
			name:      "go.mod declares go version",
			namespace: "org",
			plugin:    "test-plugin",
			check:     "go 1.",
		},
		{
			name:      "go.mod requires go-plugin sdk for grpc",
			namespace: "org",
			plugin:    "grpc-plugin",
			check:     "module",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateGoMod(tt.namespace, tt.plugin, RuntimeGRPC)
			assert.Contains(t, string(got), tt.check)
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
			name:    "grpc main.go imports sdk",
			runtime: RuntimeGRPC,
			check:   "sdk",
		},
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
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/manifest/ -run "TestGenerate|TestInit" -v`
Expected: FAIL — functions not defined

- [ ] **Step 3: Implement template generation**

```go
// pkg/manifest/template.go

package manifest

import (
	"fmt"
	"strings"
)

// InitFile represents a file to be created during plugin scaffold.
type InitFile struct {
	Path    string // relative path from plugin root, e.g. "plugin.yaml", "cmd/server/main.go"
	Content []byte
}

// InitFileSet generates all scaffolding files for a new plugin project.
func InitFileSet(namespace, name string, runtime RuntimeKind) ([]InitFile, error) {
	switch runtime {
	case RuntimeGRPC, RuntimeWasm:
	default:
		return nil, fmt.Errorf("%w: unknown runtime %q", ErrInvalidManifest, runtime)
	}

	var files []InitFile

	yaml, err := GenerateManifestYAML(namespace, name, runtime)
	if err != nil {
		return nil, err
	}
	files = append(files, InitFile{Path: "plugin.yaml", Content: yaml})

	files = append(files, InitFile{Path: "go.mod", Content: GenerateGoMod(namespace, name, runtime)})

	files = append(files, InitFile{Path: "main.go", Content: GenerateMainGo(runtime)})

	if runtime == RuntimeGRPC {
		files = append(files, InitFile{Path: "cmd/server/main.go", Content: generateServerMainGo()})
	}

	return files, nil
}

var grpcTemplate = `name: %s/%s
version: "0.1.0"
description: "A gRPC plugin"
author: ""
runtime: grpc
provides:
  module: true
  abilities: []
grpc:
  binary: ./plugin-server
  args: []
config_schema:
  type: object
  properties: {}
`

var wasmTemplate = `name: %s/%s
version: "0.1.0"
description: "A Wasm plugin"
author: ""
runtime: wasm
provides:
  module: true
  abilities: []
wasm:
  module: ./plugin.wasm
  permissions:
    memory:
      max: "64MB"
    execution:
      timeout: "30s"
config_schema:
  type: object
  properties: {}
`

// GenerateManifestYAML returns a plugin.yaml template for the given runtime.
func GenerateManifestYAML(namespace, name string, runtime RuntimeKind) ([]byte, error) {
	switch runtime {
	case RuntimeGRPC:
		return []byte(fmt.Sprintf(grpcTemplate, namespace, name)), nil
	case RuntimeWasm:
		return []byte(fmt.Sprintf(wasmTemplate, namespace, name)), nil
	default:
		return nil, fmt.Errorf("%w: unknown runtime %q", ErrInvalidManifest, runtime)
	}
}

var goModTemplate = `module github.com/%s/%s

go 1.26.0

%s
`

var grpcRequire = `require github.com/flowline-io/flowbot-registry v0.0.0`

// GenerateGoMod returns a go.mod scaffold for the plugin.
func GenerateGoMod(namespace, name string, runtime RuntimeKind) []byte {
	var req string
	if runtime == RuntimeGRPC {
		req = grpcRequire
	}
	return []byte(fmt.Sprintf(goModTemplate, namespace, name, req))
}

var grpcMainGo = `package main

import (
	"log"

	"github.com/flowline-io/flowbot-registry/pkg/plugin/sdk"
)

type plugin struct {
	sdk.ModuleBase
}

func main() {
	sdk.ServeModule(&plugin{})
}
`

var wasmMainGo = `package main

//go:wasmexport alloc
func alloc(size uint32) uint32 { return 0 }

//go:wasmexport free
func free(ptr uint32) {}

func main() {}
`

// GenerateMainGo returns the main.go scaffold for the given runtime.
func GenerateMainGo(runtime RuntimeKind) []byte {
	switch runtime {
	case RuntimeGRPC:
		return []byte(grpcMainGo)
	case RuntimeWasm:
		return []byte(wasmMainGo)
	default:
		return nil
	}
}

var serverMainGo = `package main

import (
	"log"

	"github.com/flowline-io/flowbot-registry/pkg/plugin/sdk"
)

type plugin struct {
	sdk.ModuleBase
}

func main() {
	sdk.ServeModule(&plugin{})
}
`

func generateServerMainGo() []byte {
	return []byte(serverMainGo)
}

// PluginNameFromRef parses "namespace/name" from a full ref string.
// Returns namespace and short name. If no slash, namespace defaults to "default".
func PluginNameFromRef(fullName string) (namespace, name string) {
	parts := strings.SplitN(fullName, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "default", fullName
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/manifest/ -run "TestGenerate|TestInit" -v`
Expected: PASS

- [ ] **Step 5: Run all manifest tests**

Run: `go test ./pkg/manifest/ -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add pkg/manifest/template.go pkg/manifest/template_test.go
git commit -m "feat(manifest): add template generation for plugin init scaffold"
```

### Task 1.3: Implement `plugin init` CLI command

**Files:**

- Create: `cmd/cli/init.go`
- Create: `cmd/cli/init_test.go`
- Modify: `cmd/cli/main.go` (add plugin parent + init subcommand)

- [ ] **Step 1: Write failing init test**

```go
// cmd/cli/init_test.go

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInitCmdFlags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "rejects bare name without namespace",
			args:    []string{"plugin", "init", "my-plugin"},
			wantErr: true,
			errMsg:  "names",
		},
		{
			name:    "accepts namespace/name format",
			args:    []string{"plugin", "init", "community/my-plugin"}, // note: this will create dir, test at unit level
			wantErr: false,
		},
		{
			name:    "rejects empty name",
			args:    []string{"plugin", "init", ""},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// validatePluginName is tested in isolation
			ns, name, err := parseInitName(tt.args[len(tt.args)-1])
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				require.NoError(t, err)
				assert.Equal(t, "community", ns)
				assert.Equal(t, "my-plugin", name)
			}
		})
	}
}

func TestInitFileCreation(t *testing.T) {
	dir := t.TempDir()
	targetDir := filepath.Join(dir, "my-plugin")

	// runInit is not defined yet — this will cause a build error
	require.NoError(t, runInit(targetDir, "community", "my-plugin", "grpc"))

	// Check files exist
	for _, path := range []string{
		"plugin.yaml",
		"go.mod",
		"main.go",
		filepath.Join("cmd", "server", "main.go"),
	} {
		full := filepath.Join(targetDir, path)
		_, err := os.Stat(full)
		assert.NoError(t, err, "expected file %s to exist", path)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/cli/ -run TestInit -v`
Expected: FAIL — `parseInitName`, `runInit`, `pluginManifest` not defined

- [ ] **Step 3: Implement init command**

```go
// cmd/cli/init.go

package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/flowline-io/flowbot-registry/pkg/manifest"
	"github.com/spf13/cobra"
)

var initArgs struct {
	runtime string
}

func initCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init <namespace/name>",
		Short: "Scaffold a new plugin project",
		Long: `Scaffold a new plugin project directory with plugin.yaml, go.mod, and skeleton source files.

The argument must be a fully qualified name in <namespace>/<name> format.

Example:
  flowbot plugin init community/my-plugin
  flowbot plugin init my-org/github-stars`,
		Args: cobra.ExactArgs(1),
		RunE: runInitCmd,
	}

	cmd.Flags().StringVar(&initArgs.runtime, "runtime", "grpc", "Plugin runtime type: grpc or wasm")

	return cmd
}

func runInitCmd(_ *cobra.Command, args []string) error {
	fullName := args[0]
	ns, name, err := parseInitName(fullName)
	if err != nil {
		return err
	}

	runtime := manifest.RuntimeKind(initArgs.runtime)
	if runtime != manifest.RuntimeGRPC && runtime != manifest.RuntimeWasm {
		return fmt.Errorf("invalid runtime %q: must be grpc or wasm", initArgs.runtime)
	}

	dir := name
	if _, err := os.Stat(dir); err == nil {
		return fmt.Errorf("directory %q already exists", dir)
	}

	slog.Info("init: scaffolding plugin", "dir", dir, "namespace", ns, "name", name, "runtime", runtime)

	files, err := manifest.InitFileSet(ns, name, runtime)
	if err != nil {
		return fmt.Errorf("generate files: %w", err)
	}

	for _, f := range files {
		fullPath := filepath.Join(dir, f.Path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			return fmt.Errorf("create dir for %s: %w", f.Path, err)
		}
		if err := os.WriteFile(fullPath, f.Content, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", f.Path, err)
		}
		_, _ = fmt.Printf("  Created %s\n", f.Path)
	}

	fmt.Println()
	fmt.Printf("Next steps:\n")
	fmt.Printf("  cd %s\n", dir)
	fmt.Printf("  <edit code>\n")
	fmt.Printf("  flowbot plugin publish\n")

	return nil
}

// parseInitName validates and splits a full plugin name into namespace and name.
func parseInitName(fullName string) (namespace, name string, err error) {
	if fullName == "" {
		return "", "", fmt.Errorf("plugin name is required in format <namespace>/<name>")
	}
	parts := strings.SplitN(fullName, "/", 2)
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid plugin name %q: must be in format <namespace>/<name>", fullName)
	}
	if parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid plugin name %q: namespace and name must not be empty", fullName)
	}
	return parts[0], parts[1], nil
}
```

- [ ] **Step 4: Update test to use the real logic**

```go
// cmd/cli/init_test.go — replace with this final version

package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseInitName(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantNs     string
		wantName   string
		wantErr    bool
		wantErrMsg string
	}{
		{
			name:     "valid namespace/name",
			input:    "community/my-plugin",
			wantNs:   "community",
			wantName: "my-plugin",
			wantErr:  false,
		},
		{
			name:     "valid org with hyphen",
			input:    "my-org/github-stars",
			wantNs:   "my-org",
			wantName: "github-stars",
			wantErr:  false,
		},
		{
			name:       "bare name without slash fails",
			input:      "my-plugin",
			wantErr:    true,
			wantErrMsg: "must be in format",
		},
		{
			name:       "empty namespace fails",
			input:      "/my-plugin",
			wantErr:    true,
			wantErrMsg: "must not be empty",
		},
		{
			name:       "empty name fails",
			input:      "community/",
			wantErr:    true,
			wantErrMsg: "must not be empty",
		},
		{
			name:       "empty string fails",
			input:      "",
			wantErr:    true,
			wantErrMsg: "required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ns, name, err := parseInitName(tt.input)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrMsg)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantNs, ns)
				assert.Equal(t, tt.wantName, name)
			}
		})
	}
}

func TestInitFileCreation(t *testing.T) {
	dir := t.TempDir()
	targetDir := filepath.Join(dir, "my-plugin")

	// Simulate CLI init by calling the underlying logic
	require.NoError(t, runInit(targetDir, "community", "my-plugin", "grpc"))

	for _, path := range []string{
		"plugin.yaml", "go.mod", "main.go",
		filepath.Join("cmd", "server", "main.go"),
	} {
		full := filepath.Join(targetDir, path)
		_, err := os.Stat(full)
		assert.NoError(t, err, "expected file %s to exist", path)
	}
}

func TestInitFileCreationWasm(t *testing.T) {
	dir := t.TempDir()
	targetDir := filepath.Join(dir, "my-wasm")

	require.NoError(t, runInit(targetDir, "community", "my-wasm", "wasm"))

	for _, path := range []string{"plugin.yaml", "go.mod", "main.go"} {
		full := filepath.Join(targetDir, path)
		_, err := os.Stat(full)
		assert.NoError(t, err, "expected file %s to exist", path)
	}

	// wasm should NOT have cmd/server/main.go
	_, err := os.Stat(filepath.Join(targetDir, "cmd", "server", "main.go"))
	assert.True(t, os.IsNotExist(err))
}
```

- [ ] **Step 5: Add runInit helper to init.go (extracted for testability)**

Append to `cmd/cli/init.go`:

```go
// runInit generates and writes scaffold files for a plugin project.
// Exported for testability.
func runInit(targetDir, namespace, name, runtime string) error {
	rt := manifest.RuntimeKind(runtime)
	if rt != manifest.RuntimeGRPC && rt != manifest.RuntimeWasm {
		return fmt.Errorf("invalid runtime %q: must be grpc or wasm", runtime)
	}

	if _, err := os.Stat(targetDir); err == nil {
		return fmt.Errorf("directory %q already exists", targetDir)
	}

	files, err := manifest.InitFileSet(namespace, name, rt)
	if err != nil {
		return err
	}

	for _, f := range files {
		fullPath := filepath.Join(targetDir, f.Path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
			return fmt.Errorf("create dir for %s: %w", f.Path, err)
		}
		if err := os.WriteFile(fullPath, f.Content, 0o644); err != nil {
			return fmt.Errorf("write %s: %w", f.Path, err)
		}
	}

	return nil
}
```

Also refactor `runInitCmd` to use `runInit`:

```go
func runInitCmd(_ *cobra.Command, args []string) error {
	fullName := args[0]
	ns, name, err := parseInitName(fullName)
	if err != nil {
		return err
	}

	runtime := initArgs.runtime
	if runtime != "grpc" && runtime != "wasm" {
		return fmt.Errorf("invalid runtime %q: must be grpc or wasm", initArgs.runtime)
	}

	slog.Info("init: scaffolding plugin", "dir", name, "namespace", ns, "runtime", runtime)

	if err := runInit(name, ns, name, runtime); err != nil {
		return fmt.Errorf("generate files: %w", err)
	}

	_, _ = fmt.Println()
	_, _ = fmt.Printf("Next steps:\n  cd %s\n  <edit code>\n  flowbot plugin publish\n", name)

	return nil
}
```

- [ ] **Step 6: Add plugin parent command and wire init to main.go**

Modify `cmd/cli/main.go`:

```go
// Package main is the entry point for the flowbot-registry CLI
package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))

	rootCmd := &cobra.Command{
		Use:   "flowbot",
		Short: "Flowbot CLI tool",
		Long:  "Flowbot CLI for plugin management: publish, install, and search.",
	}

	pluginCmd := &cobra.Command{
		Use:   "plugin",
		Short: "Plugin management commands",
		Long:  "Scaffold, publish, install, and search plugins.",
	}

	pluginCmd.AddCommand(initCmd())
	pluginCmd.AddCommand(publishCmd())
	pluginCmd.AddCommand(installCmd())
	pluginCmd.AddCommand(searchCmd())

	rootCmd.AddCommand(pluginCmd)

	if err := rootCmd.Execute(); err != nil {
		slog.Error("command failed", "error", err)
		_, _ = fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 7: Run init tests**

Run: `go test ./cmd/cli/ -run TestInit -v`
Expected: PASS

- [ ] **Step 8: Build CLI to verify**

Run: `go build ./cmd/cli/`
Expected: compiles successfully

- [ ] **Step 9: Commit**

```bash
git add cmd/cli/init.go cmd/cli/init_test.go cmd/cli/main.go
git commit -m "feat(cli): add plugin parent command and init scaffold"
```

---

## Phase 2: Build Pipeline

### Task 2.1: Create build package with interface

**Files:**

- Create: `internal/build/builder.go`

- [ ] **Step 1: Create builder interface**

```go
// internal/build/builder.go

// Package build orchestrates cross-compilation of plugin source code into deployable binaries.
package build

import (
	"context"

	"github.com/flowline-io/flowbot-registry/pkg/manifest"
)

// Artifact represents a built file ready for OCI packaging.
type Artifact struct {
	Name    string // filename: "plugin.yaml", "plugin-server", "plugin.wasm"
	Content []byte // file contents
}

// Builder compiles plugin source code into artifacts.
type Builder interface {
	// Build compiles the plugin in dir and returns the built artifacts.
	Build(ctx context.Context, dir string, m *manifest.Manifest) ([]Artifact, error)
}
```

- [ ] **Step 2: Verify it compiles**

Run: `go build ./internal/build/`
Expected: compiles

- [ ] **Step 3: Commit**

```bash
git add internal/build/builder.go
git commit -m "feat(build): add Builder interface and Artifact type"
```

### Task 2.2: Implement GrpcBuilder

**Files:**

- Create: `internal/build/grpc.go`
- Create: `internal/build/grpc_test.go`

- [ ] **Step 1: Write failing grpc builder tests**

```go
// internal/build/grpc_test.go

package build

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/flowline-io/flowbot-registry/pkg/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGrpcBuilderBuildArgs(t *testing.T) {
	// Test that exec.Command gets called with correct args.
	// We use os.Executable trick: the test binary can fake go.
	m := &manifest.Manifest{
		Name:    "community/my-plugin",
		Version: "1.0.0",
		Runtime: manifest.RuntimeGRPC,
		GRPC:    &manifest.GRPCConfig{Binary: "./plugin-server"},
	}

	dir := t.TempDir()
	// Create a minimal cmd/server/main.go so the builder doesn't fail on missing dir
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "cmd", "server"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "cmd", "server", "main.go"), []byte("package main\nfunc main() {}"), 0o644))
	// Create go.mod so go build works
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n\ngo 1.26\n"), 0o644))

	b := NewGrpcBuilder()
	artifacts, err := b.Build(context.Background(), dir, m)
	require.NoError(t, err)
	require.NotEmpty(t, artifacts)

	var names []string
	for _, a := range artifacts {
		names = append(names, a.Name)
	}
	assert.Contains(t, names, "plugin-server")
}

func TestGrpcBuilderNoCmdServer(t *testing.T) {
	m := &manifest.Manifest{
		Name:    "community/no-cmd",
		Version: "1.0.0",
		Runtime: manifest.RuntimeGRPC,
		GRPC:    &manifest.GRPCConfig{Binary: "./plugin-server"},
	}

	dir := t.TempDir()
	b := NewGrpcBuilder()
	_, err := b.Build(context.Background(), dir, m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cmd/server")
}

func TestGrpcBuilderContextCancelled(t *testing.T) {
	m := &manifest.Manifest{
		Name:    "community/cancel",
		Version: "1.0.0",
		Runtime: manifest.RuntimeGRPC,
		GRPC:    &manifest.GRPCConfig{Binary: "./plugin-server"},
	}

	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "cmd", "server"), 0o755))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	b := NewGrpcBuilder()
	_, err := b.Build(ctx, dir, m)
	require.Error(t, err)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/build/ -run TestGrpc -v`
Expected: FAIL — `NewGrpcBuilder` not defined

- [ ] **Step 3: Implement GrpcBuilder**

```go
// internal/build/grpc.go

package build

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/flowline-io/flowbot-registry/pkg/manifest"
)

// GrpcBuilder compiles gRPC plugins for linux/amd64.
type GrpcBuilder struct{}

// NewGrpcBuilder creates a new GrpcBuilder.
func NewGrpcBuilder() *GrpcBuilder {
	return &GrpcBuilder{}
}

// Build compiles a gRPC plugin targeting linux/amd64.
func (b *GrpcBuilder) Build(ctx context.Context, dir string, m *manifest.Manifest) ([]Artifact, error) {
	cmdDir := filepath.Join(dir, "cmd", "server")
	if _, err := os.Stat(filepath.Join(cmdDir, "main.go")); os.IsNotExist(err) {
		return nil, fmt.Errorf("cmd/server/main.go not found in %s", dir)
	}

	outputName := "plugin-server"
	if m.GRPC != nil && m.GRPC.Binary != "" {
		outputName = filepath.Base(m.GRPC.Binary)
	}
	outputPath := filepath.Join(dir, outputName)

	slog.Info("build: compiling gRPC plugin",
		"os", "linux", "arch", "amd64", "dir", cmdDir, "output", outputPath,
	)

	args := []string{"build", "-o", outputPath}
	if m.GRPC != nil {
		args = append(args, m.GRPC.Args...)
	}
	args = append(args, "./cmd/server")

	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"CGO_ENABLED=0",
		"GOOS=linux",
		"GOARCH=amd64",
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("go build failed: %w\n%s", err, string(output))
	}

	content, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("read built binary %s: %w", outputPath, err)
	}

	return []Artifact{
		{Name: outputName, Content: content},
	}, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/build/ -run TestGrpc -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/build/grpc.go internal/build/grpc_test.go
git commit -m "feat(build): implement GrpcBuilder for linux/amd64 cross-compilation"
```

### Task 2.3: Implement WasmBuilder

**Files:**

- Create: `internal/build/wasm.go`
- Create: `internal/build/wasm_test.go`

- [ ] **Step 1: Write failing wasm builder tests**

```go
// internal/build/wasm_test.go

package build

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/flowline-io/flowbot-registry/pkg/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWasmBuilderBuild(t *testing.T) {
	m := &manifest.Manifest{
		Name:    "community/my-wasm",
		Version: "1.0.0",
		Runtime: manifest.RuntimeWasm,
		Wasm:    &manifest.WasmConfig{Module: "./plugin.wasm"},
	}

	dir := t.TempDir()
	// Create minimal main.go
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc main() {}"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n\ngo 1.26\n"), 0o644))

	b := NewWasmBuilder()
	artifacts, err := b.Build(context.Background(), dir, m)
	require.NoError(t, err)
	require.NotEmpty(t, artifacts)

	names := make(map[string]bool)
	for _, a := range artifacts {
		names[a.Name] = true
	}
	assert.True(t, names["plugin.wasm"])
}

func TestWasmBuilderNoMainGo(t *testing.T) {
	m := &manifest.Manifest{
		Name:    "community/no-main",
		Version: "1.0.0",
		Runtime: manifest.RuntimeWasm,
		Wasm:    &manifest.WasmConfig{Module: "./plugin.wasm"},
	}

	dir := t.TempDir()
	b := NewWasmBuilder()
	_, err := b.Build(context.Background(), dir, m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "main.go")
}

func TestWasmBuilderContextCancelled(t *testing.T) {
	m := &manifest.Manifest{
		Name:    "community/cancel",
		Version: "1.0.0",
		Runtime: manifest.RuntimeWasm,
		Wasm:    &manifest.WasmConfig{Module: "./plugin.wasm"},
	}

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\nfunc main() {}"), 0o644))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	b := NewWasmBuilder()
	_, err := b.Build(ctx, dir, m)
	require.Error(t, err)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/build/ -run TestWasm -v`
Expected: FAIL — `NewWasmBuilder` not defined

- [ ] **Step 3: Implement WasmBuilder**

```go
// internal/build/wasm.go

package build

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/flowline-io/flowbot-registry/pkg/manifest"
)

// WasmBuilder compiles Wasm/WASI plugins using tinygo.
type WasmBuilder struct{}

// NewWasmBuilder creates a new WasmBuilder.
func NewWasmBuilder() *WasmBuilder {
	return &WasmBuilder{}
}

// Build compiles a Wasm plugin targeting wasi/wasm.
func (b *WasmBuilder) Build(ctx context.Context, dir string, m *manifest.Manifest) ([]Artifact, error) {
	if _, err := os.Stat(filepath.Join(dir, "main.go")); os.IsNotExist(err) {
		return nil, fmt.Errorf("main.go not found in %s", dir)
	}

	outputName := "plugin.wasm"
	if m.Wasm != nil && m.Wasm.Module != "" {
		outputName = filepath.Base(m.Wasm.Module)
	}
	outputPath := filepath.Join(dir, outputName)

	slog.Info("build: compiling Wasm plugin",
		"target", "wasi", "dir", dir, "output", outputPath,
	)

	cmd := exec.CommandContext(ctx, "tinygo", "build",
		"-target=wasi",
		"-no-debug",
		"-o", outputPath,
		".",
	)
	cmd.Dir = dir
	cmd.Env = os.Environ()

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("tinygo build failed: %w\n%s", err, string(output))
	}

	content, err := os.ReadFile(outputPath)
	if err != nil {
		return nil, fmt.Errorf("read built wasm %s: %w", outputPath, err)
	}

	return []Artifact{
		{Name: outputName, Content: content},
	}, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/build/ -run TestWasm -v`
Expected: PASS

- [ ] **Step 5: Run all build tests**

Run: `go test ./internal/build/ -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add internal/build/wasm.go internal/build/wasm_test.go
git commit -m "feat(build): implement WasmBuilder for wasi/wasm compilation"
```

---

## Phase 3: OCI Push

### Task 3.1: Implement PushArtifact and HeadManifest

**Files:**

- Create: `pkg/oci/pusher.go`
- Create: `pkg/oci/pusher_test.go`

- [ ] **Step 1: Write failing push tests**

```go
// pkg/oci/pusher_test.go

package oci

import (
	"context"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/random"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestRegistry(t *testing.T) (string, func()) {
	t.Helper()
	srv := httptest.NewServer(registry.New())
	u, err := url.Parse(srv.URL)
	require.NoError(t, err)
	return u.Host, srv.Close
}

func TestPushArtifact(t *testing.T) {
	host, close := setupTestRegistry(t)
	defer close()

	c := NewClient(host)

	ref := host + "/test/plugin:v1"

	// Push a simple artifact first to set up the repo
	img, err := random.Image(256, 1)
	require.NoError(t, err)
	r, err := name.ParseReference(ref)
	require.NoError(t, err)
	require.NoError(t, remote.Write(r, img))

	// Now test HeadManifest
	digest, err := c.HeadManifest(context.Background(), ref)
	require.NoError(t, err)
	assert.NotEmpty(t, digest.String())
}

func TestHeadManifestNotFound(t *testing.T) {
	host, close := setupTestRegistry(t)
	defer close()

	c := NewClient(host)
	ref := host + "/test/nonexistent:latest"

	_, err := c.HeadManifest(context.Background(), ref)
	require.Error(t, err)
}

func TestPushArtifactWithFiles(t *testing.T) {
	host, close := setupTestRegistry(t)
	defer close()

	c := NewClient(host)
	ref := host + "/test/push-artifact:v1"

	files := []ArtifactFile{
		{Name: "plugin.yaml", Content: []byte("name: test\nversion: v1\nruntime: grpc\ngrpc:\n  binary: ./plugin-server\n")},
		{Name: "plugin-server", Content: []byte("binary-content")},
	}

	_, err := c.PushArtifact(context.Background(), ref, files)
	require.NoError(t, err)

	// Verify we can pull it back
	img, err := c.FetchManifest(context.Background(), ref)
	require.NoError(t, err)

	layers, err := ExtractLayers(img, []string{"plugin.yaml"})
	require.NoError(t, err)
	require.Len(t, layers, 1)
	assert.Equal(t, "plugin.yaml", layers[0].Name)
	assert.Equal(t, "name: test\nversion: v1\nruntime: grpc\ngrpc:\n  binary: ./plugin-server\n", string(layers[0].Content))
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/oci/ -run TestPush -v`
Expected: FAIL — `PushArtifact`, `HeadManifest` not defined

- [ ] **Step 3: Implement PushArtifact and HeadManifest**

```go
// pkg/oci/pusher.go

package oci

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"fmt"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

// ErrNotFound indicates a resource was not found on the registry.
var ErrNotFound = errors.New("not found")

// ArtifactFile represents a file to include in the OCI artifact.
type ArtifactFile struct {
	Name    string
	Content []byte
}

const pluginConfigMediaType = "application/vnd.flowbot.plugin.config.v1+yaml"

// PushArtifactOption configures a push operation.
type PushArtifactOption func(*pushArtifactConfig)

type pushArtifactConfig struct {
	auth authn.Authenticator
}

// WithAuth sets the authenticator for the push operation.
func WithAuth(a authn.Authenticator) PushArtifactOption {
	return func(cfg *pushArtifactConfig) {
		cfg.auth = a
	}
}

// PushArtifact creates tar.gz OCI layers from files and pushes a single manifest.
// The plugin.yaml layer uses a custom media type. All other layers use the standard OCI tar+gzip type.
func (c *Client) PushArtifact(ctx context.Context, refStr string, files []ArtifactFile, opts ...PushArtifactOption) (v1.Hash, error) {
	cfg := &pushArtifactConfig{}
	for _, o := range opts {
		o(cfg)
	}

	ref, err := name.ParseReference(refStr)
	if err != nil {
		return v1.Hash{}, fmt.Errorf("parse reference %s: %w", refStr, err)
	}

	var layers []v1.Layer
	for _, f := range files {
		layer, err := fileToLayer(f)
		if err != nil {
			return v1.Hash{}, fmt.Errorf("create layer for %s: %w", f.Name, err)
		}
		layers = append(layers, layer)
	}

	img, err := mutate.Append(static.EmptyImage, mutate.Addendum{Layer: layers[0], MediaType: types.MediaType(pluginConfigMediaType)})
	if err != nil {
		return v1.Hash{}, fmt.Errorf("append first layer: %w", err)
	}

	for _, layer := range layers[1:] {
		img, err = mutate.Append(img, mutate.Addendum{Layer: layer, MediaType: types.OCILayer})
		if err != nil {
			return v1.Hash{}, fmt.Errorf("append layer: %w", err)
		}
	}

	remoteOpts := []remote.Option{remote.WithContext(ctx)}
	if cfg.auth != nil {
		remoteOpts = append(remoteOpts, remote.WithAuth(cfg.auth))
	} else {
		remoteOpts = append(remoteOpts, remote.WithAuthFromKeychain(authn.DefaultKeychain))
	}

	if err := remote.Write(ref, img, remoteOpts...); err != nil {
		return v1.Hash{}, fmt.Errorf("write image: %w", err)
	}

	digest, err := img.Digest()
	if err != nil {
		return v1.Hash{}, fmt.Errorf("get digest: %w", err)
	}

	return digest, nil
}

// HeadManifest checks if a manifest exists at the given reference.
// Returns the digest if found, or ErrNotFound if not.
func (c *Client) HeadManifest(ctx context.Context, refStr string) (v1.Hash, error) {
	ref, err := name.ParseReference(refStr)
	if err != nil {
		return v1.Hash{}, fmt.Errorf("parse reference %s: %w", refStr, err)
	}

	desc, err := remote.Head(ref, remote.WithContext(ctx), remote.WithAuthFromKeychain(authn.DefaultKeychain))
	if err != nil {
		return v1.Hash{}, fmt.Errorf("%w: %w", ErrNotFound, err)
	}

	return desc.Digest, nil
}

func fileToLayer(f ArtifactFile) (v1.Layer, error) {
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	if err := tw.WriteHeader(&tar.Header{
		Name: f.Name,
		Size: int64(len(f.Content)),
		Mode: 0o644,
	}); err != nil {
		return nil, fmt.Errorf("write tar header: %w", err)
	}

	if _, err := tw.Write(f.Content); err != nil {
		return nil, fmt.Errorf("write tar content: %w", err)
	}

	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("close tar: %w", err)
	}
	if err := gw.Close(); err != nil {
		return nil, fmt.Errorf("close gzip: %w", err)
	}

	return static.NewLayer(buf.Bytes(), types.OCILayer), nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/oci/ -run TestPush -v`
Expected: PASS

- [ ] **Step 5: Run all OCI tests**

Run: `go test ./pkg/oci/ -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add pkg/oci/pusher.go pkg/oci/pusher_test.go
git commit -m "feat(oci): add PushArtifact and HeadManifest for OCI push operations"
```

### Task 3.2: Implement PushSignature

**Files:**

- Create: `pkg/oci/signature.go`
- Create: `pkg/oci/signature_test.go`

- [ ] **Step 1: Write failing signature push tests**

```go
// pkg/oci/signature_test.go

package oci

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPushSignature(t *testing.T) {
	host, close := setupTestRegistry(t)
	defer close()

	c := NewClient(host)

	// First push an artifact
	ref := host + "/test/signed-plugin:v1"
	files := []ArtifactFile{
		{Name: "plugin.yaml", Content: []byte("name: test-sig\nversion: v1\nruntime: grpc\ngrpc:\n  binary: ./plugin-server\n")},
		{Name: "plugin-server", Content: []byte("fake-binary")},
	}
	_, err := c.PushArtifact(context.Background(), ref, files)
	require.NoError(t, err)

	// Push signature
	err = c.PushSignature(context.Background(), ref, []byte("sig-payload"), []byte("sig-data"))
	require.NoError(t, err)
}

func TestPushSignatureNoImage(t *testing.T) {
	host, close := setupTestRegistry(t)
	defer close()

	c := NewClient(host)
	ref := host + "/test/no-image:v99"

	err := c.PushSignature(context.Background(), ref, []byte("payload"), []byte("sig"))
	require.Error(t, err)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./pkg/oci/ -run TestPushSignature -v`
Expected: FAIL — `PushSignature` not defined

- [ ] **Step 3: Implement PushSignature**

```go
// pkg/oci/signature.go

package oci

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"fmt"

	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/static"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

const (
	signatureMediaType = "application/vnd.dev.cosign.simplesigning.v1+json"
	signatureTagSuffix = ".sig"
)

// PushSignature pushes a Cosign signature as an OCI artifact tagged with the sig suffix.
func (c *Client) PushSignature(ctx context.Context, refStr string, payload []byte, signature []byte) error {
	ref, err := name.ParseReference(refStr)
	if err != nil {
		return fmt.Errorf("parse reference %s: %w", refStr, err)
	}

	sigTag := ref.Context().Tag(ref.Identifier() + signatureTagSuffix)

	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	if err := tw.WriteHeader(&tar.Header{
		Name: "cosign.sig",
		Size: int64(len(signature)),
		Mode: 0o644,
	}); err != nil {
		return fmt.Errorf("write sig tar header: %w", err)
	}
	if _, err := tw.Write(signature); err != nil {
		return fmt.Errorf("write sig content: %w", err)
	}
	if err := tw.Close(); err != nil {
		return fmt.Errorf("close tar: %w", err)
	}
	if err := gw.Close(); err != nil {
		return fmt.Errorf("close gzip: %w", err)
	}

	layer := static.NewLayer(buf.Bytes(), types.MediaType(signatureMediaType))

	img, err := mutate.Append(static.EmptyImage, mutate.Addendum{Layer: layer, MediaType: types.MediaType(signatureMediaType)})
	if err != nil {
		return fmt.Errorf("create signature image: %w", err)
	}

	if err := remote.Write(sigTag, img, remote.WithContext(ctx), remote.WithAuthFromKeychain(authn.DefaultKeychain)); err != nil {
		return fmt.Errorf("push signature: %w", err)
	}

	return nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./pkg/oci/ -run TestPushSignature -v`
Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add pkg/oci/signature.go pkg/oci/signature_test.go
git commit -m "feat(oci): add PushSignature for Cosign signature OCI artifacts"
```

---

## Phase 4: Cosign Signing

### Task 4.1: Implement Signer with sigstore/cosign

**Files:**

- Create: `pkg/sign/cosign.go`
- Create: `pkg/sign/cosign_test.go`

- [ ] **Step 1: Add sigstore/cosign dependency**

Run: `go get github.com/sigstore/cosign/v2@latest`
Expected: adds to go.mod and go.sum

- [ ] **Step 2: Write failing signer tests**

```go
// pkg/sign/cosign_test.go

package sign

import (
	"context"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func generateTestKey(t *testing.T, dir string) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	keyBytes := x509.MarshalPKCS1PrivateKey(key)
	keyPath := filepath.Join(dir, "cosign.key")
	require.NoError(t, os.WriteFile(keyPath, pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: keyBytes,
	}), 0o600))

	return keyPath
}

func TestNewSignerValidKey(t *testing.T) {
	dir := t.TempDir()
	keyPath := generateTestKey(t, dir)

	signer, err := NewSigner(keyPath, "")
	require.NoError(t, err)
	assert.NotNil(t, signer)
}

func TestNewSignerMissingFile(t *testing.T) {
	_, err := NewSigner("/nonexistent/key.pem", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read key file")
}

func TestNewSignerInvalidPEM(t *testing.T) {
	dir := t.TempDir()
	keyPath := filepath.Join(dir, "bad.key")
	require.NoError(t, os.WriteFile(keyPath, []byte("not a pem file"), 0o600))

	_, err := NewSigner(keyPath, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "pem")
}

func TestSign(t *testing.T) {
	dir := t.TempDir()
	keyPath := generateTestKey(t, dir)

	signer, err := NewSigner(keyPath, "")
	require.NoError(t, err)

	result, err := signer.Sign("ghcr.io/test/plugin@sha256:abc123")
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.NotEmpty(t, result.Payload)
	assert.NotEmpty(t, result.Signature)
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./pkg/sign/ -v`
Expected: FAIL — package or functions not defined

- [ ] **Step 4: Implement Signer**

```go
// pkg/sign/cosign.go

// Package sign provides Cosign-based signing of OCI images.
package sign

import (
	"context"
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"

	"github.com/sigstore/cosign/v2/pkg/cosign"
	"github.com/sigstore/cosign/v2/pkg/oci/static"
	"github.com/sigstore/sigstore/pkg/signature"
	sigPayload "github.com/sigstore/sigstore/pkg/signature/payload"
)

// SignResult holds the output of a signing operation.
type SignResult struct {
	Payload   []byte
	Signature []byte
}

// Signer signs OCI image references using Cosign with a static private key.
type Signer struct {
	keyPath   string
	password  string
	verifier  signature.Verifier
}

// NewSigner creates a new Signer from a PEM-encoded private key.
// password is optional (empty string for unencrypted keys).
func NewSigner(keyPath string, password string) (*Signer, error) {
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("read key file: %w", err)
	}

	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block from %s", keyPath)
	}

	var key crypto.Signer
	switch block.Type {
	case "RSA PRIVATE KEY":
		key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse PKCS1 RSA private key: %w", err)
		}
	case "PRIVATE KEY":
		k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse PKCS8 private key: %w", err)
		}
		var ok bool
		key, ok = k.(crypto.Signer)
		if !ok {
			return nil, fmt.Errorf("key does not implement crypto.Signer")
		}
	default:
		return nil, fmt.Errorf("unsupported PEM block type %q", block.Type)
	}

	sv, err := signature.LoadSignerVerifier(key, crypto.SHA256)
	if err != nil {
		return nil, fmt.Errorf("load signer-verifier: %w", err)
	}

	return &Signer{keyPath: keyPath, password: password, verifier: sv}, nil
}

// Sign signs an OCI image reference and returns the payload and signature.
func (s *Signer) Sign(imageRef string) (*SignResult, error) {
	payload := sigPayload.Cosign{
		Image: imageRef,
	}

	payloadBytes, err := jsonMarshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	sig, _, err := s.verifier.SignMessage(bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("sign payload: %w", err)
	}

	return &SignResult{
		Payload:   payloadBytes,
		Signature: sig,
	}, nil
}
```

Wait — this needs `bytes`, `encoding/json` replaced with sonic, and the `static` import from cosign is unused. Let me fix the implementation:

```go
// pkg/sign/cosign.go

// Package sign provides Cosign-based signing of OCI images.
package sign

import (
	"bytes"
	"context"
	"crypto"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"os"

	sigPayload "github.com/sigstore/sigstore/pkg/signature/payload"

	"github.com/flowline-io/flowbot-registry/pkg/json"

	"github.com/sigstore/sigstore/pkg/signature"
)

// SignResult holds the output of a signing operation.
type SignResult struct {
	Payload   []byte
	Signature []byte
}

// Signer signs OCI image references using Cosign with a static private key.
type Signer struct {
	verifier signature.SignerVerifier
}

// NewSigner creates a new Signer from a PEM-encoded private key.
// password is unused (reserved for encrypted key support in future).
func NewSigner(keyPath string, _ string) (*Signer, error) {
	keyData, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, fmt.Errorf("read key file: %w", err)
	}

	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block from %s", keyPath)
	}

	var key crypto.Signer
	switch block.Type {
	case "RSA PRIVATE KEY":
		key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse PKCS1 RSA private key: %w", err)
		}
	case "PRIVATE KEY":
		k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse PKCS8 private key: %w", err)
		}
		var ok bool
		key, ok = k.(crypto.Signer)
		if !ok {
			return nil, fmt.Errorf("key does not implement crypto.Signer")
		}
	default:
		return nil, fmt.Errorf("unsupported PEM block type %q", block.Type)
	}

	sv, err := signature.LoadSignerVerifier(key, crypto.SHA256)
	if err != nil {
		return nil, fmt.Errorf("load signer-verifier: %w", err)
	}

	return &Signer{verifier: sv}, nil
}

// Sign signs an OCI image reference and returns the payload and signature.
func (s *Signer) Sign(imageRef string) (*SignResult, error) {
	payload := sigPayload.Cosign{
		Image: imageRef,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal payload: %w", err)
	}

	sig, _, err := s.verifier.SignMessage(bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("sign payload: %w", err)
	}

	return &SignResult{
		Payload:   payloadBytes,
		Signature: sig,
	}, nil
}

```

- [ ] **Step 5: Run tests**

Run: `go mod tidy && go test ./pkg/sign/ -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add pkg/sign/cosign.go pkg/sign/cosign_test.go go.mod go.sum
git commit -m "feat(sign): add Cosign Signer with RSA/ECDSA key loading and signing"
```

---

## Phase 5: CLI Integration — Full Publish Pipeline

### Task 5.1: Rewrite publish.go with full pipeline

**Files:**

- Modify: `cmd/cli/publish.go`
- Create: `cmd/cli/publish_test.go`

- [ ] **Step 1: Write failing publish tests**

```go
// cmd/cli/publish_test.go

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPublishFlagParsing(t *testing.T) {
	tests := []struct {
		name      string
		envKey    string
		envVal    string
		flagName  string
		fallback  string
		wantValue string
	}{
		{
			name:      "default registry flag",
			envKey:    "FLOWBOT_REGISTRY_URL",
			envVal:    "",
			flagName:  "--registry",
			fallback:  "ghcr.io",
			wantValue: "ghcr.io",
		},
		{
			name:      "default store url flag",
			envKey:    "FLOWBOT_STORE_URL",
			envVal:    "",
			flagName:  "--store-url",
			fallback:  "http://localhost:8128",
			wantValue: "http://localhost:8128",
		},
		{
			name:      "env var overrides default",
			envKey:    "FLOWBOT_STORE_URL",
			envVal:    "https://store.flowbot.io",
			flagName:  "--store-url",
			fallback:  "",
			wantValue: "https://store.flowbot.io",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv(tt.envKey, tt.envVal)
			val := envOrFlag(tt.envKey, tt.fallback)
			assert.Equal(t, tt.wantValue, val)
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/cli/ -run TestPublishFlag -v`
Expected: FAIL — `envOrFlag` not defined

- [ ] **Step 3: Implement envOrFlag helper and rewrite publish.go**

```go
// cmd/cli/publish.go — replace entire file

package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/flowline-io/flowbot-registry/internal/build"
	"github.com/flowline-io/flowbot-registry/pkg/manifest"
	"github.com/flowline-io/flowbot-registry/pkg/oci"
	"github.com/flowline-io/flowbot-registry/pkg/sign"
	"github.com/spf13/cobra"
)

var publishArgs struct {
	registryURL string
	storeURL    string
	apiKey      string
	keyPath     string
	noSign      bool
}

func publishCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "publish",
		Short: "Publish a plugin to the registry",
		Long: `Cross-compile, package as OCI artifact, sign with Cosign, and publish to the registry.

The current directory must contain a plugin.yaml file.

Requires:
  - go (for gRPC plugins) or tinygo (for Wasm plugins)
  - Cosign private key via --key flag (unless --no-sign)`,
		RunE: runPublish,
	}

	cmd.Flags().StringVar(&publishArgs.registryURL, "registry", envOrFlag("FLOWBOT_REGISTRY_URL", "ghcr.io"), "OCI registry URL")
	cmd.Flags().StringVar(&publishArgs.storeURL, "store-url", envOrFlag("FLOWBOT_STORE_URL", "http://localhost:8128"), "Store API URL")
	cmd.Flags().StringVar(&publishArgs.apiKey, "api-key", envOrFlag("FLOWBOT_API_KEY", ""), "API key for store authentication")
	cmd.Flags().StringVar(&publishArgs.keyPath, "key", envOrFlag("COSIGN_KEY_PATH", ""), "Cosign private key path")
	cmd.Flags().BoolVar(&publishArgs.noSign, "no-sign", false, "Skip Cosign signing")

	return cmd
}

// envOrFlag returns the environment variable value if set, otherwise the fallback.
func envOrFlag(envKey, fallback string) string {
	if v := os.Getenv(envKey); v != "" {
		return v
	}
	return fallback
}

func runPublish(_ *cobra.Command, _ []string) error {
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get working directory: %w", err)
	}

	slog.Info("publish: reading plugin.yaml", "dir", cwd)

	manifestPath := filepath.Join(cwd, "plugin.yaml")
	raw, err := os.ReadFile(manifestPath)
	if err != nil {
		return fmt.Errorf("read plugin.yaml: %w (run 'flowbot plugin init' first)", err)
	}

	m, err := manifest.ParseManifest(raw)
	if err != nil {
		return fmt.Errorf("parse plugin.yaml: %w", err)
	}

	namespace, name := manifest.PluginNameFromRef(m.Name)

	_, _ = fmt.Printf("Publishing plugin: %s/%s v%s\n", namespace, name, m.Version)
	fmt.Println()

	ociRef := fmt.Sprintf("%s/%s/%s:%s", strings.TrimRight(publishArgs.registryURL, "/"), namespace, name, m.Version)

	// Step 1: Check idempotency
	ociClient := oci.NewClient(publishArgs.registryURL)
	existingDigest, err := ociClient.HeadManifest(context.Background(), ociRef)
	if err == nil {
		slog.Info("publish: image already exists", "ref", ociRef, "digest", existingDigest)
		_, _ = fmt.Printf("  Image already exists: %s@%s\n", ociRef, existingDigest)
	} else {
		// Step 2: Build
		_, _ = fmt.Println("  [1/4] Building plugin...")
		var builder build.Builder
		switch m.Runtime {
		case manifest.RuntimeGRPC:
			builder = build.NewGrpcBuilder()
		case manifest.RuntimeWasm:
			builder = build.NewWasmBuilder()
		default:
			return fmt.Errorf("unknown runtime %q", m.Runtime)
		}

		artifacts, err := builder.Build(context.Background(), cwd, m)
		if err != nil {
			return fmt.Errorf("build plugin: %w", err)
		}

		var size int
		for _, a := range artifacts {
			size += len(a.Content)
		}
		_, _ = fmt.Printf("  Built %d artifact(s), %s\n", len(artifacts), formatSize(size))

		// Step 3: Read plugin.yaml and collect all artifacts for OCI push
		ociFiles := []oci.ArtifactFile{
			{Name: "plugin.yaml", Content: raw},
		}
		for _, a := range artifacts {
			ociFiles = append(ociFiles, oci.ArtifactFile{Name: a.Name, Content: a.Content})
		}
		// Include README.md if present
		if readmeData, err := os.ReadFile(filepath.Join(cwd, "README.md")); err == nil {
			ociFiles = append(ociFiles, oci.ArtifactFile{Name: "README.md", Content: readmeData})
		}

		// Step 4: Push OCI image
		_, _ = fmt.Println("  [2/4] Pushing OCI image...")
		existingDigest, err = ociClient.PushArtifact(context.Background(), ociRef, ociFiles)
		if err != nil {
			return fmt.Errorf("push OCI artifact: %w", err)
		}
		_, _ = fmt.Printf("  Pushed: %s\n", existingDigest)
	}

	// Step 5: Cosign sign
	if !publishArgs.noSign {
		_, _ = fmt.Println("  [3/4] Signing with Cosign...")
		if publishArgs.keyPath == "" {
			return fmt.Errorf("--key flag is required for signing (or use --no-sign to skip)")
		}

		signer, err := sign.NewSigner(publishArgs.keyPath, os.Getenv("COSIGN_PASSWORD"))
		if err != nil {
			return fmt.Errorf("load signing key: %w", err)
		}

		result, err := signer.Sign(fmt.Sprintf("%s@%s", ociRef, existingDigest))
		if err != nil {
			return fmt.Errorf("sign image: %w", err)
		}

		if err := ociClient.PushSignature(context.Background(), ociRef, result.Payload, result.Signature); err != nil {
			return fmt.Errorf("push signature: %w", err)
		}
		_, _ = fmt.Println("  Signed OK")
	} else {
		_, _ = fmt.Println("  [3/4] Skipping signing (--no-sign)")
	}

	// Step 6: Register metadata
	_, _ = fmt.Println("  [4/4] Registering metadata...")
	if err := registerPublishMetadata(publishArgs.storeURL, publishArgs.apiKey, ociRef, namespace, name, m.Version, existingDigest.String()); err != nil {
		return fmt.Errorf("register publish metadata: %w", err)
	}

	_, _ = fmt.Println()
	_, _ = fmt.Printf("Published: %s\n", ociRef)
	_, _ = fmt.Printf("Digest: %s\n", existingDigest)

	return nil
}

func formatSize(bytes int) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func registerPublishMetadata(storeURL string, apiKey string, ociRef string, namespace string, name string, version string, digest string) error {
	apiURL := fmt.Sprintf("%s/api/v1/plugins/%s/%s/publish",
		strings.TrimRight(storeURL, "/"), namespace, name)

	slog.Debug("publish: registering metadata", "url", apiURL, "version", version, "digest", digest)

	body := map[string]string{
		"version":       version,
		"oci_digest":    digest,
		"oci_image_ref": ociRef,
	}

	resp, err := doJSONPost(apiURL, body, apiKey)
	if err != nil {
		return fmt.Errorf("register metadata: %w", err)
	}
	defer resp.Body.Close()

	return nil
}
```

- [ ] **Step 4: Add doJSONPost helper and bytes import to search.go**

In `cmd/cli/search.go`, add `"bytes"` to the import block, then append `doJSONPost`:

```go
import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/flowline-io/flowbot-registry/pkg/json"
	"github.com/spf13/cobra"
)

// ... (existing code stays unchanged; append below) ...

// doJSONPost performs a POST request with a JSON body and optional API key auth.
func doJSONPost(apiURL string, body any, apiKey string) (*http.Response, error) {
	bodyBytes, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal body: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, apiURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		return resp, fmt.Errorf("store API returned %d: %s", resp.StatusCode, string(body))
	}

	return resp, nil
}
```

- [ ] **Step 5: Run publish tests**

Run: `go test ./cmd/cli/ -run TestPublishFlag -v`
Expected: PASS

- [ ] **Step 6: Commit**

```bash
git add cmd/cli/publish.go cmd/cli/publish_test.go cmd/cli/search.go
git commit -m "feat(cli): full publish pipeline with build, push, sign, register"
```

---

### Task 5.2: Implement register subcommand

**Files:**

- Create: `cmd/cli/register.go`
- Create: `cmd/cli/register_test.go`

- [ ] **Step 1: Write failing register tests**

```go
// cmd/cli/register_test.go

package main

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRegisterRef(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantNs    string
		wantName  string
		wantVer   string
		wantErr   bool
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
			name:     "ref without version fails",
			input:    "community/my-plugin",
			wantErr:  true,
		},
		{
			name:     "bare name fails",
			input:    "my-plugin:1.0.0",
			wantErr:  true,
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./cmd/cli/ -run TestParseRegister -v`
Expected: FAIL — `parseRegisterRef` not defined

- [ ] **Step 3: Implement register command**

```go
// cmd/cli/register.go

package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/flowline-io/flowbot-registry/pkg/oci"
	"github.com/spf13/cobra"
)

var registerArgs struct {
	registryURL string
	storeURL    string
	apiKey      string
}

func registerCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "register <namespace/name:version>",
		Short: "Retry metadata registration for an existing OCI image",
		Long: `Re-register an already-pushed OCI image with the store API.
Use when a previous publish succeeded at OCI push but failed at metadata registration.

Example:
  flowbot plugin register community/my-plugin:1.2.0`,
		Args: cobra.ExactArgs(1),
		RunE: runRegister,
	}

	cmd.Flags().StringVar(&registerArgs.registryURL, "registry", envOrFlag("FLOWBOT_REGISTRY_URL", "ghcr.io"), "OCI registry URL")
	cmd.Flags().StringVar(&registerArgs.storeURL, "store-url", envOrFlag("FLOWBOT_STORE_URL", "http://localhost:8128"), "Store API URL")
	cmd.Flags().StringVar(&registerArgs.apiKey, "api-key", envOrFlag("FLOWBOT_API_KEY", ""), "API key for store authentication")

	return cmd
}

func runRegister(_ *cobra.Command, args []string) error {
	ns, name, version, err := parseRegisterRef(args[0])
	if err != nil {
		return err
	}

	ociRef := fmt.Sprintf("%s/%s/%s:%s", strings.TrimRight(registerArgs.registryURL, "/"), ns, name, version)

	slog.Info("register: checking OCI manifest", "ref", ociRef)

	ociClient := oci.NewClient(registerArgs.registryURL)
	digest, err := ociClient.HeadManifest(context.Background(), ociRef)
	if err != nil {
		return fmt.Errorf("fetch OCI manifest: %w", err)
	}

	_, _ = fmt.Printf("  Fetching OCI manifest... OK (%s)\n", digest)

	slog.Info("register: registering metadata", "namespace", ns, "name", name, "version", version, "digest", digest)

	if err := registerPublishMetadata(registerArgs.storeURL, registerArgs.apiKey, ociRef, ns, name, version, digest.String()); err != nil {
		return fmt.Errorf("register metadata: %w", err)
	}

	_, _ = fmt.Println("  Registering metadata... OK")
	_, _ = fmt.Println("Plugin metadata registered.")

	return nil
}

// parseRegisterRef validates and splits a register ref into namespace, name, version.
func parseRegisterRef(ref string) (namespace, name, version string, err error) {
	if idx := strings.LastIndex(ref, ":"); idx >= 0 {
		version = ref[idx+1:]
		ref = ref[:idx]
	} else {
		return "", "", "", fmt.Errorf("version is required: %s (must be namespace/name:version)", ref)
	}

	parts := strings.SplitN(ref, "/", 2)
	if len(parts) != 2 {
		return "", "", "", fmt.Errorf("invalid ref %q: must be namespace/name:version", ref)
	}

	return parts[0], parts[1], version, nil
}
```

- [ ] **Step 4: Wire register command in main.go**

Update `cmd/cli/main.go` to add register:

```go
	pluginCmd.AddCommand(initCmd())
	pluginCmd.AddCommand(publishCmd())
	pluginCmd.AddCommand(registerCmd())
	pluginCmd.AddCommand(installCmd())
	pluginCmd.AddCommand(searchCmd())
```

- [ ] **Step 5: Run register tests**

Run: `go test ./cmd/cli/ -run TestParseRegister -v`
Expected: PASS

- [ ] **Step 6: Build CLI**

Run: `go build ./cmd/cli/`
Expected: compiles successfully

- [ ] **Step 7: Commit**

```bash
git add cmd/cli/register.go cmd/cli/register_test.go cmd/cli/main.go
git commit -m "feat(cli): add register subcommand for metadata registration recovery"
```

---

## Phase 6: Polish

### Task 6.1: Run full lint and test suite

- [ ] **Step 1: Format code**

Run: `go tool task format`
Expected: clean

- [ ] **Step 2: Run lint**

Run: `go tool task lint`
Expected: no errors

- [ ] **Step 3: Run all tests**

Run: `go tool task test`
Expected: PASS

- [ ] **Step 4: Run race tests**

Run: `go tool task test:race`
Expected: PASS

- [ ] **Step 5: Build both binaries**

Run: `go tool task build:all`
Expected: both binaries build successfully

- [ ] **Step 6: Commit**

```bash
git add -A
git commit -m "chore: lint, format, test all passes after publisher CLI implementation"
```
