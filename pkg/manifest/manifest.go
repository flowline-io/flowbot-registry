// Package manifest parses plugin.yaml manifest files into structured data.
package manifest

import (
	"encoding/json"
	"fmt"

	"github.com/bytedance/sonic"
)

// Manifest represents the plugin.yaml definition for a plugin.
type Manifest struct {
	Name         string      `yaml:"name" json:"name"`
	Version      string      `yaml:"version" json:"version"`
	Description  string      `yaml:"description" json:"description"`
	Author       string      `yaml:"author" json:"author"`
	Runtime      RuntimeKind `yaml:"runtime" json:"runtime"`
	Provides     Provides    `yaml:"provides" json:"provides"`
	GRPC         *GRPCConfig `yaml:"grpc" json:"grpc,omitempty"`
	Wasm         *WasmConfig `yaml:"wasm" json:"wasm,omitempty"`
	ConfigSchema RawMessage  `yaml:"config_schema" json:"config_schema,omitempty"`
}

// RawMessage wraps json.RawMessage with YAML unmarshaling support for
// arbitrary nested values that goccy/go-yaml cannot decode directly.
type RawMessage json.RawMessage

// UnmarshalYAML implements yaml.InterfaceUnmarshaler to decode any YAML
// value into JSON bytes.
func (r *RawMessage) UnmarshalYAML(fn func(any) error) error {
	var v any
	if err := fn(&v); err != nil {
		return err
	}
	b, err := sonic.Marshal(v)
	if err != nil {
		return fmt.Errorf("config_schema: failed to marshal to JSON: %w", err)
	}
	*r = RawMessage(b)
	return nil
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
