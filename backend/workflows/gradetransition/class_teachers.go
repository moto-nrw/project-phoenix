package gradetransition

import (
	"context"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/internal/schoolclass"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
)

// classRenames builds the rename map an apply implies: trimmed from-class to
// to-class display form, where an empty target means the class graduates
// and its rows are dropped. Keys are EXACT (trimmed) matches, deliberately
// not normalized: the student side promotes on exact school_class equality,
// so the teacher rows must move under precisely the same condition.
//
// There is deliberately no reverse variant: the revert replays the recorded
// ledger instead, because a reverse rename cannot distinguish a row the
// apply renamed into a class from a pre-existing row of that class.
func classRenames(mappings []schoolstructure.TransitionMapping) map[string]string {
	renames := make(map[string]string, len(mappings))
	for _, mapping := range mappings {
		if mapping.IsGraduating() {
			renames[strings.TrimSpace(mapping.FromClass)] = ""
			continue
		}
		renames[strings.TrimSpace(mapping.FromClass)] = *mapping.ToClass
	}
	return renames
}

type staffClass struct {
	staffID int64
	class   string
}

// remapClassTeacherAssignments follows the transition's class renames for
// the Klassenlehrer assignments: a promoted class carries its teachers
// along, a graduating class loses them. Left untouched, a "1a" assignment
// would survive the rollover and point at NEXT year's incoming 1a.
//
// Affected rows are deleted and re-inserted instead of updated in place:
// simultaneous renames ("1a" to "2a" while "2a" to "3a") would collide with
// the unique (staff, normalized class) index mid-sequence, and merges need
// per-staff dedupe anyway. Every delete and insert is ledgered so the revert
// replays exactly what happened.
func (w *Workflow) remapClassTeacherAssignments(ctx context.Context, transitionID int64, mappings []schoolstructure.TransitionMapping) error {
	renames := classRenames(mappings)
	if len(renames) == 0 {
		return nil
	}
	assignments, err := w.deps.Membership.ListClassAssignments(ctx, schoolmembership.ClassAssignmentFilter{})
	if err != nil {
		return fmt.Errorf("failed to list class teacher assignments: %w", err)
	}
	// Normalized class names each staff member keeps untouched; renamed rows
	// must not collide with these on re-insert.
	kept := make(map[int64]map[string]bool, len(assignments))
	var affected []schoolmembership.ClassAssignment
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
	var ledger []schoolstructure.TransitionClassTeacherEntry
	for _, assignment := range affected {
		if err := w.deps.Membership.DeleteClassAssignment(ctx, assignment.ID); err != nil {
			return fmt.Errorf("failed to delete class teacher assignment: %w", err)
		}
		ledger = append(ledger, schoolstructure.TransitionClassTeacherEntry{
			TransitionID: transitionID, StaffID: assignment.StaffID, SchoolClass: assignment.SchoolClass,
			Action: schoolstructure.LedgerActionRemoved,
		})
	}
	for _, assignment := range affected {
		target := renames[strings.TrimSpace(assignment.SchoolClass)]
		if target == "" {
			continue // graduated: the assignment is dropped
		}
		key := schoolclass.Normalize(target)
		if kept[assignment.StaffID][key] {
			continue // the staff member already holds the target class
		}
		if _, err := w.deps.Membership.CreateClassAssignment(ctx, schoolmembership.CreateClassAssignment{StaffID: assignment.StaffID, SchoolClass: target}); err != nil {
			return fmt.Errorf("failed to create class teacher assignment: %w", err)
		}
		ledger = append(ledger, schoolstructure.TransitionClassTeacherEntry{
			TransitionID: transitionID, StaffID: assignment.StaffID, SchoolClass: target,
			Action: schoolstructure.LedgerActionCreated,
		})
		if kept[assignment.StaffID] == nil {
			kept[assignment.StaffID] = make(map[string]bool)
		}
		kept[assignment.StaffID][key] = true
	}
	if err := w.deps.Structure.AppendTransitionClassTeacherLedger(ctx, ledger); err != nil {
		return fmt.Errorf("failed to record class teacher ledger: %w", translateStructureError(err))
	}
	return nil
}

// revertClassTeacherAssignments replays the ledger backwards: rows the apply
// created are deleted again, rows it removed are restored with their
// original display form. Assignments the admin changed between apply and
// revert win: a created row the admin already deleted is not deleted again
// AND its paired removed entry is not restored (the admin deliberately took
// the class away; restoring the pre-apply name would silently re-grant
// student-data scope); in a rename chain the middle class is dual-role, so a
// removed entry whose OWN class is a created target the admin deleted is
// skipped too; a restore colliding with a current assignment is skipped; and
// a restore for a staff member offboarded since the apply is skipped.
func (w *Workflow) revertClassTeacherAssignments(ctx context.Context, transitionID int64, mappings []schoolstructure.TransitionMapping) error {
	ledger, err := w.deps.Structure.ListTransitionClassTeacherLedger(ctx, transitionID)
	if err != nil {
		return fmt.Errorf("failed to load class teacher ledger: %w", translateStructureError(err))
	}
	if len(ledger) == 0 {
		return nil
	}
	renames := classRenames(mappings)
	liveStaff, err := w.liveLedgerStaff(ctx, ledger)
	if err != nil {
		return err
	}
	assignments, err := w.deps.Membership.ListClassAssignments(ctx, schoolmembership.ClassAssignmentFilter{})
	if err != nil {
		return fmt.Errorf("failed to list class teacher assignments: %w", err)
	}
	current := make(map[staffClass]schoolmembership.ClassAssignment, len(assignments))
	for _, assignment := range assignments {
		current[staffClass{assignment.StaffID, schoolclass.Normalize(assignment.SchoolClass)}] = assignment
	}
	// ledgerCreated: pairs the apply inserted at all. deletedCreated: the
	// subset this revert actually found and deleted; a pair in the first set
	// but not the second was removed by the admin after the apply.
	ledgerCreated := make(map[staffClass]bool)
	deletedCreated := make(map[staffClass]bool)
	for _, entry := range ledger {
		if entry.Action != schoolstructure.LedgerActionCreated {
			continue
		}
		key := staffClass{entry.StaffID, schoolclass.Normalize(entry.SchoolClass)}
		ledgerCreated[key] = true
		if row, ok := current[key]; ok {
			if err := w.deps.Membership.DeleteClassAssignment(ctx, row.ID); err != nil {
				return fmt.Errorf("failed to delete class teacher assignment: %w", err)
			}
			delete(current, key)
			deletedCreated[key] = true
		}
	}
	for _, entry := range ledger {
		if entry.Action != schoolstructure.LedgerActionRemoved {
			continue
		}
		if !liveStaff[entry.StaffID] {
			continue // offboarded since the apply: do not resurrect scope
		}
		key := staffClass{entry.StaffID, schoolclass.Normalize(entry.SchoolClass)}
		if ledgerCreated[key] && !deletedCreated[key] {
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
		restored, err := w.deps.Membership.CreateClassAssignment(ctx, schoolmembership.CreateClassAssignment{StaffID: entry.StaffID, SchoolClass: entry.SchoolClass})
		if err != nil {
			return fmt.Errorf("failed to restore class teacher assignment: %w", err)
		}
		current[key] = restored
	}
	return nil
}

// liveLedgerStaff resolves which staff members named in the ledger still
// exist as live rows; an offboarded staff member drops out of the map.
func (w *Workflow) liveLedgerStaff(ctx context.Context, ledger []schoolstructure.TransitionClassTeacherEntry) (map[int64]bool, error) {
	staffIDs := make([]int64, 0, len(ledger))
	seen := make(map[int64]bool, len(ledger))
	for _, entry := range ledger {
		if entry.Action != schoolstructure.LedgerActionRemoved || seen[entry.StaffID] {
			continue
		}
		seen[entry.StaffID] = true
		staffIDs = append(staffIDs, entry.StaffID)
	}
	live := make(map[int64]bool, len(staffIDs))
	if len(staffIDs) == 0 {
		return live, nil
	}
	rows, err := w.deps.Membership.ListStaff(ctx, schoolmembership.StaffFilter{IDs: staffIDs})
	if err != nil {
		return nil, fmt.Errorf("failed to resolve ledger staff: %w", err)
	}
	for _, row := range rows {
		if !row.IsDeleted() {
			live[row.ID] = true
		}
	}
	return live, nil
}
