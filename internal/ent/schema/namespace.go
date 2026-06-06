// Package schema defines the ent ORM database schema models.
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

type Namespace struct {
	ent.Schema
}

func (Namespace) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").Immutable(),
		field.String("name").NotEmpty().Unique(),
		field.String("type").NotEmpty().Comment("user or org"),
		field.Int("user_id").Optional().Nillable(),
	}
}

func (Namespace) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("plugins", Plugin.Type),
		edge.From("user", User.Type).
			Ref("namespaces").
			Field("user_id").
			Unique(),
	}
}

func (Namespace) Indexes() []ent.Index {
	return nil
}
