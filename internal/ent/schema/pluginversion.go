package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type PluginVersion struct {
	ent.Schema
}

func (PluginVersion) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").Immutable(),
		field.Int("plugin_id"),
		field.String("version").NotEmpty().Comment("semver"),
		field.String("oci_image_ref").Default("").Comment("OCI repository reference"),
		field.String("oci_digest").NotEmpty().Comment("SHA256 digest"),
		field.String("readme_html").Default(""),
		field.JSON("manifest_json", map[string]any{}).Default(map[string]any{}),
	}
}

func (PluginVersion) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("plugin", Plugin.Type).
			Ref("versions").
			Field("plugin_id").
			Unique().
			Required(),
	}
}

func (PluginVersion) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("plugin_id", "version").Unique(),
	}
}
