package education

import (
	"errors"
	"time"
)

// Model carries the identity and timestamp columns of every School Structure
// row. It mirrors the shared base shape column for column, so the rows read
// and write exactly as before, without the domain importing the generic
// transaction-runtime vocabulary (#2742).
type Model struct {
	ID        int64     `bun:"id,pk,autoincrement" json:"id"`
	CreatedAt time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp" json:"updated_at"`
}

// TenantModel carries the school a School Structure row belongs to. Its
// accessors keep the rows tenant-scoped for the legacy composition, which
// fills a missing school from the request context.
type TenantModel struct {
	TenantID int64 `bun:"tenant_id,notnull" json:"tenant_id"`
}

// GetTenantID returns the school of the row.
func (t *TenantModel) GetTenantID() int64 { return t.TenantID }

// SetTenantID sets the school of the row.
func (t *TenantModel) SetTenantID(id int64) { t.TenantID = id }

// ErrNotFound is the School Structure repositories' result for a missing row.
// It carries the RepositoryNotFound marker, so callers that classify a
// missing row by that shape (models/base.IsNoRows, api/common.IsNotFound)
// keep recognising it.
var ErrNotFound error = notFoundError{}

type notFoundError struct{}

func (notFoundError) Error() string { return "repository: not found" }

// RepositoryNotFound marks the not-found sentinel for callers that match it
// by shape rather than by identity.
func (notFoundError) RepositoryNotFound() {}

// DatabaseError is the failure shape of the School Structure repositories:
// the operation that failed and the driver's error.
type DatabaseError struct {
	Op  string
	Err error
}

func (e *DatabaseError) Error() string {
	if e.Err == nil {
		return "database error during " + e.Op
	}
	return "database error during " + e.Op + ": " + e.Err.Error()
}

func (e *DatabaseError) Unwrap() error { return e.Err }

// StoreFailure marks the error as the store's failure rather than a refusal
// of the request.
func (e *DatabaseError) StoreFailure() bool { return true }

// IsNotFound reports whether err is a missing-row result of a School
// Structure repository, or of a repository that marks it the same way.
func IsNotFound(err error) bool {
	if errors.Is(err, ErrNotFound) {
		return true
	}
	var marker interface{ RepositoryNotFound() }
	return errors.As(err, &marker)
}
