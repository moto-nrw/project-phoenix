package main

import (
	"context"
	"time"

	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	organizationcompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	presencecompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/tenant"
)

type DemoSchoolRecord struct {
	SchoolID               int64
	SeedJSON               []byte
	VisitorAccountID       int64
	VisitorParentAccountID int64
}

type DemoVisit struct {
	StudentID   int64
	Active, Web bool
	ChangedAt   time.Time
}

// demoSchoolStore is the part of the owner's demo-state capability whose
// signatures carry no owner type.
type demoSchoolStore interface {
	ReserveDemoSchool(ctx context.Context, name string, schoolID, visitorAccountID, visitorParentAccountID int64) error
	WithDemoLease(ctx context.Context, name string, run func(context.Context) error) error
}

// demoSchoolQueue is the part of the owner's queue whose signatures carry no
// owner type.
type demoSchoolQueue interface {
	ReleaseDemoSchoolOrders(context.Context) error
	RequeueDeferredDemoSchoolOrders(context.Context) error
	RequeueDeferredDemoSchoolOrder(context.Context, string) error
	FinishDemoSchoolOrder(ctx context.Context, slug string, visitorAccountID, visitorParentAccountID int64) error
	FailDemoSchoolOrder(ctx context.Context, slug string, maxAttempts int) (bool, error)
	ReturnDemoSchoolOrder(ctx context.Context, slug string) error
	ReadyDemoSchools(context.Context) ([]string, error)
	ActiveDemoSchools(ctx context.Context, since time.Time) ([]string, error)
	RetireDemoSchools(ctx context.Context, slugs []string) (hidden, erased int, err error)
}

type demoAccessExpiry interface {
	ExpireDemoAccesses(ctx context.Context) (int, []string, error)
}

// DemoRuntime composes only the owner capabilities needed by the sidecar.
// It deliberately does not construct the retained API/service factory graph.
// The calls whose signatures carry owner types are bound as functions in
// NewDemoRuntime, so the dev tools import no owner's public package.
type DemoRuntime struct {
	schools      demoSchoolStore
	queue        demoSchoolQueue
	expiry       demoAccessExpiry
	transactions tenant.UnitOfWork

	loadSchool   func(context.Context, string) (*DemoSchoolRecord, error)
	storeSchool  func(ctx context.Context, name string, state DemoSchoolRecord, update bool) error
	claimOrder   func(context.Context) (*DemoSchoolOrder, error)
	latestVisits func(ctx context.Context, webDeviceID int64, fromDate, untilDate string) ([]DemoVisit, error)
}

// NewDemoRuntime composes the sidecar on its restricted demo connection, the
// standing demo's database with its own unit of work; now is the clock the
// expiry of demo accesses reads (#3470).
func NewDemoRuntime(db transactionruntime.Database, now func() time.Time) (*DemoRuntime, error) {
	schools, err := organizationcompose.NewDemoSchools(db.Pool())
	if err != nil {
		return nil, err
	}
	expiry, err := identityaccessCompose.NewDemoAccessExpiry(db.Pool(), now)
	if err != nil {
		return nil, err
	}
	presence, err := presencecompose.New(presencecompose.Dependencies{DB: db.Pool(), Observe: func(presencecompose.Observation) {}})
	if err != nil {
		return nil, err
	}
	queue, err := organizationcompose.NewDemoSchoolQueue(db.Pool())
	if err != nil {
		return nil, err
	}
	return &DemoRuntime{
		schools: schools, queue: queue, expiry: expiry, transactions: db.UnitOfWork(),
		loadSchool: func(ctx context.Context, name string) (*DemoSchoolRecord, error) {
			state, err := schools.LoadDemoSchool(ctx, name)
			if state == nil || err != nil {
				return nil, err
			}
			return &DemoSchoolRecord{SchoolID: state.SchoolID, SeedJSON: state.SeedJSON, VisitorAccountID: state.VisitorAccountID, VisitorParentAccountID: state.VisitorParentAccountID}, nil
		},
		storeSchool: func(ctx context.Context, name string, record DemoSchoolRecord, update bool) error {
			if update {
				return schools.UpdateDemoSchoolFields(ctx, name, record.SchoolID, record.SeedJSON, record.VisitorAccountID, record.VisitorParentAccountID)
			}
			return schools.RememberDemoSchoolFields(ctx, name, record.SchoolID, record.SeedJSON, record.VisitorAccountID, record.VisitorParentAccountID)
		},
		claimOrder: func(ctx context.Context) (*DemoSchoolOrder, error) {
			order, err := queue.ClaimDemoSchoolOrder(ctx)
			if order == nil || err != nil {
				return nil, err
			}
			return &DemoSchoolOrder{
				Slug: order.Slug, SchoolName: order.SchoolName, FirstName: order.FirstName, LastName: order.LastName,
				Attempts: order.Attempts, Seeded: order.Seeded,
			}, nil
		},
		latestVisits: func(ctx context.Context, webDeviceID int64, fromDate, untilDate string) ([]DemoVisit, error) {
			rows, err := presence.LatestDemoVisits(ctx, webDeviceID, fromDate, untilDate)
			if err != nil {
				return nil, err
			}
			result := make([]DemoVisit, 0, len(rows))
			for _, row := range rows {
				result = append(result, DemoVisit{StudentID: row.StudentID, Active: row.Active, Web: row.Web, ChangedAt: row.ChangedAt})
			}
			return result, nil
		},
	}, nil
}

// ExpireDemoAccesses deletes the demo accesses 14 days past their last use
// (#3470) and returns how many, plus the demo schools no access enters any
// more.
func (d *DemoRuntime) ExpireDemoAccesses(ctx context.Context) (int, []string, error) {
	return d.expiry.ExpireDemoAccesses(tenant.WithUnitOfWork(ctx, d.transactions))
}

// RetireDemoSchools hides the named demo schools, then erases the visitors
// of the demo schools out of use (#4048). It returns how many schools it hid
// and how many orders it erased.
func (d *DemoRuntime) RetireDemoSchools(ctx context.Context, slugs []string) (int, int, error) {
	return d.queue.RetireDemoSchools(ctx, slugs)
}

// DemoSchoolOrder is a demo school of the public demo waiting for its seed
// (#3463). Seeded orders only miss their first tick.
type DemoSchoolOrder struct {
	Slug, SchoolName    string
	FirstName, LastName string
	Attempts            int
	Seeded              bool
}

// ReleaseDemoSchoolOrders returns the orders of a stopped process to the queue.
func (d *DemoRuntime) ReleaseDemoSchoolOrders(ctx context.Context) error {
	return d.queue.ReleaseDemoSchoolOrders(ctx)
}

// RequeueDeferredDemoSchoolOrders retries interrupted optional demo work as
// a clean seed, rather than replaying partial API writes in the open school.
func (d *DemoRuntime) RequeueDeferredDemoSchoolOrders(ctx context.Context) error {
	return d.queue.RequeueDeferredDemoSchoolOrders(ctx)
}

func (d *DemoRuntime) RequeueDeferredDemoSchoolOrder(ctx context.Context, slug string) error {
	return d.queue.RequeueDeferredDemoSchoolOrder(ctx, slug)
}

// ClaimDemoSchoolOrder takes the oldest waiting order, or nil.
func (d *DemoRuntime) ClaimDemoSchoolOrder(ctx context.Context) (*DemoSchoolOrder, error) {
	return d.claimOrder(ctx)
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
	return d.loadSchool(ctx, name)
}

// ReserveDemoSchool remembers a newly bootstrapped school before its full
// seed contract is available for persistence.
func (d *DemoRuntime) ReserveDemoSchool(ctx context.Context, name string, schoolID, visitorAccountID, visitorParentAccountID int64) error {
	return d.schools.ReserveDemoSchool(ctx, name, schoolID, visitorAccountID, visitorParentAccountID)
}

func (d *DemoRuntime) RememberDemoSchool(ctx context.Context, name string, state DemoSchoolRecord) error {
	return d.storeSchool(ctx, name, state, false)
}

func (d *DemoRuntime) UpdateDemoSchool(ctx context.Context, name string, state DemoSchoolRecord) error {
	return d.storeSchool(ctx, name, state, true)
}

func (d *DemoRuntime) WithDemoLease(ctx context.Context, name string, run func(context.Context) error) error {
	return d.schools.WithDemoLease(ctx, name, run)
}

func (d *DemoRuntime) TenantRuntime() tenant.UnitOfWork { return d.transactions }

// LatestDemoVisits translates the owner snapshot for the demo CLI.
// The caller must provide its tenant-scoped transaction context.
func (d *DemoRuntime) LatestDemoVisits(ctx context.Context, webDeviceID int64, fromDate, untilDate string) ([]DemoVisit, error) {
	return d.latestVisits(ctx, webDeviceID, fromDate, untilDate)
}

// ReturnDemoSchoolOrder hands a claimed order back without counting the attempt.
func (d *DemoRuntime) ReturnDemoSchoolOrder(ctx context.Context, slug string) error {
	return d.queue.ReturnDemoSchoolOrder(ctx, slug)
}
