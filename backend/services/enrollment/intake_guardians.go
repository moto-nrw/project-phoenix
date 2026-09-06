package enrollment

import (
	"context"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// IntakeGuardians owns additional guardian records on enrollment applications.
type IntakeGuardians interface {
	CreateRequestGuardian(context.Context, *capability.RequestGuardian) error
	RequestGuardians(context.Context, []int64) ([]*capability.RequestGuardian, error)
	DeleteRequestGuardians(context.Context, int64) error
}

type GuardianReader interface {
	RequestGuardians(context.Context, []int64) ([]*capability.RequestGuardian, error)
}

type DecisionGuardians interface {
	GuardianReader
	StampRequestGuardianProfile(context.Context, int64, int64) error
}

func createIntakeGuardian(ctx context.Context, owner IntakeGuardians, guardian *capability.RequestGuardian) error {
	return owner.CreateRequestGuardian(ctx, guardian)
}

func listIntakeGuardians(ctx context.Context, owner GuardianReader, requestID int64) ([]*capability.RequestGuardian, error) {
	return listIntakeGuardiansForRequests(ctx, owner, []int64{requestID})
}

func listIntakeGuardiansForRequests(ctx context.Context, owner GuardianReader, requestIDs []int64) ([]*capability.RequestGuardian, error) {
	return owner.RequestGuardians(ctx, requestIDs)
}
