package education

import (
	"context"

	"github.com/moto-nrw/project-phoenix/internal/schoolclass"
	"github.com/moto-nrw/project-phoenix/models/education"
)

// classTeacherRenames builds the rename map a transition implies for the
// Klassenlehrer assignments: normalized from-class → to-class display form,
// where an empty target means the class graduates and its assignments are
// dropped. reverse=true builds the revert direction (to → from, promotions
// only — graduated assignments were deleted on apply and have no history to
// restore from; the admin re-creates them if the revert brings the class
// back).
func classTeacherRenames(mappings []*education.GradeTransitionMapping, reverse bool) map[string]string {
	renames := make(map[string]string, len(mappings))
	for _, mapping := range mappings {
		if mapping == nil {
			continue
		}
		if mapping.IsGraduating() {
			if !reverse {
				renames[schoolclass.Normalize(mapping.FromClass)] = ""
			}
			continue
		}
		if reverse {
			key := schoolclass.Normalize(*mapping.ToClass)
			if _, taken := renames[key]; !taken {
				renames[key] = mapping.FromClass
			}
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
// identity is fine — it IS a new school year. Runs inside the apply/revert
// transaction; a nil repo (pure unit tests) disables the step.
func (s *GradeTransitionService) remapClassTeacherAssignments(ctx context.Context, renames map[string]string) error {
	if s.classTeacherRepo == nil || len(renames) == 0 {
		return nil
	}

	assignments, err := s.classTeacherRepo.List(ctx, nil)
	if err != nil {
		return err
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

	for _, assignment := range affected {
		if err := s.classTeacherRepo.Delete(ctx, assignment.ID); err != nil {
			return err
		}
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
			return err
		}
		if kept[assignment.StaffID] == nil {
			kept[assignment.StaffID] = make(map[string]bool)
		}
		kept[assignment.StaffID][key] = true
	}

	return nil
}
