// Package staffoffboarding coordinates the staff lifecycle across owner
// capabilities. It owns no tables and performs no irreversible work itself.
package staffoffboarding

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
)

var (
	ErrConflict     = errors.New("staff offboarding preview is no longer current")
	ErrInUse        = errors.New("staff is actively supervising or holds a group handover")
	ErrUnauthorized = errors.New("staff offboarding requires an authorized school principal")
)

type Actor struct {
	TenantID  int64
	AccountID int64
	StaffID   int64
	Username  string
}

type AccessPreview struct {
	Revision          string
	Roles             int64
	Permissions       int64
	Tokens            int64
	PreserveGuardian  bool
	DeactivateAccount bool
}

type AccessResult struct {
	RolesRevoked            int64
	PermissionsRevoked      int64
	TokensRevoked           int64
	GuardianAccessPreserved bool
	AccountDeactivated      bool
}

type Membership interface {
	Preview(context.Context, int64) (schoolmembership.RetirementPreview, error)
	Execute(context.Context, int64, string) (schoolmembership.Retirement, error)
}

type Workforce interface {
	Lock(context.Context, int64) error
	Preview(context.Context, int64, string) (workforce.OffboardingPreview, error)
	Execute(context.Context, int64, int64, string, string) (workforce.OffboardingCounts, error)
}

type Timetable interface {
	Preview(context.Context, int64, string) (timetable.OffboardingPreview, error)
	Execute(context.Context, int64, string, string) (timetable.OffboardingCounts, error)
}

type People interface {
	FindPerson(context.Context, int64) (peopledirectory.Person, error)
	FindPersonForMutation(context.Context, int64) (peopledirectory.Person, error)
	UnlinkTag(context.Context, int64) error
	UnlinkAccount(context.Context, int64) error
}

// Dependencies are consumer-owned ports. Authorize must resolve a school
// principal with users:delete and an acting staff record in the current tenant.
// UnitOfWork must join an ambient transaction or open a tenant transaction,
// with commit hooks. Cleanup records durable work in that same transaction;
// irreversible execution happens only after commit in the cleanup owner.
type Dependencies struct {
	UnitOfWork         func(context.Context, func(context.Context) error) error
	Authorize          func(context.Context) (Actor, error)
	Today              func() string
	FindStaff          func(context.Context, int64) (schoolmembership.Staff, error)
	Membership         Membership
	Workforce          Workforce
	Timetable          Timetable
	People             People
	PreviewAccess      func(context.Context, int64) (AccessPreview, error)
	ExecuteAccess      func(context.Context, int64, string) (AccessResult, error)
	LockSupervision    func(context.Context, int64, string) ([]int64, error)
	AppendAudit        func(context.Context, Actor, Result) error
	Cleanup            func(context.Context, int64) error
	GroupAccessChanged func(context.Context)
	Observe            func(Observation)
}

type Observation struct {
	Operation string
	Duration  time.Duration
	Result    Result
	Err       error
}

type Preview struct {
	StaffID    int64
	PersonID   int64
	AccountID  int64
	Date       string
	Revision   string
	Membership schoolmembership.RetirementPreview
	Workforce  workforce.OffboardingPreview
	Timetable  timetable.OffboardingPreview
	Access     AccessPreview
	Blocked    bool
}

type Result struct {
	StaffID    int64
	Membership schoolmembership.Retirement
	Workforce  workforce.OffboardingCounts
	Timetable  timetable.OffboardingCounts
	Access     AccessResult
}

type Workflow struct{ deps Dependencies }

func New(deps Dependencies) (*Workflow, error) {
	if deps.UnitOfWork == nil || deps.Authorize == nil || deps.Today == nil || deps.FindStaff == nil ||
		deps.Membership == nil || deps.Workforce == nil || deps.Timetable == nil || deps.People == nil ||
		deps.PreviewAccess == nil || deps.ExecuteAccess == nil || deps.LockSupervision == nil ||
		deps.AppendAudit == nil || deps.Cleanup == nil || deps.GroupAccessChanged == nil || deps.Observe == nil {
		return nil, errors.New("staff offboarding: all dependencies are required")
	}
	return &Workflow{deps: deps}, nil
}

func (w *Workflow) Preview(ctx context.Context, staffID int64) (preview Preview, err error) {
	err = w.run(ctx, "preview", staffID, func(txCtx context.Context, actor Actor) (Result, error) {
		preview, err = w.snapshot(txCtx, actor, staffID)
		return Result{}, err
	})
	if err != nil {
		return Preview{}, err
	}
	return preview, nil
}

func (w *Workflow) Execute(ctx context.Context, staffID int64, revision string) (result Result, err error) {
	if revision == "" {
		return Result{}, ErrConflict
	}
	return w.execute(ctx, staffID, revision)
}

// Offboard is the existing DELETE contract: preview and execute the current
// state in one transaction, through the same path as an explicit preview.
func (w *Workflow) Offboard(ctx context.Context, staffID int64) (Result, error) {
	return w.execute(ctx, staffID, "")
}

func (w *Workflow) execute(ctx context.Context, staffID int64, revision string) (result Result, err error) {
	err = w.run(ctx, "execute", staffID, func(txCtx context.Context, actor Actor) (Result, error) {
		preview, err := w.snapshot(txCtx, actor, staffID)
		if err != nil {
			return Result{}, err
		}
		if preview.StaffID == 0 {
			return Result{}, nil
		}
		if revision != "" && preview.Revision != revision {
			return Result{}, ErrConflict
		}
		if preview.Blocked {
			return Result{}, ErrInUse
		}
		result, err = w.mutate(txCtx, actor, preview)
		return result, err
	})
	if err != nil {
		return Result{}, err
	}
	return result, nil
}

func (w *Workflow) run(ctx context.Context, operation string, staffID int64, fn func(context.Context, Actor) (Result, error)) (err error) {
	started := time.Now()
	var result Result
	defer func() {
		if err != nil {
			result = Result{}
		}
		w.deps.Observe(Observation{Operation: operation, Duration: time.Since(started), Result: result, Err: err})
	}()
	if staffID <= 0 {
		return errors.New("staff offboarding: staff ID is required")
	}
	return w.deps.UnitOfWork(ctx, func(txCtx context.Context) error {
		actor, err := w.deps.Authorize(txCtx)
		if err != nil {
			return err
		}
		if actor.TenantID <= 0 || actor.AccountID <= 0 || actor.StaffID <= 0 {
			return ErrUnauthorized
		}
		result, err = fn(txCtx, actor)
		return err
	})
}

func (w *Workflow) snapshot(ctx context.Context, actor Actor, staffID int64) (Preview, error) {
	p := Preview{Date: w.deps.Today()}
	staff, err := w.deps.FindStaff(ctx, staffID)
	if errors.Is(err, schoolmembership.ErrStaffNotFound) {
		p.Revision = "retired"
		return p, nil
	}
	if err != nil {
		return Preview{}, err
	}
	person, err := w.deps.People.FindPerson(ctx, staff.PersonID)
	if err != nil {
		return Preview{}, err
	}
	// The unowned relationship read only chooses which account to lock. It
	// authorizes no mutation; both links are checked again after owner locks.
	if person.AccountID != nil {
		p.AccountID = *person.AccountID
		p.Access, err = w.deps.PreviewAccess(ctx, p.AccountID)
		if err != nil {
			return Preview{}, err
		}
	}
	if err := w.deps.Workforce.Lock(ctx, staffID); err != nil {
		return Preview{}, err
	}
	p.Membership, err = w.deps.Membership.Preview(ctx, staffID)
	if err != nil {
		return Preview{}, err
	}
	if p.Membership.Retirement.StaffID == 0 {
		return Preview{Revision: "retired"}, nil
	}
	if p.Membership.Retirement.PersonID != person.ID {
		return Preview{}, ErrConflict
	}
	lockedPerson, err := w.deps.People.FindPersonForMutation(ctx, person.ID)
	if err != nil {
		return Preview{}, err
	}
	if accountID(lockedPerson) != p.AccountID {
		return Preview{}, ErrConflict
	}
	p.StaffID, p.PersonID = staffID, person.ID
	supervisions, err := w.deps.LockSupervision(ctx, staffID, p.Date)
	if err != nil {
		return Preview{}, err
	}
	p.Workforce, err = w.deps.Workforce.Preview(ctx, staffID, p.Date)
	if err != nil {
		return Preview{}, err
	}
	p.Timetable, err = w.deps.Timetable.Preview(ctx, staffID, p.Date)
	if err != nil {
		return Preview{}, err
	}
	p.Blocked = len(supervisions) > 0 || p.Workforce.Blocked
	// Revisions are comparison data, not bearer credentials. Never log them.
	encoded, err := json.Marshal(struct {
		Preview         Preview
		TenantID        int64
		PersonUpdatedAt time.Time
		TagID           *string
		Supervisions    []int64
	}{p, actor.TenantID, lockedPerson.UpdatedAt, lockedPerson.TagID, supervisions})
	if err != nil {
		return Preview{}, err
	}
	p.Revision = string(encoded)
	return p, nil
}

func accountID(person peopledirectory.Person) int64 {
	if person.AccountID == nil {
		return 0
	}
	return *person.AccountID
}

func (w *Workflow) mutate(ctx context.Context, actor Actor, p Preview) (result Result, err error) {
	result.StaffID = p.StaffID
	result.Workforce, err = w.deps.Workforce.Execute(ctx, p.StaffID, actor.StaffID, p.Date, p.Workforce.Revision)
	if err != nil {
		return result, err
	}
	result.Timetable, err = w.deps.Timetable.Execute(ctx, p.StaffID, p.Date, p.Timetable.Revision)
	if err != nil {
		return result, err
	}
	result.Membership, err = w.deps.Membership.Execute(ctx, p.StaffID, p.Membership.Revision)
	if err != nil {
		return result, err
	}
	if err := w.deps.People.UnlinkTag(ctx, p.PersonID); err != nil {
		return result, err
	}
	if p.AccountID != 0 {
		result.Access, err = w.deps.ExecuteAccess(ctx, p.AccountID, p.Access.Revision)
		if err != nil {
			return result, err
		}
		if err := w.deps.People.UnlinkAccount(ctx, p.PersonID); err != nil {
			return result, err
		}
	}
	if err := w.deps.AppendAudit(ctx, actor, result); err != nil {
		return result, err
	}
	if err := w.deps.Cleanup(ctx, p.StaffID); err != nil {
		return result, err
	}
	if result.Membership.GroupAssignments > 0 || result.Workforce.Substitutions > 0 {
		w.deps.GroupAccessChanged(ctx)
	}
	return result, nil
}
