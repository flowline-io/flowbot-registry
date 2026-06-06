package schema

import (
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/field"
)

// RefreshToken stores JWT refresh tokens for user sessions.
type RefreshToken struct {
	ent.Schema
}

func (RefreshToken) Fields() []ent.Field {
	return []ent.Field{
		field.Int("id").Immutable(),
		field.String("token").NotEmpty().Unique(),
		field.Time("expires_at"),
		field.Time("created_at").Default(time.Now).Immutable(),
	}
}

func (RefreshToken) Edges() []ent.Edge {
	return nil
}

func (RefreshToken) Indexes() []ent.Index {
	return nil
}
