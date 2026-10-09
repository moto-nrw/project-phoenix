package groups

import (
	"context"
	"maps"
	"slices"
	"sort"
	"strings"

	"github.com/moto-nrw/project-phoenix/internal/schoolclass"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure/internal/domain"
)

// requireStaff resolves the staff existence check shared by both class
// operations. Only a genuine no-row outcome becomes ErrStaffNotFound (the API
// maps it to 404); any other failure (aborted tenant tx, dropped connection)
// keeps its cause and surfaces as a 500 instead of a lying "nicht gefunden".
func (s *service) requireStaff(ctx context.Context, op string, staffID int64) error {
	exists, err := s.staff.StaffExists(ctx, staffID)
	if err != nil {
		return &EducationError{Op: op, Err: err}
	}
	if !exists {
		return &EducationError{Op: op, Err: ErrStaffNotFound}
	}
	return nil
}

// GetStaffSchoolClasses returns the school classes assigned to a staff
// member, in class order, as the display strings they were entered with.
func (s *service) GetStaffSchoolClasses(ctx context.Context, staffID int64) ([]string, error) {
	if err := s.requireStaff(ctx, "GetStaffSchoolClasses", staffID); err != nil {
		return nil, err
	}

	assignments, err := s.staffClassAssignments(ctx, "GetStaffSchoolClasses", staffID)
	if err != nil {
		return nil, err
	}
	return classesInClassOrder(assignments), nil
}

// staffClassAssignments reads the staff member's class assignments, each
// assignment ID with its class as entered.
func (s *service) staffClassAssignments(ctx context.Context, op string, staffID int64) (map[int64]string, error) {
	assignments, err := s.classTeacherRepo.SchoolClassAssignmentsOfStaff(ctx, staffID)
	if err != nil {
		return nil, &EducationError{Op: op, Err: err}
	}
	return assignments, nil
}

// classesInClassOrder lists the classes in class order: case- and
// space-insensitive, ties in assignment order, as the store lists them.
func classesInClassOrder(assignments map[int64]string) []string {
	ids := slices.Collect(maps.Keys(assignments))
	sort.Slice(ids, func(i, j int) bool {
		a := strings.ToLower(strings.TrimSpace(assignments[ids[i]]))
		b := strings.ToLower(strings.TrimSpace(assignments[ids[j]]))
		if a != b {
			return a < b
		}
		return ids[i] < ids[j]
	})
	classes := make([]string, 0, len(ids))
	for _, id := range ids {
		classes = append(classes, assignments[id])
	}
	return classes
}

// SetStaffSchoolClasses replaces the staff member's class assignments with
// the submitted set. Classes are compared via schoolclass.Normalize, so "1a"
// and " 1A " count as the same assignment; unchanged rows are kept (diff, not
// delete-all) so their IDs and timestamps survive a resubmit of the same set,
// and a case-only edit ("1a" -> "1A") updates the stored display form in
// place. Class names are deliberately NOT validated against the current
// student classes: assignments may be created before students are imported.
//
// changedBy is the authenticated account ID; every actual change lands as one
// audit.staff_master_data_changes row in the same transaction — these
// assignments scope the Lehrkraft student day view, so no rewrite may happen
// without a trace.
func (s *service) SetStaffSchoolClasses(ctx context.Context, staffID int64, classes []string, changedBy int64) error {
	const op = "SetStaffSchoolClasses"

	if err := s.requireStaff(ctx, op, staffID); err != nil {
		return err
	}

	wanted, err := dedupeSchoolClasses(classes)
	if err != nil {
		return &EducationError{Op: op, Err: err}
	}

	current, err := s.staffClassAssignments(ctx, op, staffID)
	if err != nil {
		return err
	}

	// Snapshot the display strings BEFORE the diff loop: the audit must
	// record what stood in the DB when the request arrived — a case-only
	// rename would otherwise compare the new value against itself and skip
	// the trail.
	oldClasses := make([]string, 0, len(current))
	for _, schoolClass := range current {
		oldClasses = append(oldClasses, schoolClass)
	}

	currentKeys, err := s.reconcileSchoolClasses(ctx, staffID, current, wanted)
	if err != nil {
		return &EducationError{Op: op, Err: err}
	}
	if err := s.addSchoolClasses(ctx, staffID, currentKeys, wanted); err != nil {
		return &EducationError{Op: op, Err: err}
	}

	if err := s.auditSchoolClassChange(ctx, staffID, changedBy, oldClasses, wanted); err != nil {
		return &EducationError{Op: op, Err: err}
	}

	return nil
}

// auditSchoolClassChange records one audit row per actual change of the
// assignment set, in the same transaction as the writes. oldClasses is the
// pre-write snapshot of the display strings — never the (by now mutated) rows
// themselves. No-op when nothing changed or no audit trail is wired (tests,
// CLI).
func (s *service) auditSchoolClassChange(
	ctx context.Context,
	staffID, changedBy int64,
	oldClasses []string,
	wanted map[string]string,
) error {
	if s.masterDataAudit == nil {
		return nil
	}

	newClasses := make([]string, 0, len(wanted))
	for _, display := range wanted {
		newClasses = append(newClasses, display)
	}
	sortSchoolClasses(oldClasses)
	sortSchoolClasses(newClasses)

	oldValue := strings.Join(oldClasses, ", ")
	newValue := strings.Join(newClasses, ", ")
	if oldValue == newValue {
		return nil
	}

	return s.masterDataAudit.RecordSchoolClassChange(ctx, domain.SchoolClassChange{
		StaffID:   staffID,
		ChangedBy: changedBy,
		OldValue:  oldValue,
		NewValue:  newValue,
	})
}

// sortSchoolClasses orders class names by their normalized identity so the
// audit old/new values are stable regardless of input order.
func sortSchoolClasses(classes []string) {
	sort.Slice(classes, func(i, j int) bool {
		return schoolclass.Normalize(classes[i]) < schoolclass.Normalize(classes[j])
	})
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

func (s *service) reconcileSchoolClasses(ctx context.Context, staffID int64, current map[int64]string, wanted map[string]string) (map[string]struct{}, error) {
	currentKeys := make(map[string]struct{}, len(current))
	for assignmentID, schoolClass := range current {
		key := schoolclass.Normalize(schoolClass)
		currentKeys[key] = struct{}{}
		display, keep := wanted[key]
		if !keep {
			if err := s.classTeacherRepo.RemoveSchoolClass(ctx, assignmentID); err != nil {
				return nil, err
			}
			continue
		}
		if schoolClass != display {
			if err := s.classTeacherRepo.RenameSchoolClass(ctx, assignmentID, staffID, display); err != nil {
				return nil, err
			}
		}
	}

	return currentKeys, nil
}

func (s *service) addSchoolClasses(ctx context.Context, staffID int64, currentKeys map[string]struct{}, wanted map[string]string) error {
	for key, display := range wanted {
		if _, exists := currentKeys[key]; exists {
			continue
		}
		if err := s.classTeacherRepo.AssignSchoolClass(ctx, staffID, display); err != nil {
			return err
		}
	}

	return nil
}
