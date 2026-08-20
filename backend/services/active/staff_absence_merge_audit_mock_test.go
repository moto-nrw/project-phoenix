package active

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingAbsenceDeletionAuditRepo struct {
	events []*auditModels.TimeTrackingDeletion
	err    error
}

func TestWriteAbsenceDeletionAuditIncludesCustomLabel(t *testing.T) {
	svc, _, _ := absSetupService()
	customID := int64(42)
	svc.absenceTypes = NewStaffAbsenceTypeService(&absTypeRepoMock{rows: []*activeModels.StaffAbsenceType{{
		Name:  "Regenerationstag",
		Model: base.Model{ID: customID},
	}}}, nil)
	deletions := &recordingAbsenceDeletionAuditRepo{}
	svc.deletionRepo = deletions

	require.NoError(t, svc.writeAbsenceDeletionAudit(context.Background(), &activeModels.StaffAbsence{
		AbsenceType:   activeModels.AbsenceTypeOther,
		AbsenceTypeID: &customID,
	}, 100))
	require.Len(t, deletions.events, 1)
	var payload map[string]any
	require.NoError(t, json.Unmarshal(deletions.events[0].Payload, &payload))
	assert.Equal(t, "Regenerationstag", payload["absence_type_label"])
}

func (r *recordingAbsenceDeletionAuditRepo) Create(_ context.Context, event *auditModels.TimeTrackingDeletion) error {
	if r.err != nil {
		return r.err
	}
	r.events = append(r.events, event)
	return nil
}

func mergeAuditAbsences() (*activeModels.StaffAbsence, *activeModels.StaffAbsence) {
	primary := &activeModels.StaffAbsence{
		StaffID:     100,
		AbsenceType: activeModels.AbsenceTypeSick,
		DateStart:   timezone.NewDate(2026, 2, 10),
		DateEnd:     timezone.NewDate(2026, 2, 11),
		Status:      activeModels.AbsenceStatusReported,
		CreatedBy:   100,
	}
	primary.ID = 42
	secondary := &activeModels.StaffAbsence{
		StaffID:     100,
		AbsenceType: activeModels.AbsenceTypeSick,
		DateStart:   timezone.NewDate(2026, 2, 13),
		DateEnd:     timezone.NewDate(2026, 2, 14),
		Status:      activeModels.AbsenceStatusReported,
		CreatedBy:   100,
		Note:        "Zweiter Bericht",
	}
	secondary.ID = 43
	return primary, secondary
}

func TestAbsCreateAbsenceFor_MergeWritesSecondaryDeletionAudit(t *testing.T) {
	svc, absRepo, _ := absSetupServiceWithSyncer()
	primary, secondary := mergeAuditAbsences()
	absRepo.getByStaffAndDateRangeFunc = func(context.Context, int64, timezone.Date, timezone.Date) ([]*activeModels.StaffAbsence, error) {
		return []*activeModels.StaffAbsence{primary, secondary}, nil
	}
	deletionRepo := &recordingAbsenceDeletionAuditRepo{}
	svc.deletionRepo = deletionRepo

	_, err := svc.CreateAbsenceFor(context.Background(), 100, 200, nil, CreateAbsenceRequest{
		AbsenceType: activeModels.AbsenceTypeSick,
		DateStart:   "2026-02-11",
		DateEnd:     "2026-02-13",
	})

	require.NoError(t, err)
	require.Len(t, deletionRepo.events, 1)
	event := deletionRepo.events[0]
	assert.Equal(t, int64(100), event.StaffID)
	assert.Equal(t, auditModels.TimeTrackingDeletionSourceAbsence, event.Source)
	assert.Equal(t, secondary.ID, event.SourceID)
	assert.Equal(t, int64(200), event.DeletedBy)
	assert.Equal(t, "Zweiter Bericht", event.Note)
	assert.Contains(t, string(event.Payload), `"id":43`)
}

func TestAbsCreateAbsenceFor_MergeAuditFailureKeepsSecondary(t *testing.T) {
	svc, absRepo, _ := absSetupServiceWithSyncer()
	primary, secondary := mergeAuditAbsences()
	absRepo.getByStaffAndDateRangeFunc = func(context.Context, int64, timezone.Date, timezone.Date) ([]*activeModels.StaffAbsence, error) {
		return []*activeModels.StaffAbsence{primary, secondary}, nil
	}
	deleted := false
	absRepo.deleteFunc = func(context.Context, any) error {
		deleted = true
		return nil
	}
	svc.deletionRepo = &recordingAbsenceDeletionAuditRepo{err: errors.New("audit unavailable")}

	_, err := svc.CreateAbsenceFor(context.Background(), 100, 200, nil, CreateAbsenceRequest{
		AbsenceType: activeModels.AbsenceTypeSick,
		DateStart:   "2026-02-11",
		DateEnd:     "2026-02-13",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to audit merged absence 43")
	assert.False(t, deleted)
}
