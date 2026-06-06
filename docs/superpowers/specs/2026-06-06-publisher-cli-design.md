# Publisher CLI Design

## Overview

Developer publishing flow for the Flowbot plugin ecosystem. Provides a friendly CLI (`flowbot plugin`) that shields developers from cross-compilation, OCI packaging, Cosign signing, and metadata registration details.

## CLI Command Structure

```
flowbot
  plugin
    init <namespace/name>    # Scaffold a plugin project
    publish                  # Cross-compile, OCI push, sign, register
    register <namespace/name:version>  # Retry metadata registration for an existing OCI ref
    install <ref>            # Pull and install a plugin from registry
    search [query]           # Search the plugin registry
```

### `flowbot plugin init <namespace/name>`

Interactive plugin project scaffold. The argument is a fully-qualified plugin reference in `<namespace>/<name>` format (e.g., `community/github-stars`). Prompts for runtime type (grpc/wasm), then generates:

- `plugin.yaml` — manifest aligned with flowbot's Manifest schema (name includes namespace)
- `main.go` — entry point skeleton (or `cmd/server/main.go` for gRPC)
- `go.mod` — module declaration

Flags: None beyond the positional `<namespace/name>` argument.

### `flowbot plugin publish`

Full automated pipeline in the current directory:

1. Read and validate `plugin.yaml`
2. Cross-compile based on runtime type
3. Check if OCI digest already exists → skip push if so (idempotent)
4. Build OCI Image (gRPC) or single arch manifest (Wasm)
5. Cosign sign the image digest
6. Push to OCI registry
7. Register metadata with store API (Exit 1 on failure; re-run `publish` is safe since step 3 skips existing push)

Flags:

- `--key` — Cosign private key path (required, unless `--no-sign`)
- `--registry` — OCI registry URL (default: `ghcr.io`)
- `--store-url` — Store API URL (default: `http://localhost:8128`)
- `--api-key` — API key for store authentication
- `--no-sign` — skip Cosign signing step

### `flowbot plugin register <namespace/name:version>`

Recovery command. When a previous `publish` failed on metadata registration, re-register an already-pushed OCI image:

```
$ flowbot plugin register community/my-plugin:1.2.0
  Fetching OCI manifest... OK (sha256:abc123)
  Registering metadata... OK
  Plugin metadata registered.
```

Flags:

- `--registry` — OCI registry URL
- `--store-url` — Store API URL
- `--api-key` — API key for store authentication

## Package Architecture

```
cmd/cli/
  main.go           # root command + plugin parent subcommand
  init.go           # plugin init scaffold logic
  publish.go        # plugin publish pipeline orchestration
  install.go        # plugin install (moved from flat root)
  search.go         # plugin search (moved from flat root)

pkg/manifest/
  manifest.go       # Manifest struct aligned with flowbot schema
  yaml.go           # go-yaml unmarshal wrapper
  template.go       # Generate plugin.yaml + skeleton files per runtime

internal/build/
  builder.go        # Builder interface + shared helpers
  grpc.go           # GrpcBuilder: go build GOOS=linux GOARCH=amd64
  wasm.go           # WasmBuilder: tinygo build -target=wasi

pkg/oci/
  client.go         # FetchManifest, FetchManifestByDigest (existing) + PushArtifact
  pusher.go         # PushArtifact: blob upload, manifest push
  signature.go      # PushSignature: push Cosign referrer artifact

pkg/sign/
  cosign.go         # Signer using sigstore/cosign/v2
```

## Data Flow: publish Pipeline

```
plugin.yaml ──► Validate Manifest
       │
       ▼
Check existing digest (idempotency):
  remote.Head(ref) ──► if digest matches, skip build+push, jump to register
       │
       ▼
Detect runtime ──┬── grpc: GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o plugin-server ./cmd/server
                 └── wasm: tinygo build -target=wasi -o plugin.wasm .
       │
       ▼
Collect artifacts: [plugin.yaml, binary/wasm, README.md?]
       │
       ▼
Build OCI image:
  1. NewLayer(plugin.yaml)        ── tar.gz ──► blob push
  2. NewLayer(binary/wasm)        ── tar.gz ──► blob push
  3. NewLayer(README.md?)         ── tar.gz ──► blob push
  4. mut.ConfigMediaType = "application/vnd.flowbot.plugin.config.v1+yaml"
  5. remote.Write(manifest)
       │
       ▼
Cosign sign (unless --no-sign):
  1. cosign.Sign(indexRef, cosign.WithKey(keyPath))
  2. signature pushed as referrer artifact
       │
       ▼
Register metadata:
  POST {storeURL}/api/v1/plugins/{ns}/{name}/publish
  {version, oci_image_ref, oci_digest, signature_digest?}
       │
       ▼
Done. Output: ghcr.io/ns/name:version
```

## Error Handling

| Stage             | Failure                                           | Behavior                                                                                                             |
| ----------------- | ------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------- |
| Validate manifest | Missing file, invalid YAML, required field absent | Exit 1, show error + fix hint                                                                                        |
| Idempotency check | Network error on HEAD request                     | Warn, proceed with full build+push                                                                                   |
| Build             | Compilation error                                 | Exit 1, show stderr from go/tinygo build                                                                             |
| OCI Push          | 401 Unauthorized                                  | go-containerregistry handles auth via `remote.WithAuth(registryToken)` — no manual 401 handling needed               |
| OCI Push          | Network timeout                                   | Retry up to 3x with exponential backoff                                                                              |
| Cosign            | Key file missing or invalid                       | Exit 1, show path                                                                                                    |
| Cosign            | Signing failed                                    | Exit 1, show error                                                                                                   |
| Register metadata | Any failure (API unreachable, conflict, invalid)  | **Exit 1**. Image is still live; use `flowbot plugin register` to retry, or re-run `publish` (idempotent: skip push) |

## Idempotency

`publish` is safe to re-run after partial failures:

1. Before building, CLI does `HEAD <registry>/<ns>/<name>:<version>` to check if a manifest exists with the expected digest.
2. If the same OCI image already exists (same plugin.yaml + binary → same digest), the build+push step is skipped entirely. CLI proceeds directly to Cosign (if not already signed) and metadata registration.
3. On metadata registration failure, `exit 1`. Developer fixes network/auth issue and re-runs `publish` — steps 1-2 skip OCI push, step 6 registers metadata.

## Manifest Schema

Aligned with flowbot's `pkg/plugin/manifest.go` Manifest struct:

```go
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

type RuntimeKind string // "grpc" | "wasm"

type Provides struct {
    Module    bool          `yaml:"module" json:"module"`
    Abilities []AbilityDecl `yaml:"abilities" json:"abilities"`
    Provider  *ProviderDecl `yaml:"provider" json:"provider,omitempty"`
}

type GRPCConfig struct {
    Binary string   `yaml:"binary" json:"binary"`
    Args   []string `yaml:"args" json:"args"`
}

type WasmConfig struct {
    Module      string           `yaml:"module" json:"module"`
    Permissions *WasmPermissions `yaml:"permissions" json:"permissions"`
    Pool        *WasmPoolConfig  `yaml:"pool" json:"pool,omitempty"`
}
```

## Init Templates

### gRPC template (plugin.yaml)

```yaml
name: {namespace}/{name}
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
```

### Wasm template (plugin.yaml)

```yaml
name: {namespace}/{name}
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
```

## Build Interface

```go
type Artifact struct {
    Name    string // filename: "plugin.yaml", "plugin-server", "plugin.wasm"
    Content []byte // file contents
}

type Builder interface {
    Build(ctx context.Context, dir string, m *manifest.Manifest) ([]Artifact, error)
}
```

- `GrpcBuilder`: executes `CGO_ENABLED=0 go build` targeting `./cmd/server` for **linux/amd64**
- `WasmBuilder`: executes `tinygo build -target=wasi -no-debug` targeting `.` (main package), single `wasi/wasm` target

## OCI Extension

New methods on `*oci.Client`:

```go
// PushArtifact creates tar.gz OCI layers from files and pushes a single manifest.
// Plugin config layer uses media type "application/vnd.flowbot.plugin.config.v1+yaml".
// Binary layers use "application/vnd.oci.image.layer.v1.tar+gzip".
//
// Auth: uses remote.WithAuth(registryToken) — no manual 401 handling.
// go-containerregistry handles Docker Registry v2 auth challenge/response internally.
func (c *Client) PushArtifact(ctx context.Context, ref string, files []ArtifactFile, opts ...PushArtifactOption) (v1.Hash, error)

// PushSignature pushes a Cosign signature as an OCI referrer artifact.
func (c *Client) PushSignature(ctx context.Context, ref string, sigPayload []byte, signature []byte) error

// HeadManifest checks if a manifest exists at the given reference.
// Returns the digest if found, or ErrNotFound if not.
func (c *Client) HeadManifest(ctx context.Context, ref string) (v1.Hash, error)
```

Depends on `github.com/google/go-containerregistry` for `remote.Write`, `remote.Head`, `static.NewLayer`, `mutate.MediaType`.

### Layer Format

All artifacts are packaged as **tar.gz** layers (standard OCI layer format), consistent with the existing `ExtractLayers` implementation. This allows multiple files per layer (e.g., `plugin.yaml` + `plugin-server` for a given platform, plus optional `README.md` and auxiliary resources), ensuring clean directory structure on extraction.

### OCI Auth Flow

The push flow does **not** manually handle 401 responses. Instead:

1. CLI receives a JWT token from the store API (same mechanism as current `publish`/`install` flow per AGENTS.md OCI Auth Flow).
2. CLI wraps the token as `authn.AuthConfig{RegistryToken: jwt}` and passes it to the OCI client.
3. OCI client calls `remote.Write(ref, remote.WithAuth(auth))` — go-containerregistry automatically handles the full Docker Registry v2 challenge/response protocol.

## Cosign Signing

```go
type Signer struct { ... }

func NewSigner(keyPath string) (*Signer, error)
func (s *Signer) Sign(ctx context.Context, ref string) (*SignResult, error)

type SignResult struct {
    SignatureDigest string
    Payload         []byte
    Signature       []byte
}
```

Depends on `github.com/sigstore/cosign/v2` for `cosign.Sign()` with `cosign.WithKey()`.

## Testing Strategy

Table-driven tests co-located with source, minimum 3 cases per table, happy path first.

| Package           | Tests                                                                                                                                                                                                                                                                                                                                                  |
| ----------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `pkg/manifest/`   | Extended schema: grpc valid, wasm valid, invalid runtime, missing sub-config, empty YAML, namespace/name parsing, init template generation for each runtime                                                                                                                                                                                            |
| `internal/build/` | GrpcBuilder calls go build with correct GOOS/GOARCH/env, WasmBuilder calls tinygo with -target=wasi, build errors propagated, artifact file existence check, missing cmd/server directory error                                                                                                                                                        |
| `pkg/oci/`        | PushArtifact blob+manifest upload, HeadManifest existing/not-found, signature as referrer, auth token injection via remote.WithAuth                                                                                                                                                                                                                    |
| `pkg/sign/`       | Load valid PEM key, invalid PEM, missing key file, sign produces valid payload+signature structure, Cosign password from env                                                                                                                                                                                                                           |
| `cmd/cli/`        | Init enforces namespace/name format, init generates correct files per runtime, init rejects bare name without namespace, publish flag parsing (--key, --registry, --no-sign), publish pipeline with mocked build/oci/sign, register command calls HeadManifest + metadata API, env var overrides (FLOWBOT_STORE_URL, FLOWBOT_API_KEY, COSIGN_PASSWORD) |

## Environment Variables

All CLI flags have corresponding environment variable fallbacks. Env vars avoid exposing secrets in command-line arguments (important for CI/CD):

| Flag              | Environment Variable   | Default                        |
| ----------------- | ---------------------- | ------------------------------ |
| `--registry`      | `FLOWBOT_REGISTRY_URL` | `ghcr.io`                      |
| `--store-url`     | `FLOWBOT_STORE_URL`    | `http://localhost:8128`        |
| `--api-key`       | `FLOWBOT_API_KEY`      | (required)                     |
| `--key`           | `COSIGN_KEY_PATH`      | (required when signing)        |
| (cosign password) | `COSIGN_PASSWORD`      | (optional, for encrypted keys) |

## Future: OIDC Keyless Signing

The `--key` flag uses static private key signing. A future iteration will support **OIDC keyless signing** via Sigstore's Fulcio/Rekor:

```
flowbot plugin publish --keyless
```

This eliminates key management: the CLI obtains a short-lived signing certificate from Fulcio using an OIDC identity (GitHub Actions `id-token` or local OIDC provider). The signature is published to the Rekor transparency log. The `pkg/sign/` package is designed to accommodate this — `Signer` can accept a `WithKey(path)` or `WithKeyless()` option in a future update.

## Implementation Phases

1. **Manifest + Init** — Extend `pkg/manifest/` schema (runtime, provides, grpc, wasm, config_schema), add `template.go` with per-runtime template generation, implement `plugin init` with namespace enforcement (`<ns>/<name>` format)
2. **Build pipeline** — Create `internal/build/`, implement `GrpcBuilder` (linux/amd64) + `WasmBuilder` (wasi target)
3. **OCI push** — Extend `pkg/oci/` with `PushArtifact` (single manifest), `HeadManifest` (idempotency check), `PushSignature`, tar.gz layer construction, `remote.WithAuth` based auth
4. **Cosign signing** — Create `pkg/sign/`, implement `Sign()` with sigstore/cosign/v2, private key loading, `COSIGN_PASSWORD` env var for encrypted keys
5. **CLI integration** — Restructure `cmd/cli/` with `plugin` parent command, wire full `publish` pipeline (idempotency check → build → push → sign → register), implement `register` subcommand, env var fallbacks for all flags, `--no-sign` flag
6. **Polish** — Wire install OCI pull, end-to-end test, lint+format+test full pipeline
