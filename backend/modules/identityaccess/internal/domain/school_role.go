package domain

import "errors"

var ErrRoleNotFound = errors.New("role not found")

type SchoolRole struct {
	ID          int64
	TenantID    *int64
	Name        string
	IsSystem    bool
	BaseRole    *string
	Permissions []string
	// SchoolPortal marks a school role made from the Lehrkraft template (#4574).
	SchoolPortal bool
	// NameKey is LOWER(name) as PostgreSQL computes it. ListSchoolRoles fills
	// it so a caller can match names exactly as FindSchoolRoleByName does,
	// which compares the trimmed, lowered input with LOWER(name) (#5887).
	NameKey string
}
