package base

import (
	"time"
)

// Model represents the common fields for all database models
type Model struct {
	ID        int64     `bun:"id,pk,autoincrement" json:"id"`
	CreatedAt time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp" json:"updated_at"`
}

// StringIDModel represents models that use string as primary key (like RFID cards)
type StringIDModel struct {
	ID        string    `bun:"id,pk" json:"id"`
	CreatedAt time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp" json:"updated_at"`
}

// GetID, GetCreatedAt and GetUpdatedAt satisfy the base.Entity interface for
// every embedder of Model. They live here ONCE (backend-conventions.md Rule 3)
// so individual entities never redeclare these trivial accessors; an entity
// that needs different semantics may still shadow them.
func (m *Model) GetID() any              { return m.ID }
func (m *Model) GetCreatedAt() time.Time { return m.CreatedAt }
func (m *Model) GetUpdatedAt() time.Time { return m.UpdatedAt }

// GetID, GetCreatedAt and GetUpdatedAt satisfy the base.Entity interface for
// every embedder of StringIDModel (Rule 3, same as Model above).
func (m *StringIDModel) GetID() any              { return m.ID }
func (m *StringIDModel) GetCreatedAt() time.Time { return m.CreatedAt }
func (m *StringIDModel) GetUpdatedAt() time.Time { return m.UpdatedAt }

// Activatable represents models that can be activated or deactivated
type Activatable struct {
	IsActive bool `bun:"is_active,notnull,default:true" json:"is_active"`
}

// Nameable represents models with a name field
type Nameable struct {
	Name string `bun:"name,notnull" json:"name"`
}

// Deref returns the pointed-to value, or T's zero value when p is nil.
func Deref[T any](p *T) T {
	if p == nil {
		var zero T
		return zero
	}
	return *p
}
