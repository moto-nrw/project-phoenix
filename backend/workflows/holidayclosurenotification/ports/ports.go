// Package ports declares what the holiday closure notification consumes of
// its owners (ADR 0196). The root binds every port.
package ports

import (
	"context"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/moto-ogs/project-phoenix/modules/organizationtenancy"
	"github.com/moto-ogs/project-phoenix/modules/schoolcalendar"
)

// Proposals is School Calendar's proposal side.
type Proposals = schoolcalendar.HolidayClosureProposals

// Queue is School Calendar's command side.
type Queue = schoolcalendar.HolidayClosureEventQueue

// Closures is what the workflow asks of Enrollment, always in the Träger
// transaction of the Träger it names.
type Closures interface {
	PlanHolidayClosureOperations(ctx context.Context, organizationID int64, operations []enrollment.HolidayClosureOperation) ([]enrollment.HolidayClosureImpact, error)
	NotifyHolidayClosureOperation(ctx context.Context, organizationID int64, operation enrollment.HolidayClosureOperation) (enrollment.HolidayClosureOperationResult, error)
	HolidayClosureOperationsDelivery(ctx context.Context, organizationID int64, operationIDs []string) (map[string]enrollment.HolidayClosureDelivery, error)
}

// Units opens the units of work. WithinSchool and WithinTraeger each open
// a unit of their own and refuse to nest in another one.
type Units interface {
	WithinSchool(ctx context.Context, schoolID int64, fn func(context.Context) error) error
	WithinTraeger(ctx context.Context, organizationID int64, fn func(context.Context) error) error
	WithinAdmin(ctx context.Context, fn func(context.Context) error) error
}

// Schools lists the schools of a Träger in the caller's Träger transaction
// (ADR 0196 section 3.1): Organisation & Tenancy's read, the same the
// Träger calendar binds. The workflow keeps the current, undeleted ones.
type Schools interface {
	ListSchoolsByOrganization(ctx context.Context, organizationID int64) ([]organizationtenancy.School, error)
}

// ClosureMailRecovery names existing mail only; an unknown outcome never authorizes a retry.
type ClosureMailRecovery struct {
	Key     string
	Unknown bool
}

type Mails interface {
	HolidayClosureMailRecovery(context.Context, int64) ([]ClosureMailRecovery, error)
	RetryFailedTraegerEmails(context.Context, int64, []string) (int64, error)
}
