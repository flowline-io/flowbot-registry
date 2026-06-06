# Store Layer

Data access operations backed by the ent ORM client. All database queries go through this layer.

## Structure

```
store/
├── store.go   # Adapter, TxAdapter, CRUD methods, record types
└── errors.go  # Sentinel errors (ErrNotFound, ErrConflict)
```

## Adapter

Wraps `ent.Client` for non-transactional queries.

### TxAdapter

Wraps `ent.Tx` for transactional queries. Created via `Adapter.BeginTx(ctx)`. Must call `Commit()` or `Rollback()`.

```go
tx, err := adapter.BeginTx(ctx)
defer tx.Rollback()
// ... CRUD operations on tx ...
return tx.Commit()
```

## Record Types

- `NamespaceRecord` — id, name, type
- `PluginRecord` — id, namespace_id, name, display_name, description, logo_url
- `PluginVersionRecord` — id, plugin_id, version, oci_image_ref, oci_digest, readme_html, manifest_json

## Sentinel Errors

- `ErrNotFound` — record not found
- `ErrConflict` — unique constraint violation

## Rules

- All database queries go through Adapter or TxAdapter. Never use ent.Client directly in other packages.
- Never write SQL outside this layer.
- `PluginVersionCreate` and `PluginVersionUpdate` accept `manifest_json` as `map[string]any` (ent JSON field type).
