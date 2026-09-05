package activities

import (
	"context"
	"time"
)

type Model struct {
	ID        int64     `bun:"id,pk,autoincrement" json:"id"`
	CreatedAt time.Time `bun:"created_at,nullzero,notnull,default:current_timestamp" json:"created_at"`
	UpdatedAt time.Time `bun:"updated_at,nullzero,notnull,default:current_timestamp" json:"updated_at"`
}

type TenantModel struct {
	TenantID int64 `bun:"tenant_id,notnull" json:"tenant_id"`
}

func (m *TenantModel) GetTenantID() int64   { return m.TenantID }
func (m *TenantModel) SetTenantID(id int64) { m.TenantID = id }

type Repository[T any] interface {
	Create(context.Context, T) error
	FindByID(context.Context, any) (T, error)
	Update(context.Context, T) error
	Delete(context.Context, any) error
	List(context.Context, *QueryOptions) ([]T, error)
}

// QueryOptions remains only for compatibility; owner queries expose bounded filters.
type QueryOptions struct {
	StudentIDs []int64
	Limit      int
	Offset     int
}

type SupervisorQueryOptions = QueryOptions
type StudentEnrollmentQueryOptions = QueryOptions

type NullInt64 struct {
	Int64 int64
	Valid bool
}

type NullInt16 struct {
	Int16 int16
	Valid bool
}

type NullString struct {
	String string
	Valid  bool
}

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

type notFoundError struct{}

func (notFoundError) Error() string       { return "repository: not found" }
func (notFoundError) RepositoryNotFound() {}

var ErrNotFound error = notFoundError{}

func WrapDatabaseError(operation string, err error) error {
	return &DatabaseError{Op: operation, Err: err}
}

func WrapNotFoundDatabaseError(operation string) error {
	return WrapDatabaseError(operation, ErrNotFound)
}
