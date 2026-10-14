package services

import (
	"context"
	"errors"
	"log/slog"

	"github.com/uptrace/bun"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The Care Plan types a suite composing the offering-change review through
// api/testutil names.
type (
	OfferingChangeTestCapability = careplan.OfferingChangeCapability
	OfferingChangeTestBookings   = careplan.BookingMaterializationCapability
	OfferingChangeTestCatalog    = careplanCompose.OfferingChangeCatalog
	OfferingReviewTestQuery      = careplan.OfferingReviewQuery
)

// OfferingChangeTestOptions replace collaborators of Care Plan's
// offering-change review (#3561) a suite drives. Settings and Bookings are
// required. Nil values read the care offerings from the Care Plan capability
// over the test database, the planning through the Timetable owner, review
// the whole school for an admin and nobody else, and use the real calendar
// day.
type OfferingChangeTestOptions struct {
	Settings interface {
		ResolveBool(context.Context, string) (bool, error)
		ResolveString(context.Context, string) (string, error)
	}
	Bookings careplan.BookingMaterializationCapability
	Catalog  careplanCompose.OfferingChangeCatalog
	Scope    func(ctx context.Context) (schoolWide bool, groupIDs []int64, err error)
	Today    func() calendar.Date
}

// NewOfferingChangeTestModule composes the offering-change review over the
// test database with the owner bindings of services.Factory.
func NewOfferingChangeTestModule(db *bun.DB, options OfferingChangeTestOptions) (careplan.OfferingChangeCapability, error) {
	if options.Settings == nil || options.Bookings == nil {
		return nil, errors.New("offering change test module: settings and bookings are required")
	}
	auditCommand, err := auditCompose.NewCommand(NewTestAuditStore(db), func(auditCompose.AppendObservation) {})
	if err != nil {
		return nil, err
	}
	repos, err := NewStudentTestRepositories(db, auditCommand)
	if err != nil {
		return nil, err
	}
	return NewOfferingChanges(OfferingChangeInputs{
		CarePlan: repos.CarePlan, Catalog: options.Catalog, Enrollment: repos.Enrollment(), Students: repos.Student,
		Withdrawals: repos.CareWithdrawal, Settings: options.Settings, Planning: ManualPlanningReader{Query: enrollmentCompose.NewManualPlanningQuery(db), CourseGroups: repos.Timetable},
		Bookings: options.Bookings, Reviews: offeringChangeTestScope(options.Scope), Permissions: testRequestTokens.Permissions, Today: options.Today,
		Logger: slog.Default(),
	})
}

// offeringChangeTestScope is the suite's review scope, or the one an admin
// has without a review policy: the whole school.
type offeringChangeTestScope func(ctx context.Context) (bool, []int64, error)

func (s offeringChangeTestScope) Scope(ctx context.Context, permissions []string) (bool, []int64, error) {
	if s != nil {
		return s(ctx)
	}
	return securityruntime.HasAdminWildcard(permissions), nil, nil
}
