package schoolmembership

import (
	"context"
	"errors"
	"time"
)

var ErrOffboardingConflict = errors.New("staff membership changed since offboarding preview")

type RetirementPreview struct {
	Retirement Retirement
	Revision   string
}

// Retirement reports only Membership-owned changes. Staff and teacher rows
// remain as tombstones so historical references continue to resolve.
type Retirement struct {
	StaffID          int64
	PersonID         int64
	TeacherID        int64
	GroupAssignments int64
	ClassAssignments int64
}

// Offboarding is the Membership command used by the staff lifecycle workflow.
// It does not revoke identity access or mutate other owners' assignments.
type Offboarding struct {
	preview   func(context.Context, int64) (RetirementPreview, error)
	execute   func(context.Context, int64, string) (Retirement, error)
	eraseNext func(context.Context, time.Time, int64) (RetiredStaffErasure, bool, error)
	reinstate Reinstatement
}

// Reinstatement binds the Wiederaufnahme of ADR 0021, the way back from a
// retirement: the retired staff membership and caregiver profile come back
// under their own IDs, so every record keyed by them belongs to the resumed
// employment again (#3401).
type Reinstatement struct {
	Staff   func(context.Context, int64) (Staff, error)
	Teacher func(context.Context, int64) (Teacher, error)
}

// NewOffboarding binds the retirement queries and commands. eraseNext is the
// Membership half of the automatic deletion of retired staff (#3963), and
// reinstate the Wiederaufnahme; a graph without either may leave it zero.
func NewOffboarding(preview func(context.Context, int64) (RetirementPreview, error), execute func(context.Context, int64, string) (Retirement, error), eraseNext func(context.Context, time.Time, int64) (RetiredStaffErasure, bool, error), reinstate Reinstatement) *Offboarding {
	if preview == nil || execute == nil {
		panic("school membership: offboarding queries and commands are required")
	}
	return &Offboarding{preview: preview, execute: execute, eraseNext: eraseNext, reinstate: reinstate}
}

// ReinstateStaff lifts the soft delete of a retired membership. A personnel
// number another live staff member holds by now is cleared instead of failing
// the reinstatement (ADR 0021, decision 6). ErrStaffNotFound when the
// membership is live, unknown, or already erased by the automatic deletion of
// staff.
func (o *Offboarding) ReinstateStaff(ctx context.Context, staffID int64) (Staff, error) {
	if staffID <= 0 {
		return Staff{}, invalid("staff ID is required")
	}
	if o.reinstate.Staff == nil {
		return Staff{}, errReinstatementUnbound
	}
	return o.reinstate.Staff(ctx, staffID)
}

// ReinstateTeacher revives a retired caregiver profile of a live membership
// instead of growing a second one next to it. ErrTeacherNotFound when the
// profile is live or unknown.
func (o *Offboarding) ReinstateTeacher(ctx context.Context, teacherID int64) (Teacher, error) {
	if teacherID <= 0 {
		return Teacher{}, invalid("teacher ID is required")
	}
	if o.reinstate.Teacher == nil {
		return Teacher{}, errReinstatementUnbound
	}
	return o.reinstate.Teacher(ctx, teacherID)
}

var errReinstatementUnbound = errors.New("school membership: the offboarding graph has no reinstatement bound")

// Preview describes the Membership changes and a revision of the locked facts.
// Inside a workflow UnitOfWork the locks remain held until its completion.
func (o *Offboarding) Preview(ctx context.Context, staffID int64) (RetirementPreview, error) {
	if staffID <= 0 {
		return RetirementPreview{}, invalid("staff ID is required")
	}
	return o.preview(ctx, staffID)
}

// Execute rejects drift before its first mutation. Repeating a completed
// retirement is a no-op, and all writes join the caller's UnitOfWork.
func (o *Offboarding) Execute(ctx context.Context, staffID int64, revision string) (Retirement, error) {
	if staffID <= 0 || revision == "" {
		return Retirement{}, invalid("staff ID and preview revision are required")
	}
	return o.execute(ctx, staffID, revision)
}

// Retire previews and executes the current state through the same command.
func (o *Offboarding) Retire(ctx context.Context, staffID int64) (Retirement, error) {
	preview, err := o.Preview(ctx, staffID)
	if err != nil {
		return Retirement{}, err
	}
	return o.Execute(ctx, staffID, preview.Revision)
}
