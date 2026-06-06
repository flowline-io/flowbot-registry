package manifest

import (
	"fmt"
	"strings"
)

// InitFile represents a file to be created during plugin scaffold.
type InitFile struct {
	Path    string
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

	files = append(files, InitFile{Path: "main.go", Content: generateServerMainGo()})

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

go 1.26.4

%s
`

var grpcRequire = `require github.com/flowline-io/flowbot v0.92.1`

// GenerateGoMod returns a go.mod scaffold for the plugin.
func GenerateGoMod(namespace, name string, runtime RuntimeKind) []byte {
	var req string
	if runtime == RuntimeGRPC {
		req = grpcRequire
	}
	return []byte(fmt.Sprintf(goModTemplate, namespace, name, req))
}

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
		return generateServerMainGo()
	case RuntimeWasm:
		return []byte(wasmMainGo)
	default:
		return nil
	}
}

var serverMainGo = `package main

import "github.com/flowline-io/flowbot/pkg/plugin/sdk"

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
func PluginNameFromRef(fullName string) (namespace, name string) {
	parts := strings.SplitN(fullName, "/", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	return "default", fullName
}
