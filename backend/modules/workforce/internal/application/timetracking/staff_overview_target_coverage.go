package timetracking

import (
	"context"
	"fmt"

	"github.com/moto-ogs/project-phoenix/modules/workforce"
)

// TargetCoverage runs the overview's prefetch and per-staff month math and
// counts the months whose HasTarget is false, so the payroll check and the
// Zeitkonten hint can never disagree (#6890). It skips the overview's
// vacation, hint and sort work, which a count does not need. Failures carry
// the public kinds.
//
// Year and month both zero select the payroll month: the previous month
// while it is not closed for every staff member (the Monatsabschluss that
// comes next), otherwise the current month. A previous month before the
// account start has nothing to close and is skipped.
func (s *staffOverviewService) TargetCoverage(ctx context.Context, year, month int) (*workforce.TargetCoverage, error) {
	coverage, err := s.countTargetCoverage(ctx, year, month)
	if err != nil {
		return nil, mapTimeTrackingFailure(err)
	}
	return coverage, nil
}

func (s *staffOverviewService) countTargetCoverage(ctx context.Context, year, month int) (*workforce.TargetCoverage, error) {
	key, err := resolveOverviewMonth(OverviewFilters{Year: year, Month: month}, s.today())
	if err != nil {
		return nil, err
	}
	staffMembers, err := s.activeStaff(ctx)
	if err != nil {
		return nil, err
	}
	if len(staffMembers) == 0 {
		return &workforce.TargetCoverage{Year: key.Year, Month: key.Month}, nil
	}

	// One prefetch through the requested (or current) month also covers the
	// previous month: its window starts at the account anchor.
	prefetch, err := s.buildPrefetch(ctx, staffMembers, nil, key)
	if err != nil {
		return nil, err
	}
	monthSvc := newPrefetchedMonthService(prefetch, s.logger)
	if year == 0 && month == 0 {
		previous := key.addMonths(-1)
		if !previous.lastDay().Before(prefetch.from) {
			coverage, open, err := s.monthTargetCoverage(ctx, monthSvc, staffMembers, previous)
			if err != nil || open {
				return coverage, err
			}
		}
	}
	coverage, _, err := s.monthTargetCoverage(ctx, monthSvc, staffMembers, key)
	return coverage, err
}

// monthTargetCoverage counts one month and reports whether any staff member
// still has it open (not closed).
func (s *staffOverviewService) monthTargetCoverage(ctx context.Context, monthSvc WorkTimeMonthService, staffMembers []OverviewStaff, key monthKey) (*workforce.TargetCoverage, bool, error) {
	coverage := &workforce.TargetCoverage{Year: key.Year, Month: key.Month, StaffTotal: len(staffMembers)}
	open := false
	for _, staff := range staffMembers {
		summary, err := monthSvc.GetMonthSummary(ctx, staff.ID, key.Year, key.Month)
		if err != nil {
			return nil, false, fmt.Errorf("failed to compute month summary for staff %d: %w", staff.ID, err)
		}
		if !summary.HasTarget {
			coverage.StaffWithoutTarget++
		}
		open = open || !summary.IsClosed
	}
	return coverage, open, nil
}
