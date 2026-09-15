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
}
