package application

import (
	"context"
	"fmt"
	"slices"
	"strings"

	parentModels "github.com/moto-nrw/project-phoenix/modules/careplan/internal/rows"
	carePlanBindingPorts "github.com/moto-nrw/project-phoenix/modules/careplan/ports"
)

// WithSchools returns the repositories with the school names and the school
// eligibility resolved through schools.
func (r ParentPortalRepositories) WithSchools(schools carePlanBindingPorts.ParentPortalSchools) ParentPortalRepositories {
	if schools == nil {
		panic("parent portal repositories: school directory is required")
	}
	return ParentPortalRepositories{
		ParentChild:             SchoolChildRepository{ChildRepository: r.ParentChild, schools: schools},
		ParentEnrollablePhase:   SchoolEnrollablePhaseRepository{EnrollablePhaseRepository: r.ParentEnrollablePhase, schools: schools},
		ParentEnrollmentRequest: SchoolEnrollmentRequestRepository{EnrollmentRequestRepository: r.ParentEnrollmentRequest, schools: schools},
	}
}

type SchoolChildRepository struct {
	parentModels.ChildRepository
	schools carePlanBindingPorts.ParentPortalSchools
}

func (r SchoolChildRepository) ListByAccount(ctx context.Context, accountID int64) ([]*parentModels.ChildSummary, error) {
	children, err := r.ChildRepository.ListByAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if err := EnrichChildrenWithSchools(ctx, r.schools, children); err != nil {
		return nil, err
	}
	slices.SortStableFunc(children, func(left, right *parentModels.ChildSummary) int {
		if order := strings.Compare(left.SchoolName, right.SchoolName); order != 0 {
			return order
		}
		if order := strings.Compare(left.FirstName, right.FirstName); order != 0 {
			return order
		}
		return strings.Compare(left.LastName, right.LastName)
	})
	return children, nil
}
func (r SchoolChildRepository) FindForAccount(ctx context.Context, accountID, studentID int64) (*parentModels.ChildSummary, error) {
	child, err := r.ChildRepository.FindForAccount(ctx, accountID, studentID)
	if err != nil || child == nil {
		return child, err
	}
	if err := EnrichChildrenWithSchools(ctx, r.schools, []*parentModels.ChildSummary{child}); err != nil {
		return nil, err
	}
	return child, nil
}

type SchoolEnrollablePhaseRepository struct {
	parentModels.EnrollablePhaseRepository
	schools carePlanBindingPorts.ParentPortalSchools
}

func (r SchoolEnrollablePhaseRepository) ListEnrollable(ctx context.Context, accountID int64) ([]*parentModels.EnrollablePhase, error) {
	phases, err := r.EnrollablePhaseRepository.ListEnrollable(ctx, accountID)
	if err != nil || len(phases) == 0 {
		return phases, err
	}
	schools, err := ParentPortalSchoolsByID(ctx, r.schools, PhaseSchoolIDs(phases))
	if err != nil {
		return nil, fmt.Errorf("parent: load schools for enrollable phases: %w", err)
	}
	result := make([]*parentModels.EnrollablePhase, 0, len(phases))
	for _, phase := range phases {
		school, found := schools[phase.SchoolID]
		if !found {
			return nil, fmt.Errorf("parent: school %d missing for enrollable phase", phase.SchoolID)
		}
		if !school.Active || school.Deleted || (school.Hidden && !phase.HasFamilyLink) {
			continue
		}
		phase.SchoolName = school.Name
		phase.SchoolSlug = school.Slug
		phase.SchoolSubdomain = school.Subdomain
		result = append(result, phase)
	}
	slices.SortStableFunc(result, CompareEnrollablePhases)
	return result, nil
}
func CompareEnrollablePhases(left, right *parentModels.EnrollablePhase) int {
	if left.AlreadyLinked != right.AlreadyLinked {
		if left.AlreadyLinked {
			return -1
		}
		return 1
	}
	if order := strings.Compare(left.SchoolName, right.SchoolName); order != 0 {
		return order
	}
	return left.ServiceStartDate.Compare(right.ServiceStartDate)
}

type SchoolEnrollmentRequestRepository struct {
	parentModels.EnrollmentRequestRepository
	schools carePlanBindingPorts.ParentPortalSchools
}

func (r SchoolEnrollmentRequestRepository) ListByAccount(ctx context.Context, accountID int64) ([]*parentModels.EnrollmentRequestSummary, error) {
	requests, err := r.EnrollmentRequestRepository.ListByAccount(ctx, accountID)
	if err != nil || len(requests) == 0 {
		return requests, err
	}
	ids := make([]int64, 0, len(requests))
	for _, request := range requests {
		ids = append(ids, request.TenantID)
	}
	schools, err := ParentPortalSchoolsByID(ctx, r.schools, ids)
	if err != nil {
		return nil, fmt.Errorf("parent: load schools for enrollment requests: %w", err)
	}
	result := make([]*parentModels.EnrollmentRequestSummary, 0, len(requests))
	for _, request := range requests {
		school, found := schools[request.TenantID]
		if !found {
			return nil, fmt.Errorf("parent: school %d missing for enrollment request", request.TenantID)
		}
		if school.Deleted {
			continue
		}
		request.SchoolName = school.Name
		request.SchoolSlug = school.Slug
		result = append(result, request)
	}
	return result, nil
}
func EnrichChildrenWithSchools(ctx context.Context, directory carePlanBindingPorts.ParentPortalSchools, children []*parentModels.ChildSummary) error {
	ids := make([]int64, 0, len(children))
	for _, child := range children {
		ids = append(ids, child.TenantID)
	}
	schools, err := ParentPortalSchoolsByID(ctx, directory, ids)
	if err != nil {
		return fmt.Errorf("parent: load schools for children: %w", err)
	}
	for _, child := range children {
		school, found := schools[child.TenantID]
		if !found {
			return fmt.Errorf("parent: school %d missing for child", child.TenantID)
		}
		child.SchoolName = school.Name
		child.SchoolSlug = school.Slug
	}
	return nil
}
func PhaseSchoolIDs(phases []*parentModels.EnrollablePhase) []int64 {
	ids := make([]int64, 0, len(phases))
	for _, phase := range phases {
		ids = append(ids, phase.SchoolID)
	}
	return ids
}
func ParentPortalSchoolsByID(ctx context.Context, directory carePlanBindingPorts.ParentPortalSchools, ids []int64) (map[int64]carePlanBindingPorts.ParentPortalSchool, error) {
	unique := make([]int64, 0, len(ids))
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if _, found := seen[id]; !found {
			seen[id] = struct{}{}
			unique = append(unique, id)
		}
	}
	values, err := directory.ListParentPortalSchools(ctx, unique)
	if err != nil {
		return nil, err
	}
	result := make(map[int64]carePlanBindingPorts.ParentPortalSchool, len(values))
	for _, value := range values {
		result[value.ID] = value
	}
	return result, nil
}
