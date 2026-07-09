// Package display contains the info-point display domain: tenant-scoped
// screens (TVs, smartboards) that render a read-only dashboard authenticated
// by an opaque URL token (issue #1325).
package display

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/models/base"
)

// Sentinel errors shared between repository, service, and handler layers.
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
	base.Model `bun:"schema:display,table:displays"`
	base.TenantModel
	base.Nameable
	base.Activatable
	TokenHash string `bun:"token_hash,notnull" json:"-"`
}

// Repository defines data access for displays. All methods are tenant-scoped
// via RLS except FindByTokenHash (see its contract comment).
type Repository interface {
	Create(ctx context.Context, d *Display) error
	FindByID(ctx context.Context, id any) (*Display, error)
	List(ctx context.Context, filters map[string]any) ([]*Display, error)
	Update(ctx context.Context, d *Display) error
	UpdateColumns(ctx context.Context, d *Display, columns ...string) (int64, error)
	Delete(ctx context.Context, id any) error
	FindByTokenHash(ctx context.Context, tokenHash string) (*Display, error)
}
