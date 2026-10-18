//go:build testhooks

package api

import (
	"context"
	"errors"
	"log/slog"

	auditCompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The Care Plan types a suite composing the offering-change review through
// api/testutil names.
type (
	OfferingChangeTestCapability = careplan.OfferingChangeCapability
	OfferingChangeTestBookings   = careplan.BookingMaterializationCapability
	OfferingChangeTestCatalog    = careplanCompose.OfferingChangeCatalog
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
// test database with the owner bindings of Factory.
func NewOfferingChangeTestModule(db transactionruntime.Database, options OfferingChangeTestOptions) (careplan.OfferingChangeCapability, error) {
	if options.Settings == nil || options.Bookings == nil {
		return nil, errors.New("offering change test module: settings and bookings are required")
	}
	auditCommand, err := auditCompose.NewCommand(NewTestAuditStore(db), func(auditCompose.AppendObservation) {})
	if err != nil {
		return nil, err
	}
	students, repos, err := NewStudentTestRepositories(db, auditCommand)
	if err != nil {
		return nil, err
	}
	studentPeople, err := NewStudentPeopleRows(db, MustNewMembershipTestRepositories(db))
	if err != nil {
		return nil, err
	}
	deps, err := offeringChangeOwners(
		students.CarePlan, options.Catalog, repos.Enrollment(), repos.Student, studentPeople.CareWithdrawal, options.Settings,
		offeringChangeTestScope(options.Scope), requestTokenReads.Permissions, nil, nil,
	)
	if err != nil {
		return nil, err
	}
	deps.Planning = ManualPlanningReader{Query: PlanningQuery{DB: db}, CourseGroups: repos.Timetable}
	deps.Bookings = options.Bookings
	deps.Today = options.Today
	deps.Logger = slog.Default()
	return newOfferingChanges(deps)
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

// OfferingChangeOption replaces one collaborator of the offering-change
// review a suite composes.
type OfferingChangeOption func(*OfferingChangeTestOptions)

// WithOfferingChangeCatalog intercepts the care-offering reads.
func WithOfferingChangeCatalog(catalog OfferingChangeTestCatalog) OfferingChangeOption {
	return func(options *OfferingChangeTestOptions) { options.Catalog = catalog }
}

// WithOfferingChangeToday pins the review's calendar day.
func WithOfferingChangeToday(today func() calendar.Date) OfferingChangeOption {
	return func(options *OfferingChangeTestOptions) { options.Today = today }
}

// NewOfferingChanges composes Care Plan's offering-change review over the
// test database with the owner bindings of the server. The settings answer
// the offering-change flags; approvals apply through bookings.
func NewOfferingChanges(
	db transactionruntime.Database,
	settings interface {
		ResolveBool(context.Context, string) (bool, error)
		ResolveString(context.Context, string) (string, error)
	},
	bookings OfferingChangeTestBookings,
	options ...OfferingChangeOption,
) (OfferingChangeTestCapability, error) {
	chosen := OfferingChangeTestOptions{Settings: settings, Bookings: bookings}
	for _, option := range options {
		option(&chosen)
	}
	return NewOfferingChangeTestModule(db, chosen)
}
