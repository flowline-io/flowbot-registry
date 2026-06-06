// Package schema defines the ent ORM database schema models.
package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
)

// RefreshToken stores hashed refresh tokens for session management.
type RefreshToken struct {
	ent.Schema
}

func (RefreshToken) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").Immutable(),
		field.Int("user_id"),
		field.String("token_hash").NotEmpty().Unique().Sensitive(),
		field.Time("expires_at"),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (RefreshToken) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("refresh_tokens").
			Field("user_id").
			Unique().
			Required(),
	}
}

func (RefreshToken) Indexes() []ent.Index {
	return nil
}
