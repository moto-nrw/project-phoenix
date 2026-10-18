package contract

import "time"

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
type Target struct {
	PublishedAt     *time.Time
	TargetRoles     []string
	TargetOrgIDs    []int64
	TargetTenantIDs []int64
}
type ViewDetail struct {
	UserID       int64
	AccountEmail string
	UserName     string
	SeenAt       time.Time
	Dismissed    bool
}
type Stats struct {
	Queries           int
	Rows              int64
	StatementDuration time.Duration
}
type Rows[A, V, S any] struct {
	Announcement func(Announcement) A
	Target       func(A) Target
	View         func(ViewDetail) V
	Stats        func(Stats) S
}
