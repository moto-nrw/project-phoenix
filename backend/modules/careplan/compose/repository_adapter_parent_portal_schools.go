package compose

import (
	"context"
	"fmt"
	"slices"
	"strings"

	parentModels "github.com/moto-nrw/project-phoenix/modules/careplan/internal/rows"
)

// ParentPortalSchool is the Organisation & Tenancy school fact the guardian
// portal shows and filters by.
type ParentPortalSchool struct {
	ID        int64
	Name      string
	Slug      string
	Subdomain string
	Active    bool
	Hidden    bool
	Deleted   bool
}

// ParentPortalSchools resolves the schools with the given IDs; an unknown
// school is absent.
type ParentPortalSchools interface {
	ListParentPortalSchools(ctx context.Context, ids []int64) ([]ParentPortalSchool, error)
}

// WithSchools returns the repositories with the school names and the school
// eligibility resolved through schools.
func (r ParentPortalRepositories) WithSchools(schools ParentPortalSchools) ParentPortalRepositories {
	if schools == nil {
		panic("parent portal repositories: school directory is required")
	}
	return ParentPortalRepositories{
		ParentChild:             schoolChildRepository{ChildRepository: r.ParentChild, schools: schools},
		ParentEnrollablePhase:   schoolEnrollablePhaseRepository{EnrollablePhaseRepository: r.ParentEnrollablePhase, schools: schools},
		ParentEnrollmentRequest: schoolEnrollmentRequestRepository{EnrollmentRequestRepository: r.ParentEnrollmentRequest, schools: schools},
	}
}

type schoolChildRepository struct {
	parentModels.ChildRepository
	schools ParentPortalSchools
}

func (r schoolChildRepository) ListByAccount(ctx context.Context, accountID int64) ([]*parentModels.ChildSummary, error) {
	children, err := r.ChildRepository.ListByAccount(ctx, accountID)
	if err != nil {
		return nil, err
	}
	if err := enrichChildrenWithSchools(ctx, r.schools, children); err != nil {
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

func (r schoolChildRepository) FindForAccount(ctx context.Context, accountID, studentID int64) (*parentModels.ChildSummary, error) {
	child, err := r.ChildRepository.FindForAccount(ctx, accountID, studentID)
	if err != nil || child == nil {
		return child, err
	}
	if err := enrichChildrenWithSchools(ctx, r.schools, []*parentModels.ChildSummary{child}); err != nil {
		return nil, err
	}
	return child, nil
}

type schoolEnrollablePhaseRepository struct {
	parentModels.EnrollablePhaseRepository
	schools ParentPortalSchools
}

func (r schoolEnrollablePhaseRepository) ListEnrollable(ctx context.Context, accountID int64) ([]*parentModels.EnrollablePhase, error) {
	phases, err := r.EnrollablePhaseRepository.ListEnrollable(ctx, accountID)
	if err != nil || len(phases) == 0 {
		return phases, err
	}
	schools, err := parentPortalSchoolsByID(ctx, r.schools, phaseSchoolIDs(phases))
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
	slices.SortStableFunc(result, compareEnrollablePhases)
	return result, nil
}

func compareEnrollablePhases(left, right *parentModels.EnrollablePhase) int {
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

type schoolEnrollmentRequestRepository struct {
	parentModels.EnrollmentRequestRepository
	schools ParentPortalSchools
}

func (r schoolEnrollmentRequestRepository) ListByAccount(ctx context.Context, accountID int64) ([]*parentModels.EnrollmentRequestSummary, error) {
	requests, err := r.EnrollmentRequestRepository.ListByAccount(ctx, accountID)
	if err != nil || len(requests) == 0 {
		return requests, err
	}
	ids := make([]int64, 0, len(requests))
	for _, request := range requests {
		ids = append(ids, request.TenantID)
	}
	schools, err := parentPortalSchoolsByID(ctx, r.schools, ids)
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

func enrichChildrenWithSchools(ctx context.Context, directory ParentPortalSchools, children []*parentModels.ChildSummary) error {
	ids := make([]int64, 0, len(children))
	for _, child := range children {
		ids = append(ids, child.TenantID)
	}
	schools, err := parentPortalSchoolsByID(ctx, directory, ids)
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

func phaseSchoolIDs(phases []*parentModels.EnrollablePhase) []int64 {
	ids := make([]int64, 0, len(phases))
	for _, phase := range phases {
		ids = append(ids, phase.SchoolID)
	}
	return ids
}

func parentPortalSchoolsByID(ctx context.Context, directory ParentPortalSchools, ids []int64) (map[int64]ParentPortalSchool, error) {
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
	result := make(map[int64]ParentPortalSchool, len(values))
	for _, value := range values {
		result[value.ID] = value
	}
	return result, nil
}
