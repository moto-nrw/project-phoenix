package organizationtenancy

import (
	"context"
	"errors"
	"time"
)

// StandingDemoSchoolSlug is the shared demo school `go run ./cmd/devtools
// demo` provisions. Every demo access enters it while the fallback is
// selected; accesses issued before #3463 carry it as well.
const StandingDemoSchoolSlug = "messe-demo"

// StandingDemoSchool is the slug every demo access enters, the standing
// school when it is selected, otherwise none.
func StandingDemoSchool(selected bool) string {
	if selected {
		return StandingDemoSchoolSlug
	}
	return ""
}

// ErrDemoRestartRefused refuses to restart a demo school others share.
var ErrDemoRestartRefused = errors.New("the standing demo school cannot be restarted")

// DemoSchoolLookup is the school read the demo access enters a school by.
type DemoSchoolLookup interface {
	FindSchool(ctx context.Context, id int64) (School, error)
	FindSchoolBySlug(ctx context.Context, slug string) (School, error)
}

// DemoSchoolEntry is what a visitor can enter: whether the school exists,
// its progress (DemoSchoolPreparing, DemoSchoolReady, DemoSchoolFailed) and,
// once it is ready, the school and the visitor's accounts. A zero AccountID
// signs in the school's administrator, as in the standing demo school.
type DemoSchoolEntry struct {
	Exists          bool
	Status          string
	SchoolID        int64
	AccountID       int64
	ParentAccountID int64
}

// DemoSchoolAccess runs a visitor's demo school: one school ordered per
// access, or the standing school every access shares when it is selected.
type DemoSchoolAccess struct {
	schools  DemoSchoolLookup
	orders   *DemoSchoolOrders
	standing string
}

// NewDemoSchoolAccess binds the access to the schools and the orders;
// standing is the shared school's slug or empty.
func NewDemoSchoolAccess(schools DemoSchoolLookup, orders *DemoSchoolOrders, standing string) DemoSchoolAccess {
	return DemoSchoolAccess{schools: schools, orders: orders, standing: standing}
}

// PrepareDemoSchool returns the slug of the visitor's school: the standing
// school, or a newly ordered one.
func (d DemoSchoolAccess) PrepareDemoSchool(ctx context.Context, schoolName, firstName, lastName string) (string, error) {
	if d.standing != "" {
		return d.standing, nil
	}
	return d.orders.OrderDemoSchool(ctx, schoolName, firstName, lastName)
}

// ReplaceDemoSchool hides the visitor's school and queues a fresh one with
// the same names (#3470). It hides first, so the restart needs no free place
// at the capacity; both happen in the caller's transaction, so a refused
// order leaves the old school as it was. The standing school is shared: a
// restart is ErrDemoRestartRefused.
func (d DemoSchoolAccess) ReplaceDemoSchool(ctx context.Context, slug, schoolName, firstName, lastName string) (string, error) {
	if d.standing != "" || slug == StandingDemoSchoolSlug {
		return "", ErrDemoRestartRefused
	}
	if err := d.orders.RetireDemoSchool(ctx, slug); err != nil {
		return "", err
	}
	return d.PrepareDemoSchool(ctx, schoolName, firstName, lastName)
}

// MarkDemoSchoolUsed notes an entry. The standing school has no order and is
// always simulated, so there is nothing to note for it.
func (d DemoSchoolAccess) MarkDemoSchoolUsed(ctx context.Context, slug string, usedAt time.Time) error {
	if slug == d.standing {
		return nil
	}
	return d.orders.MarkDemoSchoolUsed(ctx, slug, usedAt)
}

// DemoSchoolEntry reports whether the visitor can enter the school. An
// ordered school is ready once its order is and its school is operational;
// the standing school, provisioned without an order, once its school is.
func (d DemoSchoolAccess) DemoSchoolEntry(ctx context.Context, slug string) (DemoSchoolEntry, error) {
	preparing := DemoSchoolEntry{Status: DemoSchoolPreparing}
	progress, err := d.orders.DemoSchoolProgress(ctx, slug)
	if err != nil {
		return preparing, err
	}
	if progress == nil && slug != d.standing {
		return preparing, nil
	}
	if progress != nil && progress.Status != DemoSchoolReady {
		return DemoSchoolEntry{Exists: true, Status: progress.Status}, nil
	}
	var school School
	if progress != nil {
		// The order names the school it seeded; the slug is only its address.
		school, err = d.schools.FindSchool(ctx, progress.SchoolID)
	} else {
		school, err = d.schools.FindSchoolBySlug(ctx, slug)
	}
	if errors.Is(err, ErrSchoolNotFound) {
		return preparing, nil
	}
	if err != nil {
		return preparing, err
	}
	if !school.Operational() {
		return preparing, nil
	}
	entry := DemoSchoolEntry{Exists: true, Status: DemoSchoolReady, SchoolID: school.ID}
	if progress != nil {
		entry.AccountID, entry.ParentAccountID = progress.VisitorAccountID, progress.VisitorParentAccountID
	}
	return entry, nil
}
