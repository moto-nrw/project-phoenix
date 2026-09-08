// Package domain holds the Identity & Access values the module reasons about.
package domain

import (
	"errors"
	"time"
)

var (
	ErrAccountNotFound     = errors.New("account not found")
	ErrTenantRequired      = errors.New("tenant is required")
	ErrGuardianRoleMissing = errors.New("guardian role not found")
)

// GuardianRoleName is the base role every parent-portal login carries.
const GuardianRoleName = "guardian"

type Account struct {
	ID    int64
	Email string
}

type GuardianTenantAccess struct {
	AccountID    int64
	TenantID     int64
	RoleID       int64
	RoleAssigned bool
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
