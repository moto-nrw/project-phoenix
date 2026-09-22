package organizationtenancy

import (
	"context"
	"errors"
	"fmt"
	"time"
)

var ErrDemoAlreadyRunning = errors.New("demo process already running")

// ErrDemoCapacityReached reports that the configured number of active demo
// schools exists, so no further one is queued (#3466).
var ErrDemoCapacityReached = errors.New("demo capacity reached")

// DemoSchoolState retains the complete synthetic seed contract. It is never
// exposed through a tenant or operator HTTP endpoint: it includes credentials.
type DemoSchoolState struct {
	SchoolID int64
	SeedJSON []byte
}

type DemoStateEngine interface {
	LoadDemoSchool(context.Context, string) (*DemoSchoolState, error)
	RememberDemoSchool(context.Context, string, DemoSchoolState) error
	WithDemoLease(context.Context, string, func(context.Context) error) error
}

// DemoSchools is the provisioning-state capability for the isolated demo
// process, separate from the request-scoped school administration capability.
type DemoSchools struct{ engine DemoStateEngine }

func NewDemoSchools(engine DemoStateEngine) *DemoSchools {
	if engine == nil {
		panic("demo schools require persistence")
	}
	return &DemoSchools{engine: engine}
}

func (d *DemoSchools) LoadDemoSchool(ctx context.Context, name string) (*DemoSchoolState, error) {
	if name == "" {
		return nil, fmt.Errorf("demo school name is required")
	}
	return d.engine.LoadDemoSchool(ctx, name)
}

func (d *DemoSchools) RememberDemoSchool(ctx context.Context, name string, state DemoSchoolState) error {
	if name == "" || state.SchoolID <= 0 || len(state.SeedJSON) == 0 {
		return fmt.Errorf("demo school name, school ID and seed state are required")
	}
	return d.engine.RememberDemoSchool(ctx, name, state)
}

func (d *DemoSchools) WithDemoLease(ctx context.Context, name string, run func(context.Context) error) error {
	if name == "" || run == nil {
		return fmt.Errorf("demo lease name and callback are required")
	}
	return d.engine.WithDemoLease(ctx, name, run)
}

// DemoSchoolOrder is a demo school of the public demo waiting for its seed
// (#3463). Seeded reports a stored seed state: only the first tick is missing.
type DemoSchoolOrder struct {
	Slug       string
	SchoolName string
	PersonName string
	Attempts   int
	Seeded     bool
}

// DemoQueueEngine is the demo process's side of the queue.
type DemoQueueEngine interface {
	ReleaseDemoSchoolOrders(context.Context) error
	ClaimDemoSchoolOrder(context.Context) (*DemoSchoolOrder, error)
	// FinishDemoSchoolOrder opens the school and names the visitor's
	// caregiver and parent account; zero names none.
	FinishDemoSchoolOrder(ctx context.Context, slug string, visitorAccountID, visitorParentAccountID int64) error
	FailDemoSchoolOrder(ctx context.Context, slug string, maxAttempts int) (failed bool, err error)
	// ReturnDemoSchoolOrder hands a claimed order back without counting the
	// attempt, when something other than the order stopped the seed.
	ReturnDemoSchoolOrder(ctx context.Context, slug string) error
	ReadyDemoSchools(context.Context) ([]string, error)
	// ActiveDemoSchools lists the ready schools a visitor entered since the
	// instant; the simulation serves only these (#3464).
	ActiveDemoSchools(ctx context.Context, since time.Time) ([]string, error)
	// RetireDemoSchools soft-deletes the schools of the named orders (#3470)
	// and returns how many it hid. A hidden school is neither ticked nor
	// counted, and cannot be entered.
	RetireDemoSchools(ctx context.Context, slugs []string) (int, error)
}

// DemoSchoolQueue hands the demo process its orders. Only the process that
// holds the demo lease may use it.
type DemoSchoolQueue struct{ DemoQueueEngine }

func NewDemoSchoolQueue(engine DemoQueueEngine) *DemoSchoolQueue {
	if engine == nil {
		panic("demo school queue requires persistence")
	}
	return &DemoSchoolQueue{DemoQueueEngine: engine}
}

// Demo school progress as the demo access sees it.
const (
	DemoSchoolPreparing = "preparing"
	DemoSchoolReady     = "ready"
	DemoSchoolFailed    = "failed"
)

// DemoSchoolProgress never carries the seed state.
type DemoSchoolProgress struct {
	Status           string
	SchoolID         int64
	VisitorAccountID int64
	// VisitorParentAccountID is the parent carrying the visitor's name (#3468).
	VisitorParentAccountID int64
}

type DemoOrderEngine interface {
	OrderDemoSchool(ctx context.Context, schoolName, personName string) (slug string, err error)
	DemoSchoolProgress(ctx context.Context, slug string) (*DemoSchoolProgress, error)
	MarkDemoSchoolUsed(ctx context.Context, slug string, usedAt time.Time) error
	RetireDemoSchool(ctx context.Context, slug string) error
}

// DemoSchoolOrders is the serving backend's side: it queues a demo school
// per demo access and reads its progress inside the caller's administrative
// transaction.
type DemoSchoolOrders struct{ engine DemoOrderEngine }

func NewDemoSchoolOrders(engine DemoOrderEngine) *DemoSchoolOrders {
	if engine == nil {
		panic("demo school orders require persistence")
	}
	return &DemoSchoolOrders{engine: engine}
}

// OrderDemoSchool queues a school named schoolName and returns its slug: the
// name as a DNS label plus a random suffix. It reports ErrDemoCapacityReached
// while the configured number of active demo schools exists.
func (d *DemoSchoolOrders) OrderDemoSchool(ctx context.Context, schoolName, personName string) (string, error) {
	if schoolName == "" || personName == "" {
		return "", fmt.Errorf("demo school and person name are required")
	}
	return d.engine.OrderDemoSchool(ctx, schoolName, personName)
}

// DemoSchoolProgress returns nil for a slug nobody ordered.
func (d *DemoSchoolOrders) DemoSchoolProgress(ctx context.Context, slug string) (*DemoSchoolProgress, error) {
	return d.engine.DemoSchoolProgress(ctx, slug)
}

// MarkDemoSchoolUsed notes that a visitor entered the school, which keeps its
// simulation running (#3464).
func (d *DemoSchoolOrders) MarkDemoSchoolUsed(ctx context.Context, slug string, usedAt time.Time) error {
	return d.engine.MarkDemoSchoolUsed(ctx, slug, usedAt)
}

// RetireDemoSchool soft-deletes the school of the order when a visitor
// starts over (#3470). The order keeps its row: it no longer holds a place,
// is not ticked and cannot be entered. An order without a school (still
// preparing, or failed) is left as it is.
func (d *DemoSchoolOrders) RetireDemoSchool(ctx context.Context, slug string) error {
	if slug == "" {
		return fmt.Errorf("demo school slug is required")
	}
	return d.engine.RetireDemoSchool(ctx, slug)
}
