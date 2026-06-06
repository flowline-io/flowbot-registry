# JSON Utilities

Thin wrapper around `github.com/bytedance/sonic` providing Marshal/Unmarshal as a drop-in replacement for `encoding/json`.

## Rules

- Always use this package instead of `encoding/json` for JSON operations.
- `encoding/json` is banned per project AGENTS.md anti-patterns.
- `json.RawMessage` from stdlib is still allowed for type definitions only.

## Functions

- `Marshal(v any) ([]byte, error)` — serializes to JSON bytes
- `Unmarshal(data []byte, v any) error` — deserializes from JSON bytes
- `MarshalString(v any) (string, error)` — serializes to JSON string
