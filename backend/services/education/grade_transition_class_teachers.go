package education

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/schoolclass"
	"github.com/moto-nrw/project-phoenix/models/education"
)

// classTeacherRenames builds the rename map an apply implies for the
// Klassenlehrer assignments: normalized from-class → to-class display form,
// where an empty target means the class graduates and its assignments are
// dropped. There is deliberately no reverse variant — the revert replays the
// recorded ledger instead, because a reverse rename cannot distinguish a row
// the apply renamed into a class from a pre-existing assignment of that class
// it never touched.
func classTeacherRenames(mappings []*education.GradeTransitionMapping) map[string]string {
	renames := make(map[string]string, len(mappings))
	for _, mapping := range mappings {
		if mapping == nil {
			continue
		}
		if mapping.IsGraduating() {
			renames[schoolclass.Normalize(mapping.FromClass)] = ""
			continue
		}
		renames[schoolclass.Normalize(mapping.FromClass)] = *mapping.ToClass
	}
	return renames
}

// remapClassTeacherAssignments follows the transition's class renames for
// education.class_teachers (#1772): a promoted class carries its teachers
// along, a graduating class loses them. Without this, an assignment string
// like "1a" would survive the school-year rollover and silently point at the
// NEXT incoming 1a cohort while the teacher's actual class ("2a") becomes
// invisible to them.
//
// Affected rows are deleted and re-inserted instead of updated in place:
// simultaneous renames ("1a"→"2a" while "2a"→"3a") would collide with the
// unique (tenant, staff, normalized class) index mid-sequence, and merges
// (two classes mapped onto one) need per-staff dedupe anyway. Losing row
// identity is fine — it IS a new school year.
//
// Every delete and insert is recorded in the transition's class-teacher
// ledger so the revert can replay exactly what happened, mirroring the
// student-side history. Runs inside the apply transaction; a nil repo (pure
// unit tests) disables the step.
func (s *GradeTransitionService) remapClassTeacherAssignments(
	ctx context.Context,
	transitionID int64,
	mappings []*education.GradeTransitionMapping,
) error {
	if s.classTeacherRepo == nil {
		return nil
	}
	renames := classTeacherRenames(mappings)
	if len(renames) == 0 {
		return nil
	}

	assignments, err := s.classTeacherRepo.List(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to list class teacher assignments: %w", err)
	}

	// Normalized class names each staff member keeps untouched — renamed rows
	// must not collide with these on re-insert.
	kept := make(map[int64]map[string]bool, len(assignments))
	var affected []*education.ClassTeacher
	for _, assignment := range assignments {
		if _, hit := renames[schoolclass.Normalize(assignment.SchoolClass)]; hit {
			affected = append(affected, assignment)
			continue
		}
		if kept[assignment.StaffID] == nil {
			kept[assignment.StaffID] = make(map[string]bool)
		}
		kept[assignment.StaffID][schoolclass.Normalize(assignment.SchoolClass)] = true
	}
	if len(affected) == 0 {
		return nil
	}

	var ledger []*education.GradeTransitionClassTeacher

	for _, assignment := range affected {
		if err := s.classTeacherRepo.Delete(ctx, assignment.ID); err != nil {
			return fmt.Errorf("failed to delete class teacher assignment: %w", err)
		}
		ledger = append(ledger, &education.GradeTransitionClassTeacher{
			TransitionID: transitionID,
			StaffID:      assignment.StaffID,
			SchoolClass:  assignment.SchoolClass,
			Action:       education.ClassTeacherActionRemoved,
		})
	}

	for _, assignment := range affected {
		target := renames[schoolclass.Normalize(assignment.SchoolClass)]
		if target == "" {
			continue // graduated — assignment is dropped
		}
		key := schoolclass.Normalize(target)
		if kept[assignment.StaffID][key] {
			continue // staff already holds the target class
		}
		replacement := &education.ClassTeacher{StaffID: assignment.StaffID, SchoolClass: target}
		if err := s.classTeacherRepo.Create(ctx, replacement); err != nil {
			return fmt.Errorf("failed to create class teacher assignment: %w", err)
		}
		ledger = append(ledger, &education.GradeTransitionClassTeacher{
			TransitionID: transitionID,
			StaffID:      assignment.StaffID,
			SchoolClass:  target,
			Action:       education.ClassTeacherActionCreated,
		})
		if kept[assignment.StaffID] == nil {
			kept[assignment.StaffID] = make(map[string]bool)
		}
		kept[assignment.StaffID][key] = true
	}

	if err := s.transitionRepo.CreateClassTeacherHistoryBatch(ctx, ledger); err != nil {
		return fmt.Errorf("failed to record class teacher ledger: %w", err)
	}
	return nil
}

// revertClassTeacherAssignments replays the transition's class-teacher ledger
// backwards (#1772): rows the apply created are deleted again, rows it
// removed are restored with their original display form. Rows the apply never
// touched — a pre-existing assignment of a rename target, say — have no
// ledger entry and stay untouched, which is exactly what the mapping-derived
// reverse rename could not guarantee. Assignments the admin changed between
// apply and revert win: a created row already deleted is skipped, a restore
// colliding with a current assignment is skipped.
func (s *GradeTransitionService) revertClassTeacherAssignments(ctx context.Context, transitionID int64) error {
	if s.classTeacherRepo == nil {
		return nil
	}

	ledger, err := s.transitionRepo.GetClassTeacherHistory(ctx, transitionID)
	if err != nil {
		return fmt.Errorf("failed to load class teacher ledger: %w", err)
	}
	if len(ledger) == 0 {
		return nil
	}

	assignments, err := s.classTeacherRepo.List(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to list class teacher assignments: %w", err)
	}

	type staffClass struct {
		staffID int64
		class   string
	}
	current := make(map[staffClass]*education.ClassTeacher, len(assignments))
	for _, assignment := range assignments {
		current[staffClass{assignment.StaffID, schoolclass.Normalize(assignment.SchoolClass)}] = assignment
	}

	for _, entry := range ledger {
		if entry.Action != education.ClassTeacherActionCreated {
			continue
		}
		key := staffClass{entry.StaffID, schoolclass.Normalize(entry.SchoolClass)}
		if row, ok := current[key]; ok {
			if err := s.classTeacherRepo.Delete(ctx, row.ID); err != nil {
				return fmt.Errorf("failed to delete class teacher assignment: %w", err)
			}
			delete(current, key)
		}
	}

	for _, entry := range ledger {
		if entry.Action != education.ClassTeacherActionRemoved {
			continue
		}
		key := staffClass{entry.StaffID, schoolclass.Normalize(entry.SchoolClass)}
		if _, taken := current[key]; taken {
			continue
		}
		restored := &education.ClassTeacher{StaffID: entry.StaffID, SchoolClass: entry.SchoolClass}
		if err := s.classTeacherRepo.Create(ctx, restored); err != nil {
			return fmt.Errorf("failed to restore class teacher assignment: %w", err)
		}
		current[key] = restored
	}

	return nil
}
