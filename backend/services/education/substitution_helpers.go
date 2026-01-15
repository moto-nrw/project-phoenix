package education

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	"github.com/moto-nrw/project-phoenix/internal/core/domain/education"
)

// Substitution operations

// CreateSubstitution creates a new substitution
func (s *service) CreateSubstitution(ctx context.Context, substitution *education.GroupSubstitution) error {
	// Validate substitution data
	if err := substitution.Validate(); err != nil {
		return &EducationError{Op: "CreateSubstitution", Err: err}
	}

	// Validate no backdating - start date must be today or in the future
	today := time.Now().Truncate(24 * time.Hour)
	if substitution.StartDate.Before(today) {
		return &EducationError{Op: "CreateSubstitution", Err: ErrSubstitutionBackdated}
	}

	// Verify group exists
	_, err := s.groupRepo.FindByID(ctx, substitution.GroupID)
	if err != nil {
		return &EducationError{Op: "CreateSubstitution", Err: ErrGroupNotFound}
	}

	// Verify regular staff exists - only if RegularStaffID is provided
	if substitution.RegularStaffID != nil {
		_, err = s.staffRepo.FindByID(ctx, *substitution.RegularStaffID)
		if err != nil {
			return &EducationError{Op: "CreateSubstitution", Err: ErrTeacherNotFound}
		}
	}

	// Verify substitute staff exists
	_, err = s.staffRepo.FindByID(ctx, substitution.SubstituteStaffID)
	if err != nil {
		return &EducationError{Op: "CreateSubstitution", Err: ErrTeacherNotFound}
	}

	// Note: We intentionally allow staff members to have multiple overlapping substitutions.
	// This enables a staff member to supervise multiple groups simultaneously.

	// Create the substitution
	if err := s.substitutionRepo.Create(ctx, substitution); err != nil {
		return &EducationError{Op: "CreateSubstitution", Err: err}
	}

	return nil
}

// UpdateSubstitution updates an existing substitution
func (s *service) UpdateSubstitution(ctx context.Context, substitution *education.GroupSubstitution) error {
	// Validate substitution data
	if err := substitution.Validate(); err != nil {
		return &EducationError{Op: "UpdateSubstitution", Err: err}
	}

	// Validate no backdating - start date must be today or in the future
	today := time.Now().Truncate(24 * time.Hour)
	if substitution.StartDate.Before(today) {
		return &EducationError{Op: "UpdateSubstitution", Err: ErrSubstitutionBackdated}
	}

	// Verify substitution exists
	_, err := s.substitutionRepo.FindByID(ctx, substitution.ID)
	if err != nil {
		return &EducationError{Op: "UpdateSubstitution", Err: ErrSubstitutionNotFound}
	}

	// Verify group exists
	_, err = s.groupRepo.FindByID(ctx, substitution.GroupID)
	if err != nil {
		return &EducationError{Op: "UpdateSubstitution", Err: ErrGroupNotFound}
	}

	// Verify regular staff exists - only if RegularStaffID is provided
	if substitution.RegularStaffID != nil {
		_, err = s.staffRepo.FindByID(ctx, *substitution.RegularStaffID)
		if err != nil {
			return &EducationError{Op: "UpdateSubstitution", Err: ErrTeacherNotFound}
		}
	}

	// Verify substitute staff exists
	_, err = s.staffRepo.FindByID(ctx, substitution.SubstituteStaffID)
	if err != nil {
		return &EducationError{Op: "UpdateSubstitution", Err: ErrTeacherNotFound}
	}

	// Check for conflicting substitutions (excluding this one)
	conflicts, err := s.substitutionRepo.FindOverlapping(ctx, substitution.SubstituteStaffID,
		substitution.StartDate, substitution.EndDate)
	if err == nil {
		for _, conflict := range conflicts {
			if conflict.ID != substitution.ID {
				return &EducationError{Op: "UpdateSubstitution", Err: ErrSubstitutionConflict}
			}
		}
	}

	// Update the substitution
	if err := s.substitutionRepo.Update(ctx, substitution); err != nil {
		return &EducationError{Op: "UpdateSubstitution", Err: err}
	}

	return nil
}

// DeleteSubstitution deletes a substitution by ID
func (s *service) DeleteSubstitution(ctx context.Context, id int64) error {
	// Verify substitution exists
	_, err := s.substitutionRepo.FindByID(ctx, id)
	if err != nil {
		return &EducationError{Op: "DeleteSubstitution", Err: ErrSubstitutionNotFound}
	}

	// Delete the substitution
	if err := s.substitutionRepo.Delete(ctx, id); err != nil {
		return &EducationError{Op: "DeleteSubstitution", Err: err}
	}

	return nil
}

// GetSubstitution retrieves a substitution by ID
func (s *service) GetSubstitution(ctx context.Context, id int64) (*education.GroupSubstitution, error) {
	substitution, err := s.substitutionRepo.FindByIDWithRelations(ctx, id)
	if err != nil {
		return nil, &EducationError{Op: "GetSubstitution", Err: ErrSubstitutionNotFound}
	}
	return substitution, nil
}

// ListSubstitutions retrieves substitutions with optional filtering
func (s *service) ListSubstitutions(ctx context.Context, options *base.QueryOptions) ([]*education.GroupSubstitution, error) {
	// Now using the modern ListWithOptions method with relations loaded
	substitutions, err := s.substitutionRepo.ListWithRelations(ctx, options)
	if err != nil {
		return nil, &EducationError{Op: "ListSubstitutions", Err: err}
	}
	return substitutions, nil
}

// GetActiveSubstitutions gets all active substitutions for a specific date
func (s *service) GetActiveSubstitutions(ctx context.Context, date time.Time) ([]*education.GroupSubstitution, error) {
	substitutions, err := s.substitutionRepo.FindActiveWithRelations(ctx, date)
	if err != nil {
		return nil, &EducationError{Op: "GetActiveSubstitutions", Err: err}
	}
	return substitutions, nil
}

// GetActiveGroupSubstitutions gets active substitutions for a specific group and date
func (s *service) GetActiveGroupSubstitutions(ctx context.Context, groupID int64, date time.Time) ([]*education.GroupSubstitution, error) {
	// Verify group exists
	_, err := s.groupRepo.FindByID(ctx, groupID)
	if err != nil {
		return nil, &EducationError{Op: "GetActiveGroupSubstitutions", Err: ErrGroupNotFound}
	}

	substitutions, err := s.substitutionRepo.FindActiveByGroupWithRelations(ctx, groupID, date)
	if err != nil {
		return nil, &EducationError{Op: "GetActiveGroupSubstitutions", Err: err}
	}
	return substitutions, nil
}

// GetStaffSubstitutions gets all substitutions for a staff member
func (s *service) GetStaffSubstitutions(ctx context.Context, staffID int64, asRegular bool) ([]*education.GroupSubstitution, error) {
	// Verify staff exists
	_, err := s.staffRepo.FindByID(ctx, staffID)
	if err != nil {
		return nil, &EducationError{Op: "GetStaffSubstitutions", Err: ErrTeacherNotFound}
	}

	var substitutions []*education.GroupSubstitution
	var repoErr error

	if asRegular {
		substitutions, repoErr = s.substitutionRepo.FindByRegularStaff(ctx, staffID)
	} else {
		substitutions, repoErr = s.substitutionRepo.FindBySubstituteStaff(ctx, staffID)
	}

	if repoErr != nil {
		return nil, &EducationError{Op: "GetStaffSubstitutions", Err: repoErr}
	}

	return substitutions, nil
}

// CheckSubstitutionConflicts checks for conflicting substitutions for a staff member
func (s *service) CheckSubstitutionConflicts(ctx context.Context, staffID int64, startDate, endDate time.Time) ([]*education.GroupSubstitution, error) {
	// Verify staff exists
	_, err := s.staffRepo.FindByID(ctx, staffID)
	if err != nil {
		return nil, &EducationError{Op: "CheckSubstitutionConflicts", Err: ErrTeacherNotFound}
	}

	// Validate date range
	if startDate.After(endDate) {
		return nil, &EducationError{Op: "CheckSubstitutionConflicts", Err: ErrInvalidDateRange}
	}

	// Check for conflicts
	conflicts, err := s.substitutionRepo.FindOverlapping(ctx, staffID, startDate, endDate)
	if err != nil {
		return nil, &EducationError{Op: "CheckSubstitutionConflicts", Err: err}
	}

	return conflicts, nil
}
