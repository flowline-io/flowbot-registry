package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type Plugin struct {
	ent.Schema
}

func (Plugin) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").Immutable(),
		field.Int("namespace_id").Immutable(),
		field.String("name").NotEmpty(),
		field.String("display_name").Default(""),
		field.String("description").Default(""),
		field.String("logo_url").Default(""),
	}
}

func (Plugin) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("versions", PluginVersion.Type),
	}
}

func (Plugin) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("namespace_id", "name").Unique(),
	}
}
