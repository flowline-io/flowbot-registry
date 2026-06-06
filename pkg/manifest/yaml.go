package manifest

import (
	"errors"
	"fmt"

	"github.com/goccy/go-yaml"
)

func unmarshalYAML(data []byte, v any) error {
	if err := yaml.Unmarshal(data, v); err != nil {
		return fmt.Errorf("failed to parse YAML: %w", err)
	}
	return nil
}

var errMissingField = func(field string) error {
	return fmt.Errorf("%w: %s", errors.New("missing required field"), field)
}
