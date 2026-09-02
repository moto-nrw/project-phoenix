package domain

import (
	"errors"
	"time"
)

var (
	ErrNotFound        = errors.New("person not found")
	ErrTagConflict     = errors.New("tag is already linked to another person")
	ErrAccountConflict = errors.New("account is already linked to another person")
)

type Person struct {
	ID        int64
	CreatedAt time.Time
	UpdatedAt time.Time
	TenantID  int64
	FirstName string
	LastName  string
	Birthday  string
	TagID     *string
	AccountID *int64
	DeletedAt *time.Time
}

func (p Person) IsDeleted() bool { return p.DeletedAt != nil }

type CreatePerson struct {
	FirstName string
	LastName  string
	Birthday  string
	TagID     *string
	AccountID *int64
}

type UpdatePerson struct {
	ID        int64
	FirstName string
	LastName  string
	Birthday  string
	TagID     *string
	AccountID *int64
}

type Filter struct {
	FirstNamePrefix  string
	LastNamePrefix   string
	FullNameContains string
	TagID            string
	AccountIDs       []int64
	Page             int
	PageSize         int
}

type ReleasedTag struct {
	PersonID int64
	TagID    string
}

type OperationStats struct {
	Queries           int64
	Rows              int64
	StatementDuration time.Duration
}

func (s *OperationStats) Add(other OperationStats) {
	s.Queries += other.Queries
	s.Rows += other.Rows
	s.StatementDuration += other.StatementDuration
}
