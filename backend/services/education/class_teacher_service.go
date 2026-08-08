package education

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/moto-nrw/project-phoenix/internal/schoolclass"
	"github.com/moto-nrw/project-phoenix/models/education"
)

// requireStaff resolves the staff existence check shared by both class
// operations. Only a genuine no-row outcome becomes ErrStaffNotFound (the API
// maps it to 404); any other failure (aborted tenant tx, dropped connection)
// keeps its cause and surfaces as a 500 instead of a lying "nicht gefunden".
func (s *service) requireStaff(ctx context.Context, op string, staffID int64) error {
	if _, err := s.staffRepo.FindByID(ctx, staffID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &EducationError{Op: op, Err: ErrStaffNotFound}
		}
		return &EducationError{Op: op, Err: err}
	}
	return nil
}

// GetStaffSchoolClasses returns the school classes assigned to a staff
// member, in class order, as the display strings they were entered with.
func (s *service) GetStaffSchoolClasses(ctx context.Context, staffID int64) ([]string, error) {
	if err := s.requireStaff(ctx, "GetStaffSchoolClasses", staffID); err != nil {
		return nil, err
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
// delete-all) so their IDs and timestamps survive a resubmit of the same set,
// and a case-only edit ("1a" -> "1A") updates the stored display form in
// place. Class names are deliberately NOT validated against the current
// student classes: assignments may be created before students are imported.
func (s *service) SetStaffSchoolClasses(ctx context.Context, staffID int64, classes []string) error {
	const op = "SetStaffSchoolClasses"

	if err := s.requireStaff(ctx, op, staffID); err != nil {
		return err
	}

	wanted, err := dedupeSchoolClasses(classes)
	if err != nil {
		return &EducationError{Op: op, Err: err}
	}

	current, err := s.classTeacherRepo.FindByStaff(ctx, staffID)
	if err != nil {
		return &EducationError{Op: op, Err: err}
	}

	currentByKey := make(map[string]*education.ClassTeacher, len(current))
	for _, assignment := range current {
		currentByKey[schoolclass.Normalize(assignment.SchoolClass)] = assignment
	}

	for key, assignment := range currentByKey {
		display, keep := wanted[key]
		if !keep {
			if err := s.classTeacherRepo.Delete(ctx, assignment.ID); err != nil {
				return &EducationError{Op: op, Err: err}
			}
			continue
		}
		if assignment.SchoolClass != display {
			assignment.SchoolClass = display
			if err := s.classTeacherRepo.Update(ctx, assignment); err != nil {
				return &EducationError{Op: op, Err: err}
			}
		}
	}

	for key, display := range wanted {
		if _, exists := currentByKey[key]; exists {
			continue
		}
		assignment := &education.ClassTeacher{StaffID: staffID, SchoolClass: display}
		if err := s.classTeacherRepo.Create(ctx, assignment); err != nil {
			return &EducationError{Op: op, Err: err}
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
