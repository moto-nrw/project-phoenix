package education

import (
	"context"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/internal/schoolclass"
	"github.com/moto-nrw/project-phoenix/models/education"
)

// classTeacherRenames builds the rename map an apply implies for the
// Klassenlehrer assignments: trimmed from-class → to-class display form,
// where an empty target means the class graduates and its assignments are
// dropped.
//
// Keys are EXACT (trimmed) matches, deliberately NOT schoolclass.Normalize:
// the student side promotes on exact school_class equality
// (PromoteStudentsByIDs), so the teacher rows must move under precisely the
// same condition. With normalized keys a teacher assigned "1A" would be
// remapped while the "1A" children stay put, and two mappings normalizing
// identically ("1a"→"2a" and "1A"→"2B") would collapse to one entry.
//
// There is deliberately no reverse variant — the revert replays the recorded
// ledger instead, because a reverse rename cannot distinguish a row the apply
// renamed into a class from a pre-existing assignment of that class it never
// touched.
func classTeacherRenames(mappings []*education.GradeTransitionMapping) map[string]string {
	renames := make(map[string]string, len(mappings))
	for _, mapping := range mappings {
		if mapping == nil {
			continue
		}
		if mapping.IsGraduating() {
			renames[strings.TrimSpace(mapping.FromClass)] = ""
			continue
		}
		renames[strings.TrimSpace(mapping.FromClass)] = *mapping.ToClass
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
	// must not collide with these on re-insert. Affected-row matching is
	// exact (trimmed), mirroring the student promotion; only the collision
	// bookkeeping is normalized, because that is what the unique index sees.
	kept := make(map[int64]map[string]bool, len(assignments))
	var affected []*education.ClassTeacher
	for _, assignment := range assignments {
		if _, hit := renames[strings.TrimSpace(assignment.SchoolClass)]; hit {
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
		target := renames[strings.TrimSpace(assignment.SchoolClass)]
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
// reverse rename could not guarantee.
//
// Assignments the admin changed between apply and revert win. Concretely:
//   - a created row the admin already deleted is not deleted again, AND its
//     paired removed entry is not restored — the admin deliberately took the
//     class away from this teacher, and restoring the pre-apply name would
//     silently re-grant student-data scope (a removed entry whose rename
//     target has no created ledger entry at all — merge dedupe, graduation —
//     has no such pair and restores as before);
//   - in a rename chain ("1a"→"2a" while "2a"→"3a") the middle class is
//     dual-role: a created target (paired with the removed "1a") AND a
//     restore source (its own removed entry). An admin removal of that name
//     must suppress BOTH roles, so a removed entry whose OWN class is a
//     created target the admin deleted is skipped too — otherwise the revert
//     re-creates the very name the admin just removed, and any student still
//     carrying that name (enrolled after the cohort lock, so never renamed by
//     the replay) would land back in the teacher's scope. The teacher may
//     lose sight of a cohort the admin left alone under its renamed name;
//     that errs on the side of less scope, and the admin can re-assign;
//   - a restore colliding with a current assignment is skipped;
//   - a restore for a staff member offboarded since the apply is skipped too
//     (offboarding cleared their assignments; the soft-deleted staff row
//     would otherwise satisfy the FK and resurrect scope for a person the
//     normal write path 404s on).
func (s *GradeTransitionService) revertClassTeacherAssignments(
	ctx context.Context,
	transitionID int64,
	mappings []*education.GradeTransitionMapping,
) error {
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

	renames := classTeacherRenames(mappings)

	liveStaff, err := s.liveLedgerStaff(ctx, ledger)
	if err != nil {
		return err
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

	// ledgerCreated: (staff, class) pairs the apply inserted at all.
	// deletedCreated: the subset this revert actually found and deleted —
	// a pair in the first set but not the second was removed by the admin
	// after the apply, and its paired restore must not happen.
	ledgerCreated := make(map[staffClass]bool)
	deletedCreated := make(map[staffClass]bool)
	for _, entry := range ledger {
		if entry.Action != education.ClassTeacherActionCreated {
			continue
		}
		key := staffClass{entry.StaffID, schoolclass.Normalize(entry.SchoolClass)}
		ledgerCreated[key] = true
		if row, ok := current[key]; ok {
			if err := s.classTeacherRepo.Delete(ctx, row.ID); err != nil {
				return fmt.Errorf("failed to delete class teacher assignment: %w", err)
			}
			delete(current, key)
			deletedCreated[key] = true
		}
	}

	for _, entry := range ledger {
		if entry.Action != education.ClassTeacherActionRemoved {
			continue
		}
		if !liveStaff[entry.StaffID] {
			continue // offboarded since the apply — do not resurrect scope
		}
		key := staffClass{entry.StaffID, schoolclass.Normalize(entry.SchoolClass)}
		if ledgerCreated[key] && !deletedCreated[key] {
			// Dual-role name in a rename chain (or a graduation whose class
			// was simultaneously a rename target): the class this entry would
			// restore is itself a row the apply created and the admin has
			// deleted since. Re-creating that name would re-grant scope the
			// admin explicitly removed.
			continue
		}
		if target := renames[strings.TrimSpace(entry.SchoolClass)]; target != "" {
			pair := staffClass{entry.StaffID, schoolclass.Normalize(target)}
			if ledgerCreated[pair] && !deletedCreated[pair] {
				continue // the admin removed the renamed row after the apply
			}
		}
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

// liveLedgerStaff resolves which staff members named in the ledger still
// exist as live (non-soft-deleted) rows. FindByIDs filters soft deletes, so
// an offboarded staff member simply drops out of the map.
func (s *GradeTransitionService) liveLedgerStaff(ctx context.Context, ledger []*education.GradeTransitionClassTeacher) (map[int64]bool, error) {
	staffIDs := make([]int64, 0, len(ledger))
	seen := make(map[int64]bool, len(ledger))
	for _, entry := range ledger {
		if entry.Action != education.ClassTeacherActionRemoved || seen[entry.StaffID] {
			continue
		}
		seen[entry.StaffID] = true
		staffIDs = append(staffIDs, entry.StaffID)
	}
	if len(staffIDs) == 0 {
		return map[int64]bool{}, nil
	}

	staffRows, err := s.staffRepo.FindByIDs(ctx, staffIDs)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve ledger staff: %w", err)
	}
	live := make(map[int64]bool, len(staffRows))
	for id := range staffRows {
		live[id] = true
	}
	return live, nil
}
