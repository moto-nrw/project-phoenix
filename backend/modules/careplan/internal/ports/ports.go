package ports

import (
	"context"
	"encoding/json"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
)

type Store interface {
	FindCareExits(context.Context, []int64) (map[int64]domain.CareExit, domain.OperationStats, error)
	UpsertCareExit(context.Context, domain.CareExit) (domain.OperationStats, error)
	DeleteCareExits(context.Context, []int64) (domain.OperationStats, error)

	ListCompanionEdges(context.Context, int64) ([]domain.CompanionEdge, domain.OperationStats, error)
	ListCompanionLinks(context.Context, []int64) (map[int64][]domain.CompanionLink, domain.OperationStats, error)
	CompanionCountsExcluding(context.Context, []int64, int64) (map[int64]int, domain.OperationStats, error)
	CompanionDaysCoveredExcluding(context.Context, []int64, int64) (map[int64]map[string]bool, domain.OperationStats, error)
	CompanionIDsForWeekday(context.Context, []int64, int) (map[int64][]int64, domain.OperationStats, error)
	CompanionWeekdays(context.Context, int64) ([]int, domain.OperationStats, error)
	CountCompanionLinks(context.Context, int64) (int, domain.OperationStats, error)
	ReplaceCompanionEdges(context.Context, int64, []domain.CompanionEdge) (domain.OperationStats, error)
	DeleteCompanionEdges(context.Context, []int64) (domain.OperationStats, error)

	FindCareDocument(context.Context, int64, int64, bool) (domain.CareDocument, bool, domain.OperationStats, error)
	ListCareDocuments(context.Context, int64, []string) ([]domain.CareDocument, domain.OperationStats, error)
	ListPendingCareDocumentCleanup(context.Context, int64) ([]domain.CareDocument, domain.OperationStats, error)
	ListDeletedCareDocuments(context.Context, int64, []string) ([]domain.CareDocument, domain.OperationStats, error)
	ListCareDocumentCleanups(context.Context, *int64) ([]domain.CareDocumentCleanup, domain.OperationStats, error)
	CreateCareDocument(context.Context, domain.CareDocument) (domain.CareDocument, domain.OperationStats, error)
	SoftDeleteCareDocument(context.Context, int64, int64) (time.Time, domain.OperationStats, error)
	MarkCareDocumentFileDeleted(context.Context, int64) (domain.OperationStats, error)
	QueueCareDocumentCleanup(context.Context, domain.CareDocumentCleanup) (domain.CareDocumentCleanup, domain.OperationStats, error)
	CompleteCareDocumentCleanup(context.Context, int64) (domain.OperationStats, error)
	CompleteCareDocumentCleanupByFilename(context.Context, string) (domain.OperationStats, error)
	ActivateCareDocumentCleanup(context.Context, string) (domain.OperationStats, error)
	ListCareExitRemovals(context.Context, []int64) ([]domain.CareExitRemoval, domain.OperationStats, error)
	ListCareExitSourceRemovals(context.Context, []int64) ([]domain.CareExitSourceRemoval, domain.OperationStats, error)
	RecordCareExitRemovals(context.Context, []domain.CareExitRemoval) (domain.OperationStats, error)
	RecordCareExitSourceRemovals(context.Context, []domain.CareExitSourceRemoval) (domain.OperationStats, error)
	DiscardCareExitRemovals(context.Context, []int64) (domain.OperationStats, error)

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
