package enrollment_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
)

// stubAccessLogRepo is an in-memory audit.DataAccessLogRepository that
// captures the rows written, so the export-audit unit tests assert the
// shape of the row without a database.
type stubAccessLogRepo struct {
	entries []*auditModels.DataAccessLog
	err     error
}

func (s *stubAccessLogRepo) Create(_ context.Context, entry *auditModels.DataAccessLog) error {
	if s.err != nil {
		return s.err
	}
	s.entries = append(s.entries, entry)
	return nil
}

func (s *stubAccessLogRepo) ExistsSince(context.Context, int64, string, map[string]string, time.Time) (bool, error) {
	return false, nil
}

// IDs are deliberately well above 9 so the hermetic hardcoded-ID check
// (which flags int64(1)..int64(9)) leaves these in-memory sentinels alone.
func samplePhaseForAudit() *enrollmentModels.Phase {
	p := &enrollmentModels.Phase{
		Name:             "Schuljahr 2026/27",
		ServiceStartDate: timezone.NewDate(2026, 8, 1),
		ServiceEndDate:   timezone.NewDate(2027, 7, 31),
	}
	p.ID = 7401
	return p
}

func newAuditDecisionService(repo auditModels.DataAccessLogRepository) enrollmentService.DecisionService {
	return enrollmentService.NewDecisionService(enrollmentService.DecisionServiceConfig{
		DataAccessLogRepo: repo,
	})
}

func TestRecordPhaseExportAudit_WritesRowWithPhaseWindowAndMetadata(t *testing.T) {
	repo := &stubAccessLogRepo{}
	svc := newAuditDecisionService(repo)
	phase := samplePhaseForAudit()

	err := svc.RecordPhaseExportAudit(context.Background(), 4242, "admin,config_manager", phase, "pdf", "approved", 12, 19)
	if err != nil {
		t.Fatalf("RecordPhaseExportAudit() error = %v", err)
	}
	if len(repo.entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(repo.entries))
	}
	e := repo.entries[0]

	if e.ResourceType != auditModels.ResourceTypeEnrollmentPhaseExport {
		t.Errorf("resource_type = %q, want %q", e.ResourceType, auditModels.ResourceTypeEnrollmentPhaseExport)
	}
	if e.ActorAccountID != 4242 {
		t.Errorf("actor_account_id = %d, want 4242", e.ActorAccountID)
	}
	if e.ActorRole != "admin,config_manager" {
		t.Errorf("actor_role = %q, want admin,config_manager", e.ActorRole)
	}
	if e.StudentID != nil {
		t.Errorf("student_id = %v, want nil (phase-wide export)", *e.StudentID)
	}
	// range_* must carry the phase service window — the disclosed span.
	// As TIMESTAMPTZ bounds the window maps to Berlin start/end of day.
	if !e.RangeStart.Equal(phase.ServiceStartDate.BerlinMidnight()) || !e.RangeEnd.Equal(phase.ServiceEndDate.EndOfDay()) {
		t.Errorf("range = [%s..%s], want [%s..%s]",
			e.RangeStart, e.RangeEnd, phase.ServiceStartDate, phase.ServiceEndDate)
	}
	if e.AccessedAt.IsZero() {
		t.Error("accessed_at must be set")
	}

	md := e.GetMetadata()
	if md["phase_id"] != phase.ID {
		t.Errorf("metadata.phase_id = %v, want %d", md["phase_id"], phase.ID)
	}
	if md["format"] != "pdf" {
		t.Errorf("metadata.format = %v, want pdf", md["format"])
	}
	if md["status_filter"] != "approved" {
		t.Errorf("metadata.status_filter = %v, want approved", md["status_filter"])
	}
	if md["request_count"] != 12 {
		t.Errorf("metadata.request_count = %v, want 12", md["request_count"])
	}
	if md["child_count"] != 19 {
		t.Errorf("metadata.child_count = %v, want 19", md["child_count"])
	}
}

func TestRecordPhaseExportAudit_DefaultsEmptyRole(t *testing.T) {
	repo := &stubAccessLogRepo{}
	svc := newAuditDecisionService(repo)

	if err := svc.RecordPhaseExportAudit(context.Background(), 4242, "  ", samplePhaseForAudit(), "xlsx", "", 1, 1); err != nil {
		t.Fatalf("RecordPhaseExportAudit() error = %v", err)
	}
	if got := repo.entries[0].ActorRole; got != "unknown" {
		t.Errorf("actor_role = %q, want unknown (NOT NULL column never empty)", got)
	}
	// An empty filter is recorded as the explicit "all" scope.
	if got := repo.entries[0].GetMetadata()["status_filter"]; got != "all" {
		t.Errorf("metadata.status_filter = %v, want all (empty filter)", got)
	}
}

func TestRecordPhaseExportAudit_PropagatesWriteFailure(t *testing.T) {
	repo := &stubAccessLogRepo{err: errors.New("db down")}
	svc := newAuditDecisionService(repo)

	// A failed audit write must surface as an error so the handler
	// refuses to serve the export (no trail, no disclosure).
	if err := svc.RecordPhaseExportAudit(context.Background(), 4242, "admin", samplePhaseForAudit(), "pdf", "", 1, 1); err == nil {
		t.Error("expected error when the audit write fails")
	}
}

func TestRecordPhaseExportAudit_GuardsBadInput(t *testing.T) {
	svc := newAuditDecisionService(&stubAccessLogRepo{})

	if err := svc.RecordPhaseExportAudit(context.Background(), 0, "admin", samplePhaseForAudit(), "pdf", "", 1, 1); err == nil {
		t.Error("expected error for missing actor account id")
	}
	if err := svc.RecordPhaseExportAudit(context.Background(), 4242, "admin", nil, "pdf", "", 1, 1); err == nil {
		t.Error("expected error for nil phase")
	}

	// Nil repo must fail closed, not silently skip the audit.
	if err := newAuditDecisionService(nil).RecordPhaseExportAudit(
		context.Background(), 4242, "admin", samplePhaseForAudit(), "pdf", "", 1, 1); err == nil {
		t.Error("expected error when the audit repo is not configured")
	}
}
