package enrollment

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// InsertOwnerChildForTest persists a service fixture and its generated metadata.
func InsertOwnerChildForTest(ctx context.Context, owner ChildCreator, child *RequestChild) error {
	return createIntakeChild(ctx, owner, child)
}

// ReadOwnerRequestChildrenForTest loads service-shaped child fixtures.
func ReadOwnerRequestChildrenForTest(ctx context.Context, owner RequestChildrenReader, requestID int64) ([]*RequestChild, error) {
	return listIntakeChildren(ctx, owner, requestID, false)
}

// ReadOwnerChildForTest loads a service-shaped fixture through the owner query.
func ReadOwnerChildForTest(ctx context.Context, owner ChildIDReader, id int64) (*RequestChild, error) {
	value, err := owner.ChildByID(ctx, id)
	if err != nil {
		return nil, err
	}
	return intakeChildValue(value)
}

// UpdateOwnerChildForTest writes service-shaped fixture data through the owner.
func UpdateOwnerChildForTest(ctx context.Context, owner IntakeChildren, child *RequestChild) error {
	return updateIntakeChild(ctx, owner, child)
}

// InsertOwnerRequestForTest persists a service fixture and its generated metadata.
func InsertOwnerRequestForTest(ctx context.Context, owner RequestCreator, request *enrollmentModels.Request) error {
	return createIntakeRequest(ctx, owner, request)
}

// ReadOwnerRequestForTest loads a service-shaped fixture through the owner query.
func ReadOwnerRequestForTest(ctx context.Context, owner RequestIDReader, id int64) (*enrollmentModels.Request, error) {
	return intakeRequestByID(ctx, owner, id, false)
}

// UpdateOwnerRequestGuardianForTest writes a service fixture through the owner.
func UpdateOwnerRequestGuardianForTest(ctx context.Context, owner interface {
	UpdateRequestGuardian(context.Context, *capability.Request, bool) error
}, request *enrollmentModels.Request, includeEmail bool) error {
	value, err := intakeRequestInput(request)
	if err != nil {
		return err
	}
	return owner.UpdateRequestGuardian(ctx, value, includeEmail)
}

// InsertOwnerPhaseForTest persists a service fixture and returns generated metadata.
func InsertOwnerPhaseForTest(ctx context.Context, owner interface {
	InsertPhase(context.Context, *capability.Phase) error
}, phase *capability.Phase) error {
	return owner.InsertPhase(ctx, phase)
}

// OwnerPhaseForTest copies an owner phase for isolated fixture updates.
func OwnerPhaseForTest(phase *capability.Phase) *capability.Phase {
	if phase == nil {
		return nil
	}
	value := *phase
	return &value
}

// WriteSelectionDateForTest exposes the clamp shared by the read and write
// paths. The parity test supplies their common clock snapshot (#2185).
func WriteSelectionDateForTest(phase *capability.Phase, today timezone.Date) timezone.Date {
	return offeringSelectionDateOn(phase, today)
}

// ChangeRequestApplierForTest applies change requests through the owner flow
// behind a decision service built by NewDecisionService, the way the root
// binds the applier over the same flow.
func ChangeRequestApplierForTest(svc DecisionService, bookings CareBookingGates) ChangeRequestDecisionApplier {
	contract, _ := svc.(decisionContract)
	owner, _ := contract.owner.(capability.ApprovedChildChanges)
	return NewChangeRequestDecisionApplier(owner, bookings)
}
