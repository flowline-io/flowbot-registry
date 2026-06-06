# Ent Schemas

Database schema definitions using ent ORM. These files define the database tables, fields, edges, and indexes.

## Structure

```
ent/
├── schema/              # Schema definitions (source of truth)
│   ├── namespace.go     # Namespace table
│   ├── plugin.go        # Plugin table
│   └── pluginversion.go # PluginVersion table
├── generate.go          # go:generate directive
├── client.go            # Ent client (generated)
├── ent.go               # Ent types (generated)
├── mutation.go          # Mutation builders (generated)
├── tx.go                # Transaction support (generated)
├── runtime.go           # Runtime config (generated)
├── migrate/             # Migration code (generated)
├── hook/                # Lifecycle hooks (generated)
├── predicate/           # Predicate types (generated)
├── namespace/           # Namespace field constants/predicates (generated)
├── plugin/              # Plugin field constants/predicates (generated)
├── pluginversion/       # PluginVersion field constants/predicates (generated)
└── enttest/             # Test helpers (generated)
```

## Schemas

| Schema        | Fields                                                                        | Unique Index        |
| ------------- | ----------------------------------------------------------------------------- | ------------------- |
| Namespace     | id, name, type                                                                | name                |
| Plugin        | id, namespace_id, name, display_name, description, logo_url                   | namespace_id + name |
| PluginVersion | id, plugin_id, version, oci_image_ref, oci_digest, readme_html, manifest_json | plugin_id + version |

Edges:

- Namespace -> Plugin (one-to-many, "plugins")
- Plugin -> PluginVersion (one-to-many, "versions")

## Rules

- Never edit files under `internal/ent/` except `schema/` and `generate.go`. All other files are auto-generated.
- Schema changes require re-running `go tool task ent` (runs `go generate ./internal/ent/`).
- Use ent auto-migration via `client.Schema.Create()` on startup. No manual SQL.

## Commands

```bash
go tool task ent   # Generate ent code from schemas
```
