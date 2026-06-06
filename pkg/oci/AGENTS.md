# OCI Integration

Interacts with OCI-compliant container registries using `github.com/google/go-containerregistry`.

## Components

### Client

Wraps go-containerregistry for manifest operations.

- `FetchManifest(ctx, ref) — pulls an OCI image by reference
- `FetchManifestByDigest(ctx, imageRef, digest) — pulls by digest reference

### ExtractLayers

Extracts named files from an OCI image's tar-gz layers.

- `ExtractLayers(img, fileNames) — returns `[]LayerFile` with byte content for each requested file

### LayerFile

Represents a file extracted from an OCI layer:

```go
type LayerFile struct {
    Name    string
    Content []byte
}
```

## Usage

```go
client := oci.NewClient("http://localhost:5000")
img, _ := client.FetchManifestByDigest(ctx, "registry/ns/plugin", "sha256:abc...")
layers, _ := oci.ExtractLayers(img, []string{"plugin.yaml", "README.md"})
```
