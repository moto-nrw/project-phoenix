package contract

import "time"

type Counts struct {
	Organizations int
	Schools       int
	Accounts      int
}
type OrganizationSummary struct {
	ID           int64
	Name         string
	Slug         string
	Active       bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    *time.Time
	Settings     string
	SchoolCount  int
	AccountCount int
}
type SchoolSummary struct {
	ID               int64
	OrganizationID   int64
	OrganizationName string
	Name             string
	Slug             string
	Subdomain        string
	Active           bool
	Hidden           bool
	CreatedAt        time.Time
	UpdatedAt        time.Time
	DeletedAt        *time.Time
	Address          string
	City             string
	Zip              string
	Phone            string
	Email            string
	Settings         string
	AccountCount     int
	// ChildQuotaBundles is nil when the school has no Kinderkontingent.
	ChildQuotaBundles    *int
	ChildQuotaBundleSize int
}
type PWAUsageRow struct {
	TenantID        int64
	Portal          string
	StandaloneUsers int
	EligibleUsers   int
}
type Rows[C, O, S, P any] struct {
	Counts       func(Counts) C
	Organization func(OrganizationSummary) O
	School       func(SchoolSummary) S
	PWAUsage     func(PWAUsageRow) P
}
