package education

import (
	"context"
	"strings"

	"github.com/moto-nrw/project-phoenix/internal/schoolclass"
	"github.com/moto-nrw/project-phoenix/models/education"
)

// GetStaffSchoolClasses returns the school classes assigned to a staff
// member, in class order, as the display strings they were entered with.
func (s *service) GetStaffSchoolClasses(ctx context.Context, staffID int64) ([]string, error) {
	if _, err := s.staffRepo.FindByID(ctx, staffID); err != nil {
		return nil, &EducationError{Op: "GetStaffSchoolClasses", Err: ErrStaffNotFound}
	}

	assignments, err := s.classTeacherRepo.FindByStaff(ctx, staffID)
	if err != nil {
		return nil, &EducationError{Op: "GetStaffSchoolClasses", Err: err}
	}

	classes := make([]string, 0, len(assignments))
	for _, assignment := range assignments {
		classes = append(classes, assignment.SchoolClass)
	}
	return classes, nil
}

// SetStaffSchoolClasses replaces the staff member's class assignments with
// the submitted set. Classes are compared via schoolclass.Normalize, so "1a"
// and " 1A " count as the same assignment; unchanged rows are kept (diff, not
// delete-all) so their IDs and timestamps survive a resubmit of the same set.
// Class names are deliberately NOT validated against the current student
// classes: assignments may be created before students are imported.
func (s *service) SetStaffSchoolClasses(ctx context.Context, staffID int64, classes []string) error {
	if _, err := s.staffRepo.FindByID(ctx, staffID); err != nil {
		return &EducationError{Op: "SetStaffSchoolClasses", Err: ErrStaffNotFound}
	}

	wanted, err := dedupeSchoolClasses(classes)
	if err != nil {
		return &EducationError{Op: "SetStaffSchoolClasses", Err: err}
	}

	current, err := s.classTeacherRepo.FindByStaff(ctx, staffID)
	if err != nil {
		return &EducationError{Op: "SetStaffSchoolClasses", Err: err}
	}

	currentByKey := make(map[string]int64, len(current))
	for _, assignment := range current {
		currentByKey[schoolclass.Normalize(assignment.SchoolClass)] = assignment.ID
	}

	for key, id := range currentByKey {
		if _, keep := wanted[key]; !keep {
			if err := s.classTeacherRepo.Delete(ctx, id); err != nil {
				return &EducationError{Op: "SetStaffSchoolClasses", Err: err}
			}
		}
	}

	for key, display := range wanted {
		if _, exists := currentByKey[key]; exists {
			continue
		}
		assignment := &education.ClassTeacher{StaffID: staffID, SchoolClass: display}
		if err := s.classTeacherRepo.Create(ctx, assignment); err != nil {
			return &EducationError{Op: "SetStaffSchoolClasses", Err: err}
		}
	}

	return nil
}

// dedupeSchoolClasses trims the submitted class names and dedupes them by
// their normalized identity, keeping the first display form. An entry that is
// empty after trimming is a caller error, not something to skip silently.
func dedupeSchoolClasses(classes []string) (map[string]string, error) {
	wanted := make(map[string]string, len(classes))
	for _, class := range classes {
		key := schoolclass.Normalize(class)
		if key == "" {
			return nil, ErrEmptySchoolClass
		}
		if _, seen := wanted[key]; !seen {
			wanted[key] = strings.TrimSpace(class)
		}
	}
	return wanted, nil
}
