package api

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/database"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	organizationcompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	presencecompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/uptrace/bun"
)

type DemoSchoolRecord struct {
	SchoolID int64
	SeedJSON []byte
}

type DemoVisit struct {
	StudentID   int64
	Active, Web bool
	ChangedAt   time.Time
}

// DemoRuntime composes only the owner capabilities needed by the sidecar.
// It deliberately does not construct the retained API/service factory graph.
type DemoRuntime struct {
	schools      *organizationtenancy.DemoSchools
	queue        *organizationtenancy.DemoSchoolQueue
	expiry       services.DemoAccessExpiry
	presence     *studentpresence.Module
	transactions services.TenantRuntime
}

// NewDemoRuntime composes the sidecar on its privileged connection; now is
// the clock the expiry of demo accesses reads (#3470).
func NewDemoRuntime(db *bun.DB, now func() time.Time) (*DemoRuntime, error) {
	schools, err := organizationcompose.NewDemoSchools(db)
	if err != nil {
		return nil, err
	}
	expiry, err := services.NewDemoAccessExpiry(db, now)
	if err != nil {
		return nil, err
	}
	presence, err := presencecompose.New(presencecompose.Dependencies{DB: db, Observe: func(presencecompose.Observation) {}})
	if err != nil {
		return nil, err
	}
	pgRuntime, err := database.NewDemoPostgresUnitOfWork(db, func(context.Context, time.Duration) {})
	if err != nil {
		return nil, err
	}
	transactions, err := services.BindTenantRuntime(pgRuntime.WithinTenant, pgRuntime.WithinAdmin, pgRuntime, func(error) bool { return false })
	if err != nil {
		return nil, err
	}
	queue, err := organizationcompose.NewDemoSchoolQueue(db)
	if err != nil {
		return nil, err
	}
	return &DemoRuntime{schools: schools, queue: queue, expiry: expiry, presence: presence, transactions: transactions}, nil
}

// ExpireDemoAccesses deletes the demo accesses 14 days past their last use
// (#3470) and returns how many, plus the demo schools no access enters any
// more.
func (d *DemoRuntime) ExpireDemoAccesses(ctx context.Context) (int, []string, error) {
	return d.expiry.ExpireDemoAccesses(ctx)
}

// RetireDemoSchools hides the named demo schools and returns how many it hid.
func (d *DemoRuntime) RetireDemoSchools(ctx context.Context, slugs []string) (int, error) {
	return d.queue.RetireDemoSchools(ctx, slugs)
}

// DemoSchoolOrder is a demo school of the public demo waiting for its seed
// (#3463). Seeded orders only miss their first tick.
type DemoSchoolOrder struct {
	Slug, SchoolName, PersonName string
	Attempts                     int
	Seeded                       bool
}

// ReleaseDemoSchoolOrders returns the orders of a stopped process to the queue.
func (d *DemoRuntime) ReleaseDemoSchoolOrders(ctx context.Context) error {
	return d.queue.ReleaseDemoSchoolOrders(ctx)
}

// ClaimDemoSchoolOrder takes the oldest waiting order, or nil.
func (d *DemoRuntime) ClaimDemoSchoolOrder(ctx context.Context) (*DemoSchoolOrder, error) {
	order, err := d.queue.ClaimDemoSchoolOrder(ctx)
	if order == nil || err != nil {
		return nil, err
	}
	return &DemoSchoolOrder{
		Slug: order.Slug, SchoolName: order.SchoolName, PersonName: order.PersonName, Attempts: order.Attempts, Seeded: order.Seeded,
	}, nil
}

// FinishDemoSchoolOrder opens the school for its demo access.
func (d *DemoRuntime) FinishDemoSchoolOrder(ctx context.Context, slug string, visitorAccountID, visitorParentAccountID int64) error {
	return d.queue.FinishDemoSchoolOrder(ctx, slug, visitorAccountID, visitorParentAccountID)
}

// FailDemoSchoolOrder queues the order again until maxAttempts are used.
func (d *DemoRuntime) FailDemoSchoolOrder(ctx context.Context, slug string, maxAttempts int) (bool, error) {
	return d.queue.FailDemoSchoolOrder(ctx, slug, maxAttempts)
}

// ReadyDemoSchools lists every demo school that can be entered.
func (d *DemoRuntime) ReadyDemoSchools(ctx context.Context) ([]string, error) {
	return d.queue.ReadyDemoSchools(ctx)
}

// ActiveDemoSchools lists the ready demo schools a visitor entered since the
// instant; only these get simulation ticks (#3464).
func (d *DemoRuntime) ActiveDemoSchools(ctx context.Context, since time.Time) ([]string, error) {
	return d.queue.ActiveDemoSchools(ctx, since)
}

func (d *DemoRuntime) LoadDemoSchool(ctx context.Context, name string) (*DemoSchoolRecord, error) {
	state, err := d.schools.LoadDemoSchool(ctx, name)
	if state == nil || err != nil {
		return nil, err
	}
	return &DemoSchoolRecord{SchoolID: state.SchoolID, SeedJSON: state.SeedJSON}, nil
}

func (d *DemoRuntime) RememberDemoSchool(ctx context.Context, name string, state DemoSchoolRecord) error {
	return d.schools.RememberDemoSchool(ctx, name, organizationtenancy.DemoSchoolState{SchoolID: state.SchoolID, SeedJSON: state.SeedJSON})
}

func (d *DemoRuntime) WithDemoLease(ctx context.Context, name string, run func(context.Context) error) error {
	return d.schools.WithDemoLease(ctx, name, run)
}

func (d *DemoRuntime) TenantRuntime() services.TenantRuntime { return d.transactions }

// LatestDemoVisits translates the owner snapshot for the demo CLI.
// The caller must provide its tenant-scoped transaction context.
func (d *DemoRuntime) LatestDemoVisits(ctx context.Context, webDeviceID int64, fromDate, untilDate string) ([]DemoVisit, error) {
	rows, err := d.presence.LatestDemoVisits(ctx, webDeviceID, fromDate, untilDate)
	if err != nil {
		return nil, err
	}
	result := make([]DemoVisit, 0, len(rows))
	for _, row := range rows {
		result = append(result, DemoVisit{StudentID: row.StudentID, Active: row.Active, Web: row.Web, ChangedAt: row.ChangedAt})
	}
	return result, nil
}

// ReturnDemoSchoolOrder hands a claimed order back without counting the attempt.
func (d *DemoRuntime) ReturnDemoSchoolOrder(ctx context.Context, slug string) error {
	return d.queue.ReturnDemoSchoolOrder(ctx, slug)
}
