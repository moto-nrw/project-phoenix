package enrollmenthttp

import (
	"context"
	"encoding/json"

	"github.com/moto-ogs/project-phoenix/tenant"

	capability "github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// The enrollment flow suites of the external test package (#3565) seed and
// read the owner's rows through the helpers below, each in a transaction of
// its own.

// InsertOwnerChildForTest persists a child fixture through the owner and
// copies the generated metadata back.
func InsertOwnerChildForTest(ctx context.Context, owner interface {
	InsertChild(context.Context, *capability.RequestChild) error
}, child *capability.RequestChild) error {
	return fixtureOwnerCommand(ctx, func(txCtx context.Context) error { return owner.InsertChild(txCtx, child) })
}

// ReadOwnerRequestChildrenForTest loads the children of a request.
func ReadOwnerRequestChildrenForTest(ctx context.Context, owner interface {
	ChildrenForRequest(context.Context, int64, bool) ([]*capability.RequestChild, error)
}, requestID int64) ([]*capability.RequestChild, error) {
	return fixtureOwnerQuery(ctx, func(txCtx context.Context) ([]*capability.RequestChild, error) {
		return owner.ChildrenForRequest(txCtx, requestID, false)
	})
}

// ReadOwnerChildForTest loads a child through the owner query.
func ReadOwnerChildForTest(ctx context.Context, owner interface {
	ChildByID(context.Context, int64) (*capability.RequestChild, error)
}, id int64) (*capability.RequestChild, error) {
	return fixtureOwnerQuery(ctx, func(txCtx context.Context) (*capability.RequestChild, error) { return owner.ChildByID(txCtx, id) })
}

// UpdateOwnerChildForTest writes child data through the owner.
func UpdateOwnerChildForTest(ctx context.Context, owner interface {
	UpdateChildData(context.Context, *capability.RequestChild) error
}, child *capability.RequestChild) error {
	return fixtureOwnerCommand(ctx, func(txCtx context.Context) error { return owner.UpdateChildData(txCtx, child) })
}

// InsertOwnerRequestForTest persists a request fixture through the owner and
// copies the generated metadata back.
func InsertOwnerRequestForTest(ctx context.Context, owner interface {
	InsertRequest(context.Context, *capability.Request) error
}, request *capability.Request) error {
	return fixtureOwnerCommand(ctx, func(txCtx context.Context) error { return owner.InsertRequest(txCtx, request) })
}

// ReadOwnerRequestForTest loads a request through the owner query.
func ReadOwnerRequestForTest(ctx context.Context, owner interface {
	RequestByID(context.Context, int64, bool) (*capability.Request, error)
}, id int64) (*capability.Request, error) {
	return fixtureOwnerQuery(ctx, func(txCtx context.Context) (*capability.Request, error) { return owner.RequestByID(txCtx, id, false) })
}

// UpdateOwnerRequestGuardianForTest writes a request fixture's guardian
// fields through the owner.
func UpdateOwnerRequestGuardianForTest(ctx context.Context, owner interface {
	UpdateRequestGuardian(context.Context, *capability.Request, bool) error
}, request *capability.Request, includeEmail bool) error {
	return fixtureOwnerCommand(ctx, func(txCtx context.Context) error { return owner.UpdateRequestGuardian(txCtx, request, includeEmail) })
}

// InsertOwnerPhaseForTest persists a phase fixture and returns generated
// metadata.
func InsertOwnerPhaseForTest(ctx context.Context, owner interface {
	InsertPhase(context.Context, *capability.Phase) error
}, phase *capability.Phase) error {
	return fixtureOwnerCommand(ctx, func(txCtx context.Context) error { return owner.InsertPhase(txCtx, phase) })
}

// OwnerPhaseForTest copies an owner phase for isolated fixture updates.
func OwnerPhaseForTest(phase *capability.Phase) *capability.Phase {
	if phase == nil {
		return nil
	}
	value := *phase
	return &value
}

// WriteSelectionDateForTest is the clamp the offering reads and writes
// share: a day before the service window reads its first day, a day after
// it its last day (#2185).
func WriteSelectionDateForTest(phase *capability.Phase, today calendar.Date) calendar.Date {
	if phase == nil {
		return today
	}
	if today.Before(calendar.Date(phase.ServiceStartDate)) {
		return calendar.Date(phase.ServiceStartDate)
	}
	if today.After(calendar.Date(phase.ServiceEndDate)) {
		return calendar.Date(phase.ServiceEndDate)
	}
	return today
}

// SyncApprovedChildDataInput carries a confirmed change request onto the
// student an approval created.
type SyncApprovedChildDataInput struct {
	RequestID                int64
	ChildID                  int64
	ActorAccountID           int64
	ReplaceTargetedData      bool
	PreviousSnapshot         map[string]any
	PreviousRequestGuardians []*capability.RequestGuardian
	ChildDataChanged         bool
}

// ChangeRequestDecisionApplier is the admin correction's sync of an
// approved child onto its student.
type ChangeRequestDecisionApplier interface {
	SyncApprovedChildData(ctx context.Context, input SyncApprovedChildDataInput) (*capability.RequestChild, error)
}

// ChangeRequestApplierForTest syncs approved children through the owner's
// decision flow, the way the change requests do.
func ChangeRequestApplierForTest(owner capability.ApprovedChildChanges) ChangeRequestDecisionApplier {
	return changeRequestApplier{owner: owner}
}

type changeRequestApplier struct {
	owner capability.ApprovedChildChanges
}

func (a changeRequestApplier) SyncApprovedChildData(ctx context.Context, input SyncApprovedChildDataInput) (*capability.RequestChild, error) {
	snapshot, err := json.Marshal(input.PreviousSnapshot)
	if err != nil {
		return nil, err
	}
	return fixtureOwnerQuery(ctx, func(txCtx context.Context) (*capability.RequestChild, error) {
		return a.owner.SyncApprovedChildData(txCtx, capability.ApprovedChildSync{
			RequestID: input.RequestID, ChildID: input.ChildID, ActorAccountID: input.ActorAccountID,
			ReplaceTargetedData: input.ReplaceTargetedData, PreviousSnapshot: snapshot,
			PreviousRequestGuardians: input.PreviousRequestGuardians, ChildDataChanged: input.ChildDataChanged,
		})
	})
}

// These are fixture roots, not owner persistence fallbacks. Tests that inspect
// the ambient-only contract call the native owner directly.
func fixtureOwnerCommand(ctx context.Context, call func(context.Context) error) error {
	return tenant.NewTransactionRunner().RunInTx(ctx, call)
}
func fixtureOwnerQuery[T any](ctx context.Context, call func(context.Context) (T, error)) (value T, err error) {
	err = tenant.NewTransactionRunner().RunInTx(ctx, func(txCtx context.Context) error {
		value, err = call(txCtx)
		return err
	})
	return value, err
}
