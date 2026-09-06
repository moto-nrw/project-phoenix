package enrollment

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// InsertOwnerChildForTest persists a service fixture and its generated metadata.
func InsertOwnerChildForTest(ctx context.Context, owner *capability.Module, child *enrollmentModels.RequestChild) error {
	return createIntakeChild(ctx, owner, child)
}

// ReadOwnerRequestChildrenForTest loads service-shaped child fixtures.
func ReadOwnerRequestChildrenForTest(ctx context.Context, owner *capability.Module, requestID int64) ([]*enrollmentModels.RequestChild, error) {
	return listIntakeChildren(ctx, owner, requestID, false)
}

// ReadOwnerChildForTest loads a service-shaped fixture through the owner query.
func ReadOwnerChildForTest(ctx context.Context, owner *capability.Module, id int64) (*enrollmentModels.RequestChild, error) {
	return offeringChildByID(ctx, owner, id)
}

// UpdateOwnerChildForTest writes service-shaped fixture data through the owner.
func UpdateOwnerChildForTest(ctx context.Context, owner *capability.Module, child *enrollmentModels.RequestChild) error {
	return updateIntakeChild(ctx, owner, child)
}

// InsertOwnerRequestForTest persists a service fixture and its generated metadata.
func InsertOwnerRequestForTest(ctx context.Context, owner *capability.Module, request *enrollmentModels.Request) error {
	return createIntakeRequest(ctx, owner, request)
}

// ReadOwnerRequestForTest loads a service-shaped fixture through the owner query.
func ReadOwnerRequestForTest(ctx context.Context, owner *capability.Module, id int64) (*enrollmentModels.Request, error) {
	return intakeRequestByID(ctx, owner, id, false)
}

// UpdateOwnerRequestGuardianForTest writes a service fixture through the owner.
func UpdateOwnerRequestGuardianForTest(ctx context.Context, owner *capability.Module, request *enrollmentModels.Request, includeEmail bool) error {
	value, err := intakeRequestInput(request)
	if err != nil {
		return err
	}
	return owner.UpdateRequestGuardian(ctx, value, includeEmail)
}

// InsertOwnerPhaseForTest persists a service fixture and returns generated metadata.
func InsertOwnerPhaseForTest(ctx context.Context, owner *capability.Module, phase *capability.Phase) error {
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
