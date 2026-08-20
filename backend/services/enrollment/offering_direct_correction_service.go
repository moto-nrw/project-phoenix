package enrollment

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/auth/authorize"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
)

// DirectCorrectionItem is one admin correction to a child's bookings as the
// central history shows it (#2436): who changed what, when and why. It is not
// a request — there is no pending state and nothing to decide.
type DirectCorrectionItem struct {
	Adjustment  *auditModels.EnrollmentOfferingAdjustment
	StudentName string
	// ActorName falls back to the e-mail snapshot and finally to "Unbekannt",
	// so a deleted account never renders an account id.
	ActorName string
	// Diff holds only the offerings whose booking actually changed, in the
	// same shape the offering-request history uses.
	Diff []OfferingChangeDiffEntry
}

// ListDirectCorrections returns the tenant's admin direct corrections,
// newest change first, keyset paginated on (changed_at, id).
func (s *offeringChangeRequestService) ListDirectCorrections(
	ctx context.Context,
	beforeChangedAt time.Time,
	beforeID int64,
	limit int,
) ([]*DirectCorrectionItem, *usersService.HistoryCursor, error) {
	if s.OfferingAdjustmentRepo == nil {
		return nil, nil, fmt.Errorf("offering change: offering adjustment repo not configured")
	}
	// limit+1 probes for an older page without a second count query.
	rows, err := s.OfferingAdjustmentRepo.ListDirectForTenant(ctx, beforeChangedAt, beforeID, limit+1)
	if err != nil {
		return nil, nil, fmt.Errorf("offering change: list direct corrections: %w", err)
	}
	// The cursor points at the last DB row, not the last visible item: the
	// per-child scope filters after the DB limit, so a cursor built from the
	// filtered page would skip rows.
	var next *usersService.HistoryCursor
	if len(rows) > limit {
		rows = rows[:limit]
		last := rows[len(rows)-1]
		next = &usersService.HistoryCursor{UpdatedAt: last.ChangedAt, ID: last.ID}
	}
	if len(rows) == 0 {
		return []*DirectCorrectionItem{}, nil, nil
	}

	studentIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		studentIDs = append(studentIDs, row.StudentID)
	}
	students, err := s.StudentRepo.FindByIDs(ctx, studentIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("offering change: load students for direct corrections: %w", err)
	}
	personIDs := make([]int64, 0, len(students))
	for _, st := range students {
		personIDs = append(personIDs, st.PersonID)
	}
	persons, err := s.PersonRepo.FindByIDs(ctx, personIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("offering change: load student persons for direct corrections: %w", err)
	}

	// Same per-child scope as the request history: write gate + alumnus skip.
	writable := authorize.WritableStudentFilter(ctx, jwt.PermissionsFromCtx(ctx), s.UserContext)

	items := make([]*DirectCorrectionItem, 0, len(rows))
	for _, row := range rows {
		student := students[row.StudentID]
		if student == nil || !writable(student) || student.IsAlumnus() {
			continue
		}
		item := &DirectCorrectionItem{
			Adjustment: row,
			ActorName:  directCorrectionActorName(row),
			Diff:       directCorrectionDiff(row, s.Logger),
		}
		if person := persons[student.PersonID]; person != nil {
			item.StudentName = strings.TrimSpace(person.GetFullName())
		}
		items = append(items, item)
	}
	return items, next, nil
}

func directCorrectionActorName(row *auditModels.EnrollmentOfferingAdjustment) string {
	if row.ActorNameSnapshot != nil && strings.TrimSpace(*row.ActorNameSnapshot) != "" {
		return strings.TrimSpace(*row.ActorNameSnapshot)
	}
	if row.ActorEmailSnapshot != nil && strings.TrimSpace(*row.ActorEmailSnapshot) != "" {
		return strings.TrimSpace(*row.ActorEmailSnapshot)
	}
	// Spec: deleted accounts read as "Unbekannt", never as an account id.
	return "Unbekannt"
}

// directCorrectionDiff turns the frozen before/after snapshots into the same
// changed-lines shape the offering-request history renders. Snapshots are
// written by the correction itself, so this is never a live recomputation.
func directCorrectionDiff(row *auditModels.EnrollmentOfferingAdjustment, logger *slog.Logger) []OfferingChangeDiffEntry {
	before, beforeErr := decodeAdjustmentSnapshot(row.Before)
	after, afterErr := decodeAdjustmentSnapshot(row.After)
	if beforeErr != nil || afterErr != nil {
		if logger != nil {
			logger.Warn("offering change: decode direct correction snapshot failed",
				slog.Int64("adjustment_id", row.ID),
			)
		}
		return nil
	}

	ids := make([]int64, 0, len(before)+len(after))
	labels := make(map[int64]string, len(before)+len(after))
	for _, side := range []map[int64]offeringAdjustmentSnapshot{before, after} {
		for id, snapshot := range side {
			if !slices.Contains(ids, id) {
				ids = append(ids, id)
			}
			if name := strings.TrimSpace(snapshot.OfferingName); name != "" {
				labels[id] = name
			}
		}
	}
	slices.Sort(ids)

	diff := make([]OfferingChangeDiffEntry, 0, len(ids))
	for _, id := range ids {
		entry := OfferingChangeDiffEntry{OfferingID: id, Label: labels[id]}
		if entry.Label == "" {
			entry.Label = fmt.Sprintf("Angebot %d", id)
		}
		entry.OldState = "not_booked"
		if snapshot, ok := before[id]; ok {
			entry.OldState = "booked"
			entry.OldDays = canonicalDays(snapshot.SelectedDays)
		}
		entry.NewState = "removed"
		if snapshot, ok := after[id]; ok {
			entry.NewState = "booked"
			entry.NewDays = canonicalDays(snapshot.SelectedDays)
		}
		if entry.OldState == entry.NewState && slices.Equal(entry.OldDays, entry.NewDays) {
			continue
		}
		diff = append(diff, entry)
	}
	return diff
}

func decodeAdjustmentSnapshot(raw json.RawMessage) (map[int64]offeringAdjustmentSnapshot, error) {
	out := map[int64]offeringAdjustmentSnapshot{}
	if len(raw) == 0 {
		return out, nil
	}
	var entries []offeringAdjustmentSnapshot
	if err := json.Unmarshal(raw, &entries); err != nil {
		return nil, err
	}
	for _, entry := range entries {
		id, err := strconv.ParseInt(entry.OfferingID, 10, 64)
		if err != nil {
			return nil, err
		}
		out[id] = entry
	}
	return out, nil
}
