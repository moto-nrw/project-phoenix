package timetracking

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingAbsenceDeletionAuditRepo struct {
	events []*TimeTrackingDeletionEvent
	err    error
}

func TestWriteAbsenceDeletionAuditIncludesCustomLabel(t *testing.T) {
	t.Parallel()

	svc, _, _ := absSetupService()
	customID := int64(42)
	svc.absenceTypes = &absTypeReaderMock{rows: []*StaffAbsenceType{{
		Name:  "Regenerationstag",
		Model: Model{ID: customID},
	}}}
	deletions := &recordingAbsenceDeletionAuditRepo{}
	svc.deletionRepo = deletions

	require.NoError(t, svc.writeAbsenceDeletionAudit(context.Background(), &StaffAbsence{
		AbsenceType:   AbsenceTypeOther,
		AbsenceTypeID: &customID,
	}, 100))
	require.Len(t, deletions.events, 1)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(deletions.events[0].Payload, &payload))
	assert.Equal(t, "Regenerationstag", payload["absence_type_label"])
}

func (r *recordingAbsenceDeletionAuditRepo) Create(_ context.Context, event *TimeTrackingDeletionEvent) error {
	if r.err != nil {
		return r.err
	}
	r.events = append(r.events, event)
	return nil
}

func mergeAuditAbsences() (*StaffAbsence, *StaffAbsence) {
	primary := &StaffAbsence{
		StaffID:     100,
		AbsenceType: AbsenceTypeSick,
		DateStart:   NewDate(2026, 2, 10),
		DateEnd:     NewDate(2026, 2, 11),
		Status:      AbsenceStatusReported,
		CreatedBy:   100,
	}
	primary.ID = 42
	secondary := &StaffAbsence{
		StaffID:     100,
		AbsenceType: AbsenceTypeSick,
		DateStart:   NewDate(2026, 2, 13),
		DateEnd:     NewDate(2026, 2, 14),
		Status:      AbsenceStatusReported,
		CreatedBy:   100,
		Note:        "Zweiter Bericht",
	}
	secondary.ID = 43
	return primary, secondary
}

func TestAbsCreateAbsenceFor_MergeWritesSecondaryDeletionAudit(t *testing.T) {
	t.Parallel()

	svc, absRepo, _ := absSetupServiceWithSyncer()
	primary, secondary := mergeAuditAbsences()
	absRepo.getByStaffAndDateRangeFunc = func(context.Context, int64, Date, Date) ([]*StaffAbsence, error) {
		return []*StaffAbsence{primary, secondary}, nil
	}
	deletionRepo := &recordingAbsenceDeletionAuditRepo{}
	svc.deletionRepo = deletionRepo

	_, err := svc.CreateAbsenceFor(context.Background(), 100, 200, nil, CreateAbsenceRequest{
		AbsenceType: AbsenceTypeSick,
		DateStart:   "2026-02-11",
		DateEnd:     "2026-02-13",
	})

	require.NoError(t, err)
	require.Len(t, deletionRepo.events, 1)
	event := deletionRepo.events[0]
	assert.Equal(t, int64(100), event.StaffID)
	assert.Equal(t, "absence", event.Source)
	assert.Equal(t, secondary.ID, event.SourceID)
	assert.Equal(t, int64(200), event.DeletedBy)
	assert.Equal(t, "Zweiter Bericht", event.Note)
	assert.Contains(t, string(event.Payload), `"id":43`)
}

func TestAbsCreateAbsenceFor_MergeAuditFailureKeepsSecondary(t *testing.T) {
	t.Parallel()

	svc, absRepo, _ := absSetupServiceWithSyncer()
	primary, secondary := mergeAuditAbsences()
	absRepo.getByStaffAndDateRangeFunc = func(context.Context, int64, Date, Date) ([]*StaffAbsence, error) {
		return []*StaffAbsence{primary, secondary}, nil
	}
	deleted := false
	absRepo.deleteFunc = func(context.Context, any) error {
		deleted = true
		return nil
	}
	svc.deletionRepo = &recordingAbsenceDeletionAuditRepo{err: errors.New("audit unavailable")}

	_, err := svc.CreateAbsenceFor(context.Background(), 100, 200, nil, CreateAbsenceRequest{
		AbsenceType: AbsenceTypeSick,
		DateStart:   "2026-02-11",
		DateEnd:     "2026-02-13",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to audit merged absence 43")
	assert.False(t, deleted)
}
