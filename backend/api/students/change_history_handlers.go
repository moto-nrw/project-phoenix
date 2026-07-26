package students

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/models/users"
)

// changeHistoryEntry is the per-child change-history row returned to the
// frontend (issue #1455). Values are already display-ready German strings, so
// the UI renders a plain "Vorher → Nachher" table without field-specific logic.
type changeHistoryEntry struct {
	ID        string    `json:"id"`
	Field     string    `json:"field"`
	OldValue  string    `json:"old_value"`
	NewValue  string    `json:"new_value"`
	EditedBy  string    `json:"edited_by"`
	ChangedAt time.Time `json:"changed_at"`
}

// recordStudentChanges appends the per-child change history for an update.
// Atomic with the edit: the audit write runs on the caller's transaction, so a
// failed insert poisons that transaction (PostgreSQL 25P02) and any later
// COMMIT fails anyway. We therefore propagate the error and let the edit roll
// back rather than commit an unlogged change — a recorded edit must always have
// its audit row.
func (rs *Resource) recordStudentChanges(ctx context.Context, before, after *users.Student) error {
	if rs.StudentAuditService == nil {
		return nil
	}

	claims := jwt.ClaimsFromCtx(ctx)
	name := strings.TrimSpace(claims.FirstName + " " + claims.LastName)
	if name == "" {
		name = claims.Username
	}

	if err := rs.StudentAuditService.RecordChanges(ctx, before, after, int64(claims.ID), name); err != nil {
		if rs.Logger != nil {
			rs.Logger.Warn("failed to record student change history",
				slog.Int64("student_id", after.ID),
				slog.String("error", err.Error()),
			)
		}
		return err
	}
	return nil
}

func normalizeDeparturePlanForAudit(student *users.Student) {
	if student == nil {
		return
	}
	if student.AllowedDepartureModes != nil {
		student.DepartureDays = student.AllowedDepartureModes.DepartureDays()
		return
	}
	student.DepartureDays = users.DepartureDaysFromLegacy(student.BusDays, student.PickupDays)
}

// getStudentChangeHistory returns the per-child change history. Gated on full
// access (admin / group supervisor) so it reads as a support tool for the OGS
// office, not general staff surveillance.
func (rs *Resource) getStudentChangeHistory(w http.ResponseWriter, r *http.Request) {
	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return
	}

	if !rs.checkStudentFullAccess(r, student) {
		renderError(w, r, common.ErrorForbidden(errors.New("full access required to view change history")))
		return
	}

	if rs.StudentAuditService == nil {
		renderError(w, r, common.ErrorInternalServer(errors.New("student audit service not configured")))
		return
	}

	edits, err := rs.StudentAuditService.GetChangeHistory(r.Context(), student.ID)
	if err != nil {
		renderError(w, r, common.ErrorInternalServer(err))
		return
	}

	entries := make([]changeHistoryEntry, 0, len(edits))
	for _, e := range edits {
		entries = append(entries, changeHistoryEntry{
			ID:        strconv.FormatInt(e.ID, 10),
			Field:     e.FieldName,
			OldValue:  derefOrEmpty(e.OldValue),
			NewValue:  derefOrEmpty(e.NewValue),
			EditedBy:  e.EditedByName,
			ChangedAt: e.CreatedAt,
		})
	}

	common.Respond(w, r, http.StatusOK, entries, "Change history retrieved successfully")
}

func derefOrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
