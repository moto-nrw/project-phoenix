// Package display holds the ORM compatibility row for display.displays:
// tenant-scoped info-point screens (TVs, smartboards) that render a read-only
// dashboard authenticated by an opaque URL token (issue #1325). Runtime reads
// and writes go through modules/devicefleet.
package display

import (
	"context"
	"errors"
	"time"
)

// Sentinel errors shared between the retained repository shape, the service,
// and the handler layer.
var (
	// ErrNotFound: no display matches the given ID or token.
	ErrNotFound = errors.New("display not found")
	// ErrInactive: the display exists but has been deactivated.
	ErrInactive = errors.New("display inactive")
	// ErrInvalidInput: the request payload failed validation (e.g. name rules).
	ErrInvalidInput = errors.New("invalid display input")
)

// Display is a registered info-point screen. The raw access token is never
// stored — only its SHA-256 hash. A display whose token leaks is revoked by
// regenerating the token or deleting the row.
type Display struct {
	//nolint:unused // BUN consumes this table metadata through reflection.
	tableName struct{}  `bun:"table:display.displays,alias:display"`
	ID        int64     `bun:"id,pk,autoincrement" json:"id"`
	CreatedAt time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp" json:"updated_at"`
	TenantID  int64     `bun:"tenant_id,notnull" json:"tenant_id"`
	Name      string    `bun:"name,notnull" json:"name"`
	IsActive  bool      `bun:"is_active,notnull,default:true" json:"is_active"`
	TokenHash string    `bun:"token_hash,notnull" json:"-"`
}

// GetTenantID returns the tenant ID.
func (d *Display) GetTenantID() int64 { return d.TenantID }

// SetTenantID sets the tenant ID.
func (d *Display) SetTenantID(id int64) { d.TenantID = id }

// Repository is the retained data-access shape for displays. All methods are
// tenant-scoped except FindByTokenHash (see its contract comment).
type Repository interface {
	Create(ctx context.Context, d *Display) error
	FindByID(ctx context.Context, id any) (*Display, error)
	List(ctx context.Context, filters map[string]any) ([]*Display, error)
	Update(ctx context.Context, d *Display) error
	UpdateColumns(ctx context.Context, d *Display, columns ...string) (int64, error)
	Delete(ctx context.Context, id any) error
	FindByTokenHash(ctx context.Context, tokenHash string) (*Display, error)
}
