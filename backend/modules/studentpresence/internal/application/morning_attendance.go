package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// MorningAttendanceOn returns the stored morning marks of date (#4790) for
// the named children.
func (s *Service) MorningAttendanceOn(ctx context.Context, date calendar.Date, children []studentpresence.MorningAttendanceChild) (result []studentpresence.MorningAttendanceEntry, err error) {
	if _, err := calendar.ParseDate(date.String()); err != nil {
		return nil, fmt.Errorf("%w: %w", studentpresence.ErrMorningAttendanceInvalid, err)
	}
	studentIDs, entryIDs := make([]int64, 0, len(children)), make([]int64, 0)
	for _, child := range children {
		if !validMorningAttendanceChild(child) {
			return nil, studentpresence.ErrMorningAttendanceInvalid
		}
		if child.StudentID > 0 {
			studentIDs = append(studentIDs, child.StudentID)
		} else {
			entryIDs = append(entryIDs, child.ClassListEntryID)
		}
	}
	if len(studentIDs) == 0 && len(entryIDs) == 0 {
		return []studentpresence.MorningAttendanceEntry{}, nil
	}
	err = s.runWrite(ctx, "list_morning_attendance", func(txCtx context.Context) (ports.Stats, error) {
		var stats ports.Stats
		result, stats, err = s.store.ListMorningAttendance(txCtx, date, studentIDs, entryIDs)
		return stats, err
	})
	return result, err
}

// RecordMorningAttendance stores the marks of one school morning (#4790). It
// never touches active.attendance: the OGS check-ins stay as they are.
func (s *Service) RecordMorningAttendance(ctx context.Context, in studentpresence.MorningAttendanceWrite) error {
	if _, err := calendar.ParseDate(in.Date.String()); err != nil || in.RecordedBy <= 0 || len(in.Marks) == 0 {
		return studentpresence.ErrMorningAttendanceInvalid
	}
	// A child named twice keeps its last mark: one statement cannot update
	// the same row twice.
	marks := make([]studentpresence.MorningAttendanceMark, 0, len(in.Marks))
	position := make(map[studentpresence.MorningAttendanceChild]int, len(in.Marks))
	for _, mark := range in.Marks {
		if !validMorningAttendanceChild(mark.Child) {
			return studentpresence.ErrMorningAttendanceInvalid
		}
		if mark.Status != studentpresence.MorningAttendancePresent && mark.Status != studentpresence.MorningAttendanceAbsent {
			return studentpresence.ErrMorningAttendanceInvalid
		}
		if i, seen := position[mark.Child]; seen {
			marks[i] = mark
			continue
		}
		position[mark.Child] = len(marks)
		marks = append(marks, mark)
	}
	return s.runWrite(ctx, "record_morning_attendance", func(txCtx context.Context) (ports.Stats, error) {
		return s.store.UpsertMorningAttendance(txCtx, in.Date, in.RecordedBy, marks)
	})
}

// DeleteExpiredMorningAttendance deletes the morning marks dated before the
// first Berlin calendar day a window of retentionDays keeps (#4790), and the
// class handovers of those days (#4794): both record which children were not
// at school in the morning. It joins the school's cleanup transaction and
// never opens its own, so the deletion commits with its deletion-log entry;
// the count covers both.
func (s *Service) DeleteExpiredMorningAttendance(ctx context.Context, retentionDays int) (deleted int, err error) {
	if retentionDays <= 0 {
		return 0, fmt.Errorf("%w, got %d", studentpresence.ErrAttendanceRetentionWindow, retentionDays)
	}
	cutoff := calendar.TodayDate().AddDays(-retentionDays)
	err = s.run("delete_morning_attendance_before", func() (ports.Stats, error) {
		if err := s.tx.Require(ctx); err != nil {
			return ports.Stats{}, err
		}
		stats, err := s.store.DeleteMorningAttendanceBefore(ctx, cutoff)
		if err != nil {
			return stats, err
		}
		handovers, err := s.store.DeleteClassHandoversBefore(ctx, cutoff)
		stats.Queries += handovers.Queries
		stats.StatementDuration += handovers.StatementDuration
		stats.Rows += handovers.Rows
		deleted = int(stats.Rows)
		return stats, err
	})
	return deleted, err
}

// validMorningAttendanceChild accepts exactly one positive child ID.
func validMorningAttendanceChild(child studentpresence.MorningAttendanceChild) bool {
	switch {
	case child.StudentID > 0 && child.ClassListEntryID == 0:
		return true
	case child.StudentID == 0 && child.ClassListEntryID > 0:
		return true
	default:
		return false
	}
}
