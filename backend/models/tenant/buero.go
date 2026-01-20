package tenant

import (
	"errors"
	"strings"
	"time"

	"github.com/uptrace/bun"
)

// Buero represents an office managing multiple OGS (optional middle layer)
// A Büro belongs to a Träger and manages one or more OGS facilities
type Buero struct {
	ID           string    `bun:"id,pk" json:"id"`
	TraegerID    string    `bun:"traeger_id,notnull" json:"traeger_id"`
	Name         string    `bun:"name,notnull" json:"name"`
	ContactEmail *string   `bun:"contact_email" json:"contact_email,omitempty"`
	CreatedAt    time.Time `bun:"created_at,notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt    time.Time `bun:"updated_at,notnull,default:current_timestamp" json:"updated_at"`

	// Relations
	Traeger *Traeger `bun:"rel:belongs-to,join:traeger_id=id" json:"traeger,omitempty"`
}

// TableName returns the database table name
func (b *Buero) TableName() string {
	return "tenant.buero"
}

// BeforeAppendModel handles schema qualification for BUN queries
func (b *Buero) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.SelectQuery); ok {
		q.ModelTableExpr(`tenant.buero AS "buero"`)
	}
	if q, ok := query.(*bun.InsertQuery); ok {
		q.ModelTableExpr(`tenant.buero`)
	}
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(`tenant.buero AS "buero"`)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(`tenant.buero AS "buero"`)
	}
	return nil
}

// Validate ensures buero data is valid
func (b *Buero) Validate() error {
	if strings.TrimSpace(b.Name) == "" {
		return errors.New("büro name is required")
	}
	if strings.TrimSpace(b.TraegerID) == "" {
		return errors.New("träger ID is required")
	}
	b.Name = strings.TrimSpace(b.Name)
	return nil
}

// GetID returns the entity's ID
func (b *Buero) GetID() interface{} {
	return b.ID
}

// GetCreatedAt returns the creation timestamp
func (b *Buero) GetCreatedAt() time.Time {
	return b.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (b *Buero) GetUpdatedAt() time.Time {
	return b.UpdatedAt
}
