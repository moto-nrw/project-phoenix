package application

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/workforce"
	"github.com/moto-nrw/project-phoenix/modules/workforce/internal/domain"
)

func (s *Service) StaffPlanningProfiles(ctx context.Context, staffIDs []int64, from, to string) (result []domain.StaffPlanningProfile, err error) {
	if err = domain.ValidateDate(from, "from"); err != nil {
		return nil, err
	}
	if err = domain.ValidateDate(to, "to"); err != nil {
		return nil, err
	}
	err = s.run(ctx, "staff_planning_profiles", func(ctx context.Context, stats *domain.OperationStats) error {
		var queryStats domain.OperationStats
		result, queryStats, err = s.store.StaffPlanningProfiles(ctx, staffIDs, from, to)
		stats.Add(queryStats)
		return err
	})
	return result, err
}

func (e Engine) StaffPlanningProfiles(ctx context.Context, staffIDs []int64, from, to string) ([]workforce.StaffPlanningProfile, error) {
	values, err := e.service.StaffPlanningProfiles(ctx, staffIDs, from, to)
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
