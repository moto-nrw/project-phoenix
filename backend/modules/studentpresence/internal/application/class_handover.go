package application

import (
	"context"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ClassHandoversOn returns the handovers of date (#4794), each assembled with
// its absent children. Without a handover the absences are not read.
func (s *Service) ClassHandoversOn(ctx context.Context, date calendar.Date) (result []studentpresence.ClassHandover, err error) {
	if _, err := calendar.ParseDate(date.String()); err != nil {
		return nil, fmt.Errorf("%w: %w", studentpresence.ErrClassHandoverInvalid, err)
	}
	err = s.runWrite(ctx, "list_class_handovers", func(txCtx context.Context) (ports.Stats, error) {
		records, stats, err := s.store.ListClassHandovers(txCtx, date)
		if err != nil || len(records) == 0 {
			result = []studentpresence.ClassHandover{}
			return stats, err
		}
		ids := make([]int64, 0, len(records))
		for _, record := range records {
			ids = append(ids, record.ID)
		}
		absent, absenceStats, err := s.store.ListClassHandoverAbsences(txCtx, ids)
		stats.Queries += absenceStats.Queries
		stats.Rows += absenceStats.Rows
		stats.StatementDuration += absenceStats.StatementDuration
		result = make([]studentpresence.ClassHandover, 0, len(records))
		for _, record := range records {
			handover := record.ClassHandover
			handover.AbsentStudentIDs = absent[record.ID]
			if handover.AbsentStudentIDs == nil {
				handover.AbsentStudentIDs = []int64{}
			}
			result = append(result, handover)
		}
		return stats, err
	})
	return result, err
}

// ChangeClassHandover stores the handover of one class (#4794), replacing an
// earlier one of the same class and date with the clock's now as its
// moment, or deletes it on Withdraw. It never touches the morning attendance
// or the OGS attendance.
func (s *Service) ChangeClassHandover(ctx context.Context, in studentpresence.ClassHandoverChange) (existed bool, err error) {
	class := strings.ToLower(strings.TrimSpace(in.SchoolClass))
	if _, dateErr := calendar.ParseDate(in.Date.String()); class == "" || dateErr != nil {
		return false, studentpresence.ErrClassHandoverInvalid
	}
	if !in.Withdraw && (in.HandedOverBy <= 0 || !validIDs(in.AbsentStudentIDs)) {
		return false, studentpresence.ErrClassHandoverInvalid
	}
	in.SchoolClass = strings.TrimSpace(in.SchoolClass)
	in.AbsentStudentIDs = distinctIDs(in.AbsentStudentIDs)
	err = s.runWrite(ctx, "change_class_handover", func(txCtx context.Context) (ports.Stats, error) {
		if in.Withdraw {
			stats, err := s.store.DeleteClassHandover(txCtx, class, in.Date)
			existed = stats.Rows > 0
			return stats, err
		}
		var stats ports.Stats
		existed, stats, err = s.store.ReplaceClassHandover(txCtx, in, class, s.now())
		return stats, err
	})
	return existed, err
}

// distinctIDs keeps the first occurrence of each ID.
func distinctIDs(ids []int64) []int64 {
	seen := make(map[int64]bool, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}
