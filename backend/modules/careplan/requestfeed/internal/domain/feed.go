package domain

import "time"

type Access struct {
	Active             bool
	GeneralRequests    bool
	EnrollmentRequests bool
	SchoolName         string
	Subdomain          string
}

func (a Access) Allowed() bool { return a.Active && (a.GeneralRequests || a.EnrollmentRequests) }

type Subscription struct {
	TenantID   int64
	AccountID  int64
	SchoolName string
	Subdomain  string
}

type Item struct {
	Kind      string
	ID        int64
	CreatedAt time.Time
}
