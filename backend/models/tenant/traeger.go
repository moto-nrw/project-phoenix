package tenant

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/uptrace/bun"
)

// Traeger represents a carrier organization (top-level tenant)
// A Träger operates one or more OGS facilities, optionally through Büros
type Traeger struct {
	ID           string          `bun:"id,pk" json:"id"`
	Name         string          `bun:"name,notnull" json:"name"`
	ContactEmail *string         `bun:"contact_email" json:"contact_email,omitempty"`
	BillingInfo  json.RawMessage `bun:"billing_info,type:jsonb" json:"billing_info,omitempty"`
	CreatedAt    time.Time       `bun:"created_at,notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt    time.Time       `bun:"updated_at,notnull,default:current_timestamp" json:"updated_at"`

	// Relations (loaded via JOIN when needed)
	Bueros []*Buero `bun:"rel:has-many,join:id=traeger_id" json:"bueros,omitempty"`
}

// TableName returns the database table name
func (t *Traeger) TableName() string {
	return "tenant.traeger"
}

// BeforeAppendModel handles schema qualification for BUN queries
func (t *Traeger) BeforeAppendModel(query any) error {
	if q, ok := query.(*bun.SelectQuery); ok {
		q.ModelTableExpr(`tenant.traeger AS "traeger"`)
	}
	if q, ok := query.(*bun.InsertQuery); ok {
		q.ModelTableExpr(`tenant.traeger`)
	}
	if q, ok := query.(*bun.UpdateQuery); ok {
		q.ModelTableExpr(`tenant.traeger AS "traeger"`)
	}
	if q, ok := query.(*bun.DeleteQuery); ok {
		q.ModelTableExpr(`tenant.traeger AS "traeger"`)
	}
	return nil
}

// Validate ensures traeger data is valid
func (t *Traeger) Validate() error {
	if strings.TrimSpace(t.Name) == "" {
		return errors.New("träger name is required")
	}
	t.Name = strings.TrimSpace(t.Name)
	return nil
}

// GetID returns the entity's ID
func (t *Traeger) GetID() interface{} {
	return t.ID
}

// GetCreatedAt returns the creation timestamp
func (t *Traeger) GetCreatedAt() time.Time {
	return t.CreatedAt
}

// GetUpdatedAt returns the last update timestamp
func (t *Traeger) GetUpdatedAt() time.Time {
	return t.UpdatedAt
}
