package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

func (e Engine) FindCareExits(ctx context.Context, studentIDs []int64) (map[int64]careplan.CareExit, error) {
	values, err := e.service.FindCareExits(ctx, studentIDs)
	return values, MapError(err)
}

func (e Engine) UpsertCareExit(ctx context.Context, value careplan.CareExit) error {
	return MapError(e.withinTenant(ctx, func(txCtx context.Context) error { return e.service.UpsertCareExit(txCtx, value) }))
}

func (e Engine) DeleteCareExits(ctx context.Context, studentIDs []int64) error {
	return MapError(e.withinTenant(ctx, func(txCtx context.Context) error { return e.service.DeleteCareExits(txCtx, studentIDs) }))
}

func (e Engine) ListCompanionEdges(ctx context.Context, studentID int64) ([]careplan.CompanionEdge, error) {
	values, err := e.service.ListCompanionEdges(ctx, studentID)
	return values, MapError(err)
}

func (e Engine) ListCompanionLinks(ctx context.Context, studentIDs []int64) (map[int64][]careplan.CompanionLink, error) {
	links, err := e.service.ListCompanionLinks(ctx, studentIDs)
	if err != nil {
		return nil, MapError(err)
	}
	companionIDs := make([]int64, 0)
	seen := make(map[int64]bool)
	for _, values := range links {
		for _, link := range values {
			if !seen[link.CompanionStudentID] {
				seen[link.CompanionStudentID] = true
				companionIDs = append(companionIDs, link.CompanionStudentID)
			}
		}
	}
	if len(companionIDs) == 0 {
		return links, nil
	}
	names, err := e.people.ListStudentNamesByID(ctx, companionIDs)
	if err != nil {
		return nil, err
	}
	nameByStudent := make(map[int64][2]string, len(names))
	for _, name := range names {
		nameByStudent[name.StudentID] = [2]string{name.FirstName, name.LastName}
	}
	for studentID, values := range links {
		for i := range values {
			name := nameByStudent[values[i].CompanionStudentID]
			values[i].FirstName, values[i].LastName = name[0], name[1]
		}
		links[studentID] = values
	}
	return links, nil
}

func (e Engine) CompanionCountsExcluding(ctx context.Context, studentIDs []int64, excludeID int64) (map[int64]int, error) {
	values, err := e.service.CompanionCountsExcluding(ctx, studentIDs, excludeID)
	return values, MapError(err)
}

func (e Engine) CompanionDaysCoveredExcluding(ctx context.Context, studentIDs []int64, excludeID int64) (map[int64]map[string]bool, error) {
	values, err := e.service.CompanionDaysCoveredExcluding(ctx, studentIDs, excludeID)
	return values, MapError(err)
}

func (e Engine) CompanionIDsForWeekday(ctx context.Context, studentIDs []int64, weekday int) (map[int64][]int64, error) {
	values, err := e.service.CompanionIDsForWeekday(ctx, studentIDs, weekday)
	return values, MapError(err)
}

func (e Engine) CompanionWeekdays(ctx context.Context, studentID int64) ([]int, error) {
	values, err := e.service.CompanionWeekdays(ctx, studentID)
	return values, MapError(err)
}

func (e Engine) CountCompanionLinks(ctx context.Context, studentID int64) (int, error) {
	value, err := e.service.CountCompanionLinks(ctx, studentID)
	return value, MapError(err)
}

func (e Engine) ReplaceCompanionEdges(ctx context.Context, studentID int64, edges []careplan.CompanionEdge) error {
	return MapError(e.withinTenant(ctx, func(txCtx context.Context) error { return e.service.ReplaceCompanionEdges(txCtx, studentID, edges) }))
}

func (e Engine) DeleteCompanionEdges(ctx context.Context, edgeIDs []int64) error {
	return MapError(e.withinTenant(ctx, func(txCtx context.Context) error { return e.service.DeleteCompanionEdges(txCtx, edgeIDs) }))
}

func (e Engine) FindCareDocument(ctx context.Context, studentID, documentID int64, includeDeleted bool) (careplan.CareDocument, error) {
	value, err := e.service.FindCareDocument(ctx, studentID, documentID, includeDeleted)
	return value, MapError(err)
}

func (e Engine) ListCareDocuments(ctx context.Context, studentID int64, categories []string) ([]careplan.CareDocument, error) {
	values, err := e.service.ListCareDocuments(ctx, studentID, categories)
	return values, MapError(err)
}

func (e Engine) ListPendingCareDocumentCleanup(ctx context.Context, studentID int64) ([]careplan.CareDocument, error) {
	values, err := e.service.ListPendingCareDocumentCleanup(ctx, studentID)
	return values, MapError(err)
}

func (e Engine) ListDeletedCareDocuments(ctx context.Context, studentID int64, categories []string) ([]careplan.CareDocument, error) {
	values, err := e.service.ListDeletedCareDocuments(ctx, studentID, categories)
	return values, MapError(err)
}

func (e Engine) ListCareDocumentCleanups(ctx context.Context, studentID *int64) ([]careplan.CareDocumentCleanup, error) {
	values, err := e.service.ListCareDocumentCleanups(ctx, studentID)
	return values, MapError(err)
}

func (e Engine) CreateCareDocument(ctx context.Context, value careplan.CareDocument) (result careplan.CareDocument, err error) {
	err = e.withinTenant(ctx, func(txCtx context.Context) error {
		result, err = e.service.CreateCareDocument(txCtx, value)
		return err
	})
	return result, MapError(err)
}

func (e Engine) SoftDeleteCareDocument(ctx context.Context, documentID, deletedBy int64) (result time.Time, err error) {
	err = e.withinTenant(ctx, func(txCtx context.Context) error {
		result, err = e.service.SoftDeleteCareDocument(txCtx, documentID, deletedBy)
		return err
	})
	return result, MapError(err)
}

func (e Engine) MarkCareDocumentFileDeleted(ctx context.Context, documentID int64) error {
	return MapError(e.withinTenant(ctx, func(txCtx context.Context) error { return e.service.MarkCareDocumentFileDeleted(txCtx, documentID) }))
}

func (e Engine) QueueCareDocumentCleanup(ctx context.Context, value careplan.CareDocumentCleanup) (result careplan.CareDocumentCleanup, err error) {
	err = e.withinTenant(ctx, func(txCtx context.Context) error {
		result, err = e.service.QueueCareDocumentCleanup(txCtx, value)
		return err
	})
	return result, MapError(err)
}

func (e Engine) CompleteCareDocumentCleanup(ctx context.Context, cleanupID int64) error {
	return MapError(e.withinTenant(ctx, func(txCtx context.Context) error { return e.service.CompleteCareDocumentCleanup(txCtx, cleanupID) }))
}

func (e Engine) CompleteCareDocumentCleanupByFilename(ctx context.Context, filename string) error {
	return MapError(e.withinTenant(ctx, func(txCtx context.Context) error {
		return e.service.CompleteCareDocumentCleanupByFilename(txCtx, filename)
	}))
}

func (e Engine) ActivateCareDocumentCleanup(ctx context.Context, filename string) error {
	return MapError(e.withinTenant(ctx, func(txCtx context.Context) error { return e.service.ActivateCareDocumentCleanup(txCtx, filename) }))
}

func (e Engine) ListCareExitRemovals(ctx context.Context, studentIDs []int64) ([]careplan.CareExitRemoval, error) {
	values, err := e.service.ListCareExitRemovals(ctx, studentIDs)
	return values, MapError(err)
}

func (e Engine) ListCareExitSourceRemovals(ctx context.Context, studentIDs []int64) ([]careplan.CareExitSourceRemoval, error) {
	values, err := e.service.ListCareExitSourceRemovals(ctx, studentIDs)
	return values, MapError(err)
}

func (e Engine) RecordCareExitRemovals(ctx context.Context, values []careplan.CareExitRemoval) error {
	return MapError(e.withinTenant(ctx, func(txCtx context.Context) error {
		return e.service.RecordCareExitRemovals(txCtx, values)
	}))
}

func (e Engine) RecordCareExitSourceRemovals(ctx context.Context, values []careplan.CareExitSourceRemoval) error {
	return MapError(e.withinTenant(ctx, func(txCtx context.Context) error {
		return e.service.RecordCareExitSourceRemovals(txCtx, values)
	}))
}

func (e Engine) DiscardCareExitRemovals(ctx context.Context, studentIDs []int64) error {
	return MapError(e.withinTenant(ctx, func(txCtx context.Context) error {
		return e.service.DiscardCareExitRemovals(txCtx, studentIDs)
	}))
}
