package domain

import (
	"errors"
	"net"
	"time"
)

var (
	ErrAnnouncementNotFound = errors.New("communication: announcement not found")
	ErrInvalidTarget        = errors.New("communication: invalid announcement target")
)

type Announcement struct {
	ID              int64
	Title           string
	Content         string
	Type            string
	Severity        string
	Version         *string
	Active          bool
	PublishedAt     *time.Time
	ExpiresAt       *time.Time
	TargetRoles     []string
	TargetOrgIDs    []int64
	TargetTenantIDs []int64
	CreatedBy       int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

type AnnouncementStats struct {
	AnnouncementID int64
	TargetCount    int
	SeenCount      int
	DismissedCount int
}

type AnnouncementViewDetail struct {
	UserID       int64
	AccountEmail string
	UserName     string
	SeenAt       time.Time
	Dismissed    bool
}

type AuditEntry struct {
	OperatorID int64
	Action     string
	ResourceID int64
	RequestIP  net.IP
	Changes    map[string]any
}

type OperationStats struct {
	Queries                      int
	Rows                         int64
	StatementDuration            time.Duration
	DuplicatePreventionConflicts int
}

func (s *OperationStats) Add(other OperationStats) {
	s.Queries += other.Queries
	s.Rows += other.Rows
	s.StatementDuration += other.StatementDuration
	s.DuplicatePreventionConflicts += other.DuplicatePreventionConflicts
}
