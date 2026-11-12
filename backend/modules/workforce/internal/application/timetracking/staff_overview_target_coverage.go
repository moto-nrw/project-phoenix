package timetracking

import (
	"context"
	"fmt"

	"github.com/moto-ogs/project-phoenix/modules/workforce"
)

// TargetCoverage runs the overview's prefetch and per-staff month math and
// counts the months whose HasTarget is false, so the payroll check and the
// Zeitkonten hint can never disagree (#6890). It skips the overview's
// vacation, hint and sort work, which a count does not need. Year and month
// both zero mean the current month; failures carry the public kinds.
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
	coverage := &workforce.TargetCoverage{Year: key.Year, Month: key.Month}

	staffMembers, err := s.activeStaff(ctx)
	if err != nil {
		return nil, err
	}
	coverage.StaffTotal = len(staffMembers)
	if len(staffMembers) == 0 {
		return coverage, nil
	}

	prefetch, err := s.buildPrefetch(ctx, staffMembers, nil, key)
	if err != nil {
		return nil, err
	}
	monthSvc := newPrefetchedMonthService(prefetch, s.logger)
	for _, staff := range staffMembers {
		summary, err := monthSvc.GetMonthSummary(ctx, staff.ID, key.Year, key.Month)
		if err != nil {
			return nil, fmt.Errorf("failed to compute month summary for staff %d: %w", staff.ID, err)
		}
		if !summary.HasTarget {
			coverage.StaffWithoutTarget++
		}
	}
	return coverage, nil
}
