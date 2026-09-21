package presence

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence/legacy/models/active"
)

// GetCrossTenantStudents returns students visiting from other tenants.
func (s *service) GetCrossTenantStudents(ctx context.Context, hostingTenantID int64) ([]active.CrossTenantStudent, error) {
	if s.CrossTenantRepo == nil {
		return []active.CrossTenantStudent{}, nil
	}

	students, err := s.CrossTenantRepo.FindCrossTenantStudents(ctx, hostingTenantID)
	if err != nil {
		return nil, &ActiveError{Op: "GetCrossTenantStudents", Err: fmt.Errorf("query failed: %w", err)}
	}
	if len(students) > 0 {
		if err := s.attachHomeSchools(ctx, students); err != nil {
			return nil, err
		}
	}

	s.getLogger().Info("cross-tenant students queried",
		slog.Int64("hosting_tenant_id", hostingTenantID),
		slog.Int("count", len(students)),
	)

	return students, nil
}

// attachHomeSchools names each visiting student's home school by its slug.
func (s *service) attachHomeSchools(ctx context.Context, students []active.CrossTenantStudent) error {
	if s.Schools == nil {
		return &ActiveError{Op: "GetCrossTenantStudents", Err: errors.New("school query is required")}
	}
	ids := make([]int64, 0, len(students))
	seen := make(map[int64]struct{}, len(students))
	for _, student := range students {
		if _, found := seen[student.HomeTenantID]; !found {
			seen[student.HomeTenantID] = struct{}{}
			ids = append(ids, student.HomeTenantID)
		}
	}
	schools, schoolErr := s.Schools.ListSchoolsByID(ctx, ids)
	if schoolErr != nil {
		return &ActiveError{Op: "GetCrossTenantStudents", Err: fmt.Errorf("load home schools: %w", schoolErr)}
	}
	slugs := make(map[int64]string, len(schools))
	for _, school := range schools {
		slugs[school.ID] = school.Slug
	}
	for index := range students {
		slug, found := slugs[students[index].HomeTenantID]
		if !found {
			return &ActiveError{Op: "GetCrossTenantStudents", Err: fmt.Errorf("home school %d not found", students[index].HomeTenantID)}
		}
		students[index].HomeTenant = slug
	}
	return nil
}
