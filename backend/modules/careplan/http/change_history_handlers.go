package careplanhttp

import (
	context "context"
	slog "log/slog"
	strings "strings"

	sessiontoken "github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	peopleModule "github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	departure "github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"
)

// recordStudentChanges appends the per-child change history for an update.
// Atomic with the edit: the audit write runs on the caller's transaction, so a
// failed insert poisons that transaction (PostgreSQL 25P02) and any later
// COMMIT fails anyway. We therefore propagate the error and let the edit roll
// back rather than commit an unlogged change — a recorded edit must always have
// its audit row.
func (rs *Resource) recordStudentChanges(ctx context.Context, before, after *peopleModule.StudentState) error {
	if rs.StudentAuditService == nil {
		return nil
	}

	editedBy := int64(sessiontoken.ClaimsFromCtx(ctx).ID)
	if err := rs.StudentAuditService.RecordStudentChanges(ctx, before.AuditSnapshot(), after.AuditSnapshot(), editedBy, auditActorName(ctx, editedBy)); err != nil {
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

// auditActorName is the display name the change history records for the
// authenticated editor: the name on the session, else its username. A context
// whose account is not the editor is attributed to nobody rather than to the
// wrong person; the owner stores its own stand-in for that.
func auditActorName(ctx context.Context, editedBy int64) string {
	claims := sessiontoken.ClaimsFromCtx(ctx)
	if int64(claims.ID) != editedBy {
		return ""
	}
	name := strings.TrimSpace(claims.FirstName + " " + claims.LastName)
	if name == "" {
		name = strings.TrimSpace(claims.Username)
	}
	return name
}

func normalizeDeparturePlanForAudit(student *peopleModule.StudentState) {
	if student == nil {
		return
	}
	if allowed := student.AllowedDepartureModes.Normalize(); allowed.HasAny() {
		student.AllowedDepartureModes = allowed
		student.DepartureDays = allowed.DepartureDays()
		return
	}
	student.DepartureDays = departure.DepartureDaysFromLegacy(student.BusDays, student.PickupDays)
}

func studentAuditBeforeImage(student *peopleModule.StudentState, normalizeDeparture bool) peopleModule.StudentState {
	before := *student
	if normalizeDeparture {
		normalizeDeparturePlanForAudit(&before)
	}
	return before
}
