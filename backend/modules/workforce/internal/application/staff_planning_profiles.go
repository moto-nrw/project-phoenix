package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
)

// StaffPlanningProfiles owns validation, the RLS unit, observation and public
// mapping together. The engine exposes this complete query by embedding it.
func (s *Service) StaffPlanningProfiles(ctx context.Context, staffIDs []int64, from, to string) ([]workforce.StaffPlanningProfile, error) {
	if err := domain.ValidateDate(from, "from"); err != nil {
		return nil, MapError(err)
	}
	if err := domain.ValidateDate(to, "to"); err != nil {
		return nil, MapError(err)
	}
	var values []domain.StaffPlanningProfile
	err := s.run(ctx, "staff_planning_profiles", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		var queryErr error
		values, queryStats, queryErr = s.store.StaffPlanningProfiles(ctx, staffIDs, from, to)
		stats.Add(queryStats)
		return queryErr
	})
	if err != nil {
		return nil, MapError(err)
	}
	result := make([]workforce.StaffPlanningProfile, 0, len(values))
	for _, value := range values {
		result = append(result, workforce.StaffPlanningProfile{
			MembershipID: value.MembershipID, WorkTimeModelID: value.WorkTimeModelID,
			RotationAnchorDate: value.RotationAnchorDate, Schedules: schedulesToPublic(value.Schedules),
		})
	}
	return result, nil
}
