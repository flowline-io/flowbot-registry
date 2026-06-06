// Package json provides sonic-based JSON Marshal/Unmarshal, replacing encoding/json.
package json

import (
	"github.com/bytedance/sonic"
)

// Marshal serializes v to JSON bytes using sonic.
func Marshal(v any) ([]byte, error) {
	return sonic.Marshal(v)
}

// Unmarshal deserializes JSON bytes into v using sonic.
func Unmarshal(data []byte, v any) error {
	return sonic.Unmarshal(data, v)
}

// MarshalString serializes v to a JSON string using sonic.
func MarshalString(v any) (string, error) {
	return sonic.MarshalString(v)
}
