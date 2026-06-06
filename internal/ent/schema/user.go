// Package schema defines the ent ORM database schema models.
package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// User stores registered user credentials and identity.
type User struct {
	ent.Schema
}

func (User) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").Immutable(),
		field.String("email").NotEmpty().Unique(),
		field.String("password_hash").NotEmpty(),
		field.Time("created_at").Default(time.Now).Immutable(),
		field.Time("updated_at").Default(time.Now).UpdateDefault(time.Now),
	}
}

func (User) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("refresh_tokens", RefreshToken.Type),
		edge.To("namespaces", Namespace.Type),
	}
}

func (User) Indexes() []ent.Index {
	return nil
}
