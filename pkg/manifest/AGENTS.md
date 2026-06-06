# Manifest

Parses `plugin.yaml` files into structured data for validation and publishing.

## Manifest

```go
type Manifest struct {
    Name        string `yaml:"name" json:"name"`
    Version     string `yaml:"version" json:"version"`
    Description string `yaml:"description" json:"description"`
    Author      string `yaml:"author" json:"author"`
}
```

## Functions

- `ParseManifest(data []byte) (*Manifest, error)` — parses YAML bytes, validates required fields (name, version)

## Dependencies

Uses `github.com/goccy/go-yaml` for YAML parsing. Internal `unmarshalYAML` wrapper for consistent error formatting.
