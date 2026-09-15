package domain

import (
	"errors"
	"fmt"
	"time"
)

type HasSchoolsError struct{ Count int }

func (e *HasSchoolsError) Error() string { return fmt.Sprintf("organization has %d schools", e.Count) }
func (e *HasSchoolsError) Unwrap() error { return ErrHasSchools }

var (
	ErrNotFound       = errors.New("organization not found")
	ErrSlugConflict   = errors.New("organization slug already exists")
	ErrAlreadyDeleted = errors.New("organization is already deleted")
	ErrNotDeleted     = errors.New("organization is not deleted")
	ErrHasSchools     = errors.New("organization has schools")
)

type Organization struct {
	ID        int64
	CreatedAt time.Time
	UpdatedAt time.Time
	Name      string
	Slug      string
	Active    bool
	DeletedAt *time.Time
	Settings  string
}

func (o Organization) IsDeleted() bool { return o.DeletedAt != nil }

type CreateOrganization struct {
	Name   string
	Slug   string
	Active bool
}

type UpdateOrganization struct {
	ID     int64
	Name   string
	Slug   string
	Active bool
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
