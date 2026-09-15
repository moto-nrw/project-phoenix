package compose

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/auditlog"
	auditcompose "github.com/moto-nrw/project-phoenix/modules/auditlog/compose"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/requestreview"
	"github.com/uptrace/bun"
)

type AuditObservation = auditlog.Observation

func NewCorrectionLog(db *bun.DB, people ReviewPeople, allowed func(context.Context) bool, observe func(AuditObservation)) (requestreview.CorrectionLog, error) {
	if people == nil || allowed == nil {
		return nil, errors.New("correction log: people and access policy are required")
	}
	query, err := auditcompose.NewCorrectionQueries(db, observe)
	if err != nil {
		return nil, err
	}
	return correctionLog{query: query, people: reviewDirectory{people: people}, allowed: allowed}, nil
}

type correctionLog struct {
	query   auditlog.CorrectionQuery
	people  reviewDirectory
	allowed func(context.Context) bool
}

func (c correctionLog) History(ctx context.Context, filter requestreview.QueueFilter) ([]requestreview.Row, *requestreview.Cursor, error) {
	probe := auditlog.CorrectionFilter{Limit: filter.Limit}
	if probe.Limit > 0 {
		probe.Limit++
	}
	if filter.Before != nil {
		probe.BeforeInstant, probe.BeforeID = filter.Before.Instant, filter.Before.ID
	}
	rows, err := c.query.ListDirectCorrections(ctx, probe)
	if err != nil {
		return nil, nil, fmt.Errorf("offering change: list direct corrections: %w", err)
	}
	var next *requestreview.Cursor
	if filter.Limit > 0 && len(rows) > filter.Limit {
		rows = rows[:filter.Limit]
		last := rows[len(rows)-1]
		next = &requestreview.Cursor{Instant: last.ChangedAt, ID: last.ID}
	}
	result := make([]requestreview.Row, 0, len(rows))
	if len(rows) == 0 {
		return result, next, nil
	}
	ids := make([]int64, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.StudentID)
	}
	students, err := c.people.FindStudents(ctx, ids)
	if err != nil {
		return nil, nil, fmt.Errorf("offering change: load students for direct corrections: %w", err)
	}
	ids = ids[:0]
	for _, student := range students {
		ids = append(ids, student.PersonID)
	}
	persons, err := c.people.PersonNames(ctx, ids)
	if err != nil {
		return nil, nil, fmt.Errorf("offering change: load student persons for direct corrections: %w", err)
	}
	if !c.allowed(ctx) {
		return result, next, nil
	}
	for _, row := range rows {
		student, found := students[row.StudentID]
		if !found || student.Alumnus {
			continue
		}
		person := persons[student.PersonID]
		name := strings.TrimSpace(person.FirstName + " " + person.LastName)
		entries, diffErr := careplan.ReviewCorrectionDiff(row.Before, row.After)
		if diffErr != nil {
			slog.Warn("offering change: decode direct correction snapshot failed",
				"adjustment_id", row.ID,
			)
		}
		diff := make([]requestreview.OfferingRequestDiffResponse, 0, len(entries))
		for _, entry := range entries {
			diff = append(diff, requestreview.OfferingRequestDiffResponse{OfferingID: strconv.FormatInt(entry.OfferingID, 10), Label: entry.Label, Old: GermanOfferingDiffLabel(entry.OldState, entry.OldDays), New: GermanOfferingDiffLabel(entry.NewState, entry.NewDays)})
		}
		result = append(result, requestreview.Row{Type: requestreview.TypeDirectCorrection, SortTime: row.ChangedAt, ID: row.ID, StudentID: row.StudentID, StudentName: name, DecidedAt: row.ChangedAt,
			Data: requestreview.DirectCorrectionResponse{ID: strconv.FormatInt(row.ID, 10), StudentID: strconv.FormatInt(row.StudentID, 10), StudentName: name, ChangedAt: row.ChangedAt, ChangedByName: row.ActorName(), Reason: row.Reason, Diff: diff},
		})
	}
	return result, next, nil
}

func GermanOfferingDiffLabel(state string, days []string) string {
	switch state {
	case "not_booked":
		return "nicht gebucht"
	case "removed":
		return "abgemeldet"
	}
	if len(days) == 0 {
		return "alle Betreuungstage"
	}
	labels := map[string]string{"mon": "Mo", "tue": "Di", "wed": "Mi", "thu": "Do", "fri": "Fr", "sat": "Sa", "sun": "So"}
	parts := make([]string, 0, len(days))
	for _, day := range days {
		if label, found := labels[day]; found {
			parts = append(parts, label)
		}
	}
	return strings.Join(parts, ", ")
}
