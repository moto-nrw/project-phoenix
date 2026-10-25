package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/domain"
)

type Store interface {
	Create(context.Context, organizationtenancy.CreateOrganization) (organizationtenancy.Organization, domain.OperationStats, error)
	Update(context.Context, organizationtenancy.UpdateOrganization) (organizationtenancy.Organization, domain.OperationStats, error)
	FindByID(context.Context, int64, string) (organizationtenancy.Organization, bool, domain.OperationStats, error)
	FindBySlug(context.Context, string) (organizationtenancy.Organization, bool, domain.OperationStats, error)
	List(context.Context) ([]organizationtenancy.Organization, domain.OperationStats, error)
	ListByIDs(context.Context, []int64) ([]organizationtenancy.Organization, domain.OperationStats, error)
	CountByIDs(context.Context, []int64) (int, domain.OperationStats, error)
	CountNonDeletedSchools(context.Context, int64) (int, domain.OperationStats, error)
	SoftDelete(context.Context, int64) (domain.OperationStats, error)
	Restore(context.Context, int64) (domain.OperationStats, error)
	CreateSchool(context.Context, organizationtenancy.CreateSchool) (organizationtenancy.School, domain.OperationStats, error)
	UpdateSchool(context.Context, organizationtenancy.UpdateSchool) (organizationtenancy.School, domain.OperationStats, error)
	// LockSchoolChildQuota serializes a quota change with its membership checks.
	LockSchoolChildQuota(context.Context, int64) (domain.OperationStats, error)
	// SetSchoolChildQuota writes the Kinderkontingent; nil bundles remove it.
	SetSchoolChildQuota(ctx context.Context, id int64, bundles *int, bundleSize int) (organizationtenancy.School, domain.OperationStats, error)
	FindSchoolByID(context.Context, int64, string) (organizationtenancy.School, bool, domain.OperationStats, error)
	FindSchoolBySlug(context.Context, string) (organizationtenancy.School, bool, domain.OperationStats, error)
	FindSchoolByOrganizationAndSlug(context.Context, int64, string) (organizationtenancy.School, bool, domain.OperationStats, error)
	FindSchoolBySubdomain(context.Context, string) (organizationtenancy.School, bool, domain.OperationStats, error)
	ListSchools(context.Context, []int64, *int64, string) ([]organizationtenancy.School, domain.OperationStats, error)
	CountSchoolsByID(context.Context, []int64) (int, domain.OperationStats, error)
	SetSchoolDeleted(context.Context, int64, bool) (domain.OperationStats, error)
	TraegerMembershipStore
}

// TraegerMembershipStore persists platform.organization_accounts (ADR 0084).
// Every method runs on the caller's administrative transaction.
type TraegerMembershipStore interface {
	ListActiveAccountTraegerIDs(ctx context.Context, accountID int64) ([]int64, domain.OperationStats, error)
	ListTraegerSchoolIDs(ctx context.Context, organizationID int64) ([]int64, domain.OperationStats, error)
	ListTraegerMemberships(ctx context.Context, organizationID int64) ([]organizationtenancy.TraegerMembership, domain.OperationStats, error)
	UpsertActiveTraegerMembership(ctx context.Context, organizationID, accountID int64) (organizationtenancy.TraegerMembership, domain.OperationStats, error)
	DeactivateTraegerMembership(ctx context.Context, organizationID, accountID int64) (bool, domain.OperationStats, error)
}

type Transaction interface {
	RunAdmin(context.Context, func(context.Context) error) error
	RunRead(context.Context, func(context.Context) error) error
	// RunTenant runs fn in one transaction scoped to the school tenantID.
	RunTenant(ctx context.Context, tenantID int64, fn func(context.Context) error) error
}

type Observation struct {
	Operation string
	Duration  time.Duration
	Stats     domain.OperationStats
	Err       error
}

type Observer func(Observation)
