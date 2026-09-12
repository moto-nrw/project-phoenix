package application

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
)

type MasterDataReviews struct {
	requests careplan.StudentDataRequestQuery
	people   ports.MasterDataDirectory
	scope    ports.ReviewScopeResolver
	today    func() careplan.Date
}

func NewMasterDataReviews(requests careplan.StudentDataRequestQuery, people ports.MasterDataDirectory, scope ports.ReviewScopeResolver, today func() careplan.Date) *MasterDataReviews {
	return &MasterDataReviews{requests: requests, people: people, scope: scope, today: today}
}

func (s *MasterDataReviews) ListPending(ctx context.Context, filter careplan.RequestQueueFilter) ([]*careplan.MasterDataReviewItem, *careplan.RequestCursor, error) {
	probe := probeLimit(filter)
	rows, err := s.requests.ListStudentDataRequests(ctx, careplan.StudentDataRequestFilter{Statuses: []string{"pending"}, Queue: &probe})
	if err != nil {
		return nil, nil, fmt.Errorf("review: list pending: %w", err)
	}
	rows, next := nextCursor(rows, filter.Limit, func(row careplan.StudentDataChangeRequest) (time.Time, int64) {
		return row.CreatedAt, row.ID
	})
	if len(rows) == 0 {
		return []*careplan.MasterDataReviewItem{}, next, nil
	}
	changes := make([]ports.MasterDataFieldChange, 0, len(rows))
	for _, row := range rows {
		changes = append(changes, ports.MasterDataFieldChange{
			RequestID: row.ID, StudentID: row.StudentID, Target: row.Target, Field: row.FieldKey,
			OldValue: row.OldValue, NewValue: row.NewValue,
		})
	}
	facts, err := s.people.ReviewFields(ctx, changes)
	if err != nil {
		return nil, nil, err
	}
	scope, err := s.scope(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("review: resolve reviewer scope: %w", err)
	}
	day := careplan.Date(filter.UrgentDate)
	if day.IsZero() {
		day = s.today()
	}
	items := make([]*careplan.MasterDataReviewItem, 0, len(rows))
	for i := range rows {
		fact := facts[rows[i].ID]
		if !scope.Allows(fact.Student) || fact.Student.Alumnus || fact.Student.CareEndedOn(day) {
			continue
		}
		items = append(items, &careplan.MasterDataReviewItem{
			Request: &rows[i], FirstName: fact.FirstName, LastName: fact.LastName,
			BulkEligible: fact.BulkEligible, BulkIneligibleReason: fact.BulkIneligibleReason,
			BulkIneligibleText: fact.BulkIneligibleText, CurrentValueChanged: fact.CurrentValueChanged,
		})
	}
	return items, next, nil
}

func (s *MasterDataReviews) ListHistory(ctx context.Context, filter careplan.RequestQueueFilter) ([]*careplan.MasterDataHistoryItem, *careplan.RequestCursor, error) {
	probe := probeLimit(filter)
	rows, err := s.requests.ListStudentDataRequests(ctx, careplan.StudentDataRequestFilter{
		Statuses: []string{"auto_applied", "approved", "rejected"}, Queue: &probe,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("review: list decided: %w", err)
	}
	rows, next := nextCursor(rows, filter.Limit, func(row careplan.StudentDataChangeRequest) (time.Time, int64) {
		return row.UpdatedAt, row.ID
	})
	if len(rows) == 0 {
		return []*careplan.MasterDataHistoryItem{}, nil, nil
	}
	studentIDs := make([]int64, 0, len(rows))
	reviewerIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		studentIDs = append(studentIDs, row.StudentID)
		if row.ReviewedBy != nil && *row.ReviewedBy > 0 {
			reviewerIDs = append(reviewerIDs, *row.ReviewedBy)
		}
	}
	students, err := s.people.FindStudents(ctx, studentIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("review: load students: %w", err)
	}
	personIDs := make([]int64, 0, len(students))
	for _, student := range students {
		personIDs = append(personIDs, student.PersonID)
	}
	persons, err := s.people.PersonNames(ctx, personIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("review: load persons: %w", err)
	}
	scope, err := s.scope(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("review: resolve reviewer scope: %w", err)
	}
	reviewers, err := s.people.ReviewerNames(ctx, reviewerIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("review: load reviewers: %w", err)
	}
	items := make([]*careplan.MasterDataHistoryItem, 0, len(rows))
	for i := range rows {
		student, found := students[rows[i].StudentID]
		if !found || !scope.Allows(&student) || student.Alumnus {
			continue
		}
		person := persons[student.PersonID]
		items = append(items, &careplan.MasterDataHistoryItem{
			Request: &rows[i], FirstName: person.FirstName, LastName: person.LastName,
			ReviewerName: reviewerDisplayName(reviewers, rows[i].ReviewedBy),
		})
	}
	return items, next, nil
}
