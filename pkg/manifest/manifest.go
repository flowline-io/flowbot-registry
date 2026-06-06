// Package manifest parses plugin.yaml manifest files into structured data.
package manifest

// Manifest represents the plugin.yaml definition for a plugin.
type Manifest struct {
	Name        string `yaml:"name" json:"name"`
	Version     string `yaml:"version" json:"version"`
	Description string `yaml:"description" json:"description"`
	Author      string `yaml:"author" json:"author"`
}

// ParseManifest parses raw YAML bytes into a Manifest.
func ParseManifest(data []byte) (*Manifest, error) {
	var m Manifest
	err := unmarshalYAML(data, &m)
	if err != nil {
		return nil, err
	}
	if m.Name == "" {
		return nil, errMissingField("name")
	}
	if m.Version == "" {
		return nil, errMissingField("version")
	}
	return &m, nil
}
