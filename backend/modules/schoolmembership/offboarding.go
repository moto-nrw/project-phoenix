package schoolmembership

import (
	"context"
	"errors"
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
	preview func(context.Context, int64) (RetirementPreview, error)
	execute func(context.Context, int64, string) (Retirement, error)
}

func NewOffboarding(preview func(context.Context, int64) (RetirementPreview, error), execute func(context.Context, int64, string) (Retirement, error)) *Offboarding {
	if preview == nil || execute == nil {
		panic("school membership: offboarding queries and commands are required")
	}
	return &Offboarding{preview: preview, execute: execute}
}

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
