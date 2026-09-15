package application

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
)

type OfferingReviewDependencies struct {
	Requests   careplan.OfferingChangeRequestQuery
	Catalog    *Service
	People     ports.OfferingReviewDirectory
	Enrollment ports.OfferingReviewEnrollment
	Courses    ports.OfferingReviewCourses
	Scope      ports.ReviewScopeResolver
	Today      func() careplan.Date
	Logger     *slog.Logger
}

type OfferingReviews struct{ OfferingReviewDependencies }

func NewOfferingReviews(deps OfferingReviewDependencies) *OfferingReviews {
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	return &OfferingReviews{OfferingReviewDependencies: deps}
}

func (s *OfferingReviews) reviewDay(filter careplan.RequestQueueFilter) careplan.Date {
	day, err := careplan.ParseDate(filter.UrgentDate)
	if err != nil {
		return s.Today()
	}
	return day
}

func (s *OfferingReviews) rows(ctx context.Context, filter careplan.RequestQueueFilter, history bool) ([]careplan.OfferingChangeRequest, *careplan.RequestCursor, error) {
	ids := filter.StudentIDs
	if strings.TrimSpace(filter.Search) != "" {
		var err error
		ids, err = s.People.SearchStudentIDs(ctx, filter.Search, ids)
		if err != nil {
			return nil, nil, err
		}
	}
	probe := probeLimit(filter)
	query := careplan.OfferingChangeFilter{
		StudentIDs: ids, StudentID: filter.StudentID, Statuses: []string{"pending"},
		UrgentOnly: filter.UrgentOnly, UrgentDate: s.reviewDay(filter).String(),
		BeforeInstant: filter.BeforeInstant, BeforeID: filter.BeforeID, Limit: probe.Limit,
	}
	if history {
		query.Statuses, query.Order = []string{"approved", "rejected", "withdrawn"}, "updated"
	}
	rows, err := s.Requests.ListOfferingChanges(ctx, query)
	if err != nil {
		return nil, nil, err
	}
	rows, cursor := nextCursor(rows, filter.Limit, func(row careplan.OfferingChangeRequest) (time.Time, int64) {
		if history {
			return row.UpdatedAt, row.ID
		}
		return row.CreatedAt, row.ID
	})
	return rows, cursor, nil
}

func (s *OfferingReviews) students(ctx context.Context, rows []careplan.OfferingChangeRequest) (map[int64]ports.ReviewStudent, error) {
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.StudentID)
	}
	return s.People.FindStudents(ctx, ids)
}

func (s *OfferingReviews) ListPending(ctx context.Context, filter careplan.RequestQueueFilter) ([]*careplan.OfferingReviewItem, *careplan.RequestCursor, error) {
	rows, cursor, err := s.rows(ctx, filter, false)
	if err != nil {
		return nil, nil, fmt.Errorf("offering change: list pending: %w", err)
	}
	students, err := s.students(ctx, rows)
	if err != nil {
		return nil, nil, fmt.Errorf("offering change: load students: %w", err)
	}
	scope, err := s.Scope(ctx)
	if err != nil {
		return nil, nil, err
	}
	today := s.reviewDay(filter)
	visible := make([]careplan.OfferingChangeRequest, 0, len(rows))
	personIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		student, ok := students[row.StudentID]
		if !ok || !scope.Allows(&student) || student.Alumnus || student.CareEndedOn(today) {
			continue
		}
		visible = append(visible, row)
		if student.PersonID > 0 {
			personIDs = append(personIDs, student.PersonID)
		}
	}
	names, err := s.People.PersonNames(ctx, personIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("offering change: load student persons: %w", err)
	}
	reviews, diffErr := s.pendingReviews(ctx, visible, today)
	if diffErr != nil {
		s.Logger.Warn("offering change: preload pending diffs failed", slog.String("error", diffErr.Error()))
	}
	result := make([]*careplan.OfferingReviewItem, 0, len(visible))
	for i := range visible {
		row := visible[i]
		item := &careplan.OfferingReviewItem{Request: &row, RequestedEffectiveFrom: row.EffectiveFrom}
		item.Request.EffectiveFrom = offeringAppliedDate(careplan.Date(row.EffectiveFrom), today, careplan.Date("")).String()
		name := names[students[row.StudentID].PersonID]
		item.StudentName = strings.TrimSpace(name.FirstName + " " + name.LastName)
		if review := reviews[row.ID]; review != nil {
			item.Request.EffectiveFrom = review.applied.String()
			item.EarliestEffectiveFrom, item.LatestEffectiveFrom = review.earliest.String(), review.latest.String()
			if diffErr == nil {
				item.Diff, item.Unchanged, item.FullWithdrawal = review.diff, review.unchanged, review.withdrawal
			}
		}
		result = append(result, item)
	}
	return result, cursor, nil
}

// PendingCount deliberately excludes catalog reads and selection materialization.
func (s *OfferingReviews) PendingCount(ctx context.Context, today careplan.Date) (int, error) {
	rows, err := s.Requests.ListOfferingChanges(ctx, careplan.OfferingChangeFilter{Statuses: []string{"pending"}})
	if err != nil {
		return 0, fmt.Errorf("offering change: list pending for count: %w", err)
	}
	students, err := s.students(ctx, rows)
	if err != nil {
		return 0, fmt.Errorf("offering change: load students for count: %w", err)
	}
	scope, err := s.Scope(ctx)
	if err != nil {
		return 0, err
	}
	count := 0
	for _, row := range rows {
		student, ok := students[row.StudentID]
		if ok && scope.Allows(&student) && !student.Alumnus && !student.CareEndedOn(today) {
			count++
		}
	}
	return count, nil
}

func (s *OfferingReviews) ListHistory(ctx context.Context, filter careplan.RequestQueueFilter) ([]*careplan.OfferingHistoryItem, *careplan.RequestCursor, error) {
	rows, cursor, err := s.rows(ctx, filter, true)
	if err != nil {
		return nil, nil, fmt.Errorf("offering change: list decided: %w", err)
	}
	if len(rows) == 0 {
		return []*careplan.OfferingHistoryItem{}, cursor, nil
	}
	students, err := s.students(ctx, rows)
	if err != nil {
		return nil, nil, fmt.Errorf("offering change: load students for history: %w", err)
	}
	personIDs := make([]int64, 0, len(students))
	for _, student := range students {
		personIDs = append(personIDs, student.PersonID)
	}
	names, err := s.People.PersonNames(ctx, personIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("offering change: load student persons for history: %w", err)
	}
	reviewerIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		if row.ReviewedBy != nil && *row.ReviewedBy > 0 && !slices.Contains(reviewerIDs, *row.ReviewedBy) {
			reviewerIDs = append(reviewerIDs, *row.ReviewedBy)
		}
	}
	reviewers, err := s.People.ReviewerNames(ctx, reviewerIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("offering change: load reviewers for history: %w", err)
	}
	scope, err := s.Scope(ctx)
	if err != nil {
		return nil, nil, err
	}
	result := make([]*careplan.OfferingHistoryItem, 0, len(rows))
	for i := range rows {
		row := &rows[i]
		student, ok := students[row.StudentID]
		if !ok || !scope.Allows(&student) || student.Alumnus {
			continue
		}
		name := names[student.PersonID]
		item := &careplan.OfferingHistoryItem{Request: row, StudentName: strings.TrimSpace(name.FirstName + " " + name.LastName), ReviewerName: reviewerDisplayName(reviewers, row.ReviewedBy)}
		if len(row.DecisionSnapshot) > 0 && strings.TrimSpace(string(row.DecisionSnapshot)) != "null" {
			item.Diff, err = careplan.ParseOfferingReviewDecisionDiff(row.DecisionSnapshot)
		} else {
			item.Requested, err = s.requestedItems(ctx, row.Payload)
		}
		if err != nil {
			s.Logger.Warn("offering change: resolve requested offerings for history failed",
				slog.Int64("request_id", row.ID),
				slog.String("error", err.Error()),
			)
		}
		result = append(result, item)
	}
	return result, cursor, nil
}

func (s *OfferingReviews) requestedItems(ctx context.Context, payload []byte) ([]careplan.OfferingRequestedItem, error) {
	selected, err := careplan.ParseOfferingReviewSelections(payload)
	if err != nil {
		return nil, err
	}
	if len(selected) == 0 {
		return []careplan.OfferingRequestedItem{}, nil
	}
	ids := make([]int64, 0, len(selected))
	for _, item := range selected {
		ids = append(ids, item.OfferingID)
	}
	offerings, err := s.Catalog.ListCareOfferings(ctx, domain.CareOfferingFilter{IDs: ids})
	if err != nil {
		return nil, fmt.Errorf("list requested offerings: %w", err)
	}
	byID := make(map[int64]domain.CareOffering, len(offerings))
	for _, offering := range offerings {
		byID[offering.ID] = offering
	}
	result := make([]careplan.OfferingRequestedItem, 0, len(selected))
	for _, item := range selected {
		result = append(result, careplan.OfferingRequestedItem{OfferingID: item.OfferingID, Name: offeringLabel(item.OfferingID, byID), Days: slices.Clone(item.SelectedDays)})
	}
	sort.SliceStable(result, func(i, j int) bool {
		left, right := byID[result[i].OfferingID].SortOrder, byID[result[j].OfferingID].SortOrder
		if left == right {
			return result[i].Name < result[j].Name
		}
		return left < right
	})
	return result, nil
}

func offeringLabel(id int64, catalog map[int64]domain.CareOffering) string {
	if name := catalog[id].Name; name != "" {
		return name
	}
	return fmt.Sprintf("Angebot %d", id)
}

func offeringAppliedDate(requested, today, phaseStart careplan.Date) careplan.Date {
	if requested.Before(today) {
		requested = today
	}
	if requested.Before(phaseStart) {
		requested = phaseStart
	}
	return requested
}
