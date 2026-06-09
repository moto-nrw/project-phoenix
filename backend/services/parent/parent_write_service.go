package parent

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// maxParentNoteLen bounds a single note so a parent can't paste a novel
// the staff card then has to render. Generous for a "kurze Nachricht".
const maxParentNoteLen = 2000

// dateKeyLayout keys a status-day by its calendar date for set membership.
const dateKeyLayout = "2006-01-02"

// Sentinel errors the HTTP layer maps to stable status codes. They are
// part of the package contract — handlers switch on them via errors.Is.
var (
	// ErrChildNotLinked means the account is not a guardian of the
	// student. Handlers MUST map this to 403/404 and never leak whether
	// the student exists at another school.
	ErrChildNotLinked = errors.New("parent: child not linked to account")
	// ErrSickNoteDisabled means operations.parent_sick_note_enabled is
	// off for the child's tenant.
	ErrSickNoteDisabled = errors.New("parent: sick notes disabled for this school")
	// ErrNotesDisabled means operations.parent_notes_enabled is off for
	// the child's tenant.
	ErrNotesDisabled = errors.New("parent: parent notes disabled for this school")
	// ErrNoDates means the sick-note request carried no dates.
	ErrNoDates = errors.New("parent: at least one date is required")
	// ErrEmptyNote means the note body was blank after trimming.
	ErrEmptyNote = errors.New("parent: note body must not be empty")
	// ErrNoteTooLong means the note body exceeded maxParentNoteLen.
	ErrNoteTooLong = errors.New("parent: note body too long")
)

// resolveOwnedChild validates the account is a guardian of the student
// and returns the child's tenant id. The cross-tenant lookup runs under
// an admin tx; a nil child becomes ErrChildNotLinked so the caller never
// trusts a studentID it can't prove ownership of.
func (s *service) resolveOwnedChild(ctx context.Context, accountID, studentID int64) (*parentChild, error) {
	if accountID <= 0 {
		return nil, fmt.Errorf("parent: account_id must be positive")
	}
	if studentID <= 0 {
		return nil, fmt.Errorf("parent: student_id must be positive")
	}

	var resolved *parentChild
	err := tenant.WithAdminTx(ctx, s.db, func(adminCtx context.Context, _ bun.Tx) error {
		child, findErr := s.childRepo.FindForAccount(adminCtx, accountID, studentID)
		if findErr != nil {
			return findErr
		}
		if child == nil {
			return ErrChildNotLinked
		}
		resolved = &parentChild{tenantID: child.TenantID}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return resolved, nil
}

// parentChild is the minimal resolved context a per-child write needs.
type parentChild struct {
	tenantID int64
}

// SubmitSickNote reports the child sick for the given dates.
func (s *service) SubmitSickNote(ctx context.Context, accountID, studentID int64, dates []time.Time, reason string) ([]*activeModels.StudentStatusDay, error) {
	if len(dates) == 0 {
		return nil, ErrNoDates
	}

	child, err := s.resolveOwnedChild(ctx, accountID, studentID)
	if err != nil {
		return nil, err
	}

	enabled, err := s.settings.ResolveBoolForTenant(ctx, child.tenantID, configModels.KeyParentSickNoteEnabled)
	if err != nil {
		return nil, fmt.Errorf("parent: resolve sick-note setting: %w", err)
	}
	if !enabled {
		return nil, ErrSickNoteDisabled
	}

	now := time.Now()
	today := timezone.DateOfUTC(now)

	var notePtr *string
	if trimmed := strings.TrimSpace(reason); trimmed != "" {
		// Count characters (runes), not UTF-8 bytes, so the limit matches the
		// frontend's maxLength — a German text with umlauts stays under the
		// budget even though it spans more bytes.
		if utf8.RuneCountInString(trimmed) > maxParentNoteLen {
			return nil, ErrNoteTooLong
		}
		notePtr = &trimmed
	}

	normalized := make([]time.Time, 0, len(dates))
	for _, d := range dates {
		normalized = append(normalized, timezone.DateOfUTC(d))
	}

	var result []*activeModels.StudentStatusDay
	txErr := tenant.WithTenantTx(ctx, s.db, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		// A sick day and an excused day are mutually exclusive — clear any
		// excused entries the parent's sick note now overrides.
		if err := s.statusDayRepo.MarkClearedForDates(txCtx, studentID, activeModels.StudentStatusDayExcused, normalized, now, activeModels.StudentStatusSourceParent); err != nil {
			return err
		}
		for _, d := range normalized {
			if err := s.statusDayRepo.UpsertReported(txCtx, &activeModels.StudentStatusDay{
				StudentID:  studentID,
				Date:       d,
				Status:     activeModels.StudentStatusDaySick,
				ReportedAt: now,
				Source:     activeModels.StudentStatusSourceParent,
				Note:       notePtr,
			}); err != nil {
				return err
			}
		}

		if containsDate(normalized, today) {
			fresh, err := s.studentRepo.FindByIDForUpdate(txCtx, studentID)
			if err != nil {
				return err
			}
			applyLiveSickToday(fresh, now)
			if err := s.studentRepo.Update(txCtx, fresh); err != nil {
				return err
			}
		}

		rows, err := s.statusDayRepo.FindActiveByStudentAndDateRange(txCtx, studentID, minDate(normalized), maxDate(normalized))
		if err != nil {
			return err
		}
		// Return only the sick days the parent actually submitted. The range
		// query spans min..max, so for a non-contiguous submission (e.g. Mon
		// + Wed) it can also return an unrelated active row in between (a
		// Tuesday excused day) which must not be surfaced to the parent as a
		// sick day.
		dateSet := make(map[string]struct{}, len(normalized))
		for _, d := range normalized {
			dateSet[d.Format(dateKeyLayout)] = struct{}{}
		}
		filtered := make([]*activeModels.StudentStatusDay, 0, len(normalized))
		for _, r := range rows {
			if r.Status != activeModels.StudentStatusDaySick {
				continue
			}
			if _, ok := dateSet[timezone.DateOfUTC(r.Date).Format(dateKeyLayout)]; !ok {
				continue
			}
			filtered = append(filtered, r)
		}
		result = filtered

		capturedTenant := child.tenantID
		tenant.RegisterAfterCommit(txCtx, func() {
			s.broadcastStudentUpdated(capturedTenant, studentID)
		})
		return nil
	})
	if txErr != nil {
		return nil, fmt.Errorf("parent: submit sick note: %w", txErr)
	}

	s.logger.Info("parent submitted sick note",
		slog.Int64("account_id", accountID),
		slog.Int64("student_id", studentID),
		slog.Int64("tenant_id", child.tenantID),
		slog.Int("days", len(normalized)),
		slog.Bool("has_reason", notePtr != nil),
	)
	return result, nil
}

// ChildFeatures resolves the parent-portal feature toggles for the child's
// tenant after verifying ownership.
func (s *service) ChildFeatures(ctx context.Context, accountID, studentID int64) (ChildFeatureFlags, error) {
	child, err := s.resolveOwnedChild(ctx, accountID, studentID)
	if err != nil {
		return ChildFeatureFlags{}, err
	}
	sick, err := s.settings.ResolveBoolForTenant(ctx, child.tenantID, configModels.KeyParentSickNoteEnabled)
	if err != nil {
		return ChildFeatureFlags{}, fmt.Errorf("parent: resolve sick-note setting: %w", err)
	}
	notes, err := s.settings.ResolveBoolForTenant(ctx, child.tenantID, configModels.KeyParentNotesEnabled)
	if err != nil {
		return ChildFeatureFlags{}, fmt.Errorf("parent: resolve notes setting: %w", err)
	}
	return ChildFeatureFlags{SickNoteEnabled: sick, NotesEnabled: notes}, nil
}

// ListSickDays returns the child's active sick days in [from, to].
func (s *service) ListSickDays(ctx context.Context, accountID, studentID int64, from, to time.Time) ([]*activeModels.StudentStatusDay, error) {
	child, err := s.resolveOwnedChild(ctx, accountID, studentID)
	if err != nil {
		return nil, err
	}

	var out []*activeModels.StudentStatusDay
	txErr := tenant.WithTenantTx(ctx, s.db, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		rows, err := s.statusDayRepo.FindActiveByStudentAndDateRange(txCtx, studentID, from, to)
		if err != nil {
			return err
		}
		sick := make([]*activeModels.StudentStatusDay, 0, len(rows))
		for _, r := range rows {
			if r.Status == activeModels.StudentStatusDaySick {
				sick = append(sick, r)
			}
		}
		out = sick
		return nil
	})
	if txErr != nil {
		return nil, fmt.Errorf("parent: list sick days: %w", txErr)
	}
	return out, nil
}

// AddParentNote appends a note and returns the newest few.
func (s *service) AddParentNote(ctx context.Context, accountID, studentID int64, body string) ([]*usersModels.StudentParentNote, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return nil, ErrEmptyNote
	}
	// Characters (runes), not bytes — matches the frontend maxLength budget.
	if utf8.RuneCountInString(body) > maxParentNoteLen {
		return nil, ErrNoteTooLong
	}

	child, err := s.resolveOwnedChild(ctx, accountID, studentID)
	if err != nil {
		return nil, err
	}

	enabled, err := s.settings.ResolveBoolForTenant(ctx, child.tenantID, configModels.KeyParentNotesEnabled)
	if err != nil {
		return nil, fmt.Errorf("parent: resolve notes setting: %w", err)
	}
	if !enabled {
		return nil, ErrNotesDisabled
	}

	var notes []*usersModels.StudentParentNote
	txErr := tenant.WithTenantTx(ctx, s.db, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		note := &usersModels.StudentParentNote{
			StudentID:         studentID,
			GuardianAccountID: accountID,
			Body:              body,
		}
		note.SetTenantID(child.tenantID)
		if err := s.noteRepo.Create(txCtx, note); err != nil {
			return err
		}
		list, err := s.noteRepo.ListByStudent(txCtx, studentID, ParentNoteDisplayLimit)
		if err != nil {
			return err
		}
		notes = list
		return nil
	})
	if txErr != nil {
		return nil, fmt.Errorf("parent: add note: %w", txErr)
	}

	s.logger.Info("parent added note",
		slog.Int64("account_id", accountID),
		slog.Int64("student_id", studentID),
		slog.Int64("tenant_id", child.tenantID),
	)
	return notes, nil
}

// ListParentNotes returns the newest notes for the child (authorization
// only — not gated by the setting).
func (s *service) ListParentNotes(ctx context.Context, accountID, studentID int64, limit int) ([]*usersModels.StudentParentNote, error) {
	child, err := s.resolveOwnedChild(ctx, accountID, studentID)
	if err != nil {
		return nil, err
	}
	if limit <= 0 {
		limit = ParentNoteDisplayLimit
	}

	var notes []*usersModels.StudentParentNote
	txErr := tenant.WithTenantTx(ctx, s.db, child.tenantID, func(txCtx context.Context, _ bun.Tx) error {
		list, err := s.noteRepo.ListByStudent(txCtx, studentID, limit)
		if err != nil {
			return err
		}
		notes = list
		return nil
	})
	if txErr != nil {
		return nil, fmt.Errorf("parent: list notes: %w", txErr)
	}
	return notes, nil
}

// broadcastStudentUpdated fires a tenant-scoped student_updated SSE event
// so supervisors' live views refresh after a parent-side change. Mirrors
// the staff handler's broadcast; fire-and-forget.
func (s *service) broadcastStudentUpdated(tenantID, studentID int64) {
	if s.broadcaster == nil || tenantID <= 0 {
		return
	}
	source := activeModels.StudentStatusSourceParent
	event := realtime.NewEvent(realtime.EventStudentUpdated, "", realtime.EventData{Source: &source})
	if err := s.broadcaster.BroadcastToTenant(tenantID, event); err != nil {
		s.logger.Warn("parent: failed to broadcast student update",
			slog.Int64("tenant_id", tenantID),
			slog.Int64("student_id", studentID),
			slog.String("error", err.Error()),
		)
	}
}

func applyLiveSickToday(student *usersModels.Student, now time.Time) {
	trueVal := true
	falseVal := false
	student.Sick = &trueVal
	student.SickSince = &now
	student.Excused = &falseVal
	student.ExcusedSince = nil
}

func containsDate(dates []time.Time, needle time.Time) bool {
	needle = timezone.DateOfUTC(needle)
	for _, d := range dates {
		if timezone.DateOfUTC(d).Equal(needle) {
			return true
		}
	}
	return false
}

func minDate(dates []time.Time) time.Time {
	min := dates[0]
	for _, d := range dates[1:] {
		if d.Before(min) {
			min = d
		}
	}
	return min
}

func maxDate(dates []time.Time) time.Time {
	max := dates[0]
	for _, d := range dates[1:] {
		if d.After(max) {
			max = d
		}
	}
	return max
}
