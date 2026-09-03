package ports

import (
	"context"
	"encoding/json"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
)

type Store interface {
	FindCareOffering(context.Context, int64) (domain.CareOffering, bool, domain.OperationStats, error)
	ListCareOfferings(context.Context, domain.CareOfferingFilter) ([]domain.CareOffering, domain.OperationStats, error)
	CountCareOfferingsByPhase(context.Context, int64) (int, domain.OperationStats, error)
	CreateCareOffering(context.Context, domain.CareOfferingFields) (domain.CareOffering, domain.OperationStats, error)
	UpdateCareOffering(context.Context, int64, domain.CareOfferingFields) (domain.CareOffering, domain.OperationStats, error)
	DeleteCareOffering(context.Context, int64) (domain.OperationStats, error)
	ReplaceAutoAddTriggers(context.Context, int64, []int64) (domain.OperationStats, error)

	FindOfferingChange(context.Context, int64, bool) (domain.OfferingChangeRequest, bool, domain.OperationStats, error)
	ListOfferingChanges(context.Context, domain.OfferingChangeFilter) ([]domain.OfferingChangeRequest, domain.OperationStats, error)
	CreateOfferingChange(context.Context, domain.OfferingChangeRequest) (domain.OfferingChangeRequest, domain.OperationStats, error)
	UpdateOfferingChangeEffectiveFrom(context.Context, int64, string) (domain.OperationStats, error)
	UpdateApprovedCompleteWithdrawal(context.Context, int64, bool) (domain.OperationStats, error)
	UpdatePendingOfferingChange(context.Context, domain.UpdatePendingOfferingChange) (domain.OperationStats, error)
	DecideOfferingChange(context.Context, domain.DecideOfferingChange) (domain.OperationStats, error)
	UpdateOfferingChangeSnapshot(context.Context, int64, json.RawMessage) (domain.OperationStats, error)
	ClosePendingOfferingChanges(context.Context, []int64, string, *int64, time.Time) (domain.OperationStats, error)
}

type Observation struct {
	Operation string
	Duration  time.Duration
	Stats     domain.OperationStats
	Err       error
}

type Observer func(Observation)
