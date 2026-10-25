package organizationtenancy

import (
	"context"
	"time"
)

// DashboardCounts are the platform-wide counts the operator overview shows
// before devices are attached.
type DashboardCounts struct {
	Organizations int
	Schools       int
	Accounts      int
}

// DashboardOrganization is an organisation with its school and account counts.
type DashboardOrganization struct {
	ID           int64
	Name         string
	Slug         string
	Active       bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    *time.Time
	Settings     string
	TestSystem   bool
	SchoolCount  int
	AccountCount int
}

// DashboardSchool is a school with its organisation's name and account count.
type DashboardSchool struct {
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

// PWAPortalStaff and PWAPortalParent are the portal labels the operator
// dashboard's PWA usage query writes (the stored push portals of Delivery).
const (
	PWAPortalStaff  = "staff"
	PWAPortalParent = "parent"
)

// DashboardPWAUsage is one (school, portal) bucket of PWA standalone usage.
type DashboardPWAUsage struct {
	TenantID        int64
	Portal          string
	StandaloneUsers int
	EligibleUsers   int
}

// OperatorDashboard supplies the projection reads required by provisioning.
type OperatorDashboard interface {
	Counts(context.Context) (DashboardCounts, error)
	OrganizationSummaries(context.Context) ([]DashboardOrganization, error)
	SchoolSummaries(context.Context, *int64) ([]DashboardSchool, error)
	PWAUsage(context.Context, int64, time.Duration) ([]DashboardPWAUsage, error)
	AllSchoolsPWAUsage(context.Context, time.Duration) ([]DashboardPWAUsage, error)
}
