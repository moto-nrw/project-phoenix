package api

import (
	"context"
	"errors"

	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	auditClassList "github.com/moto-nrw/project-phoenix/modules/auditlog/classlist"
	auditModels "github.com/moto-nrw/project-phoenix/modules/auditlog/records"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	schoolMembershipCompose "github.com/moto-nrw/project-phoenix/modules/schoolmembership/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// The audited class-list administration (#2382, #3355) lives with School
// Membership, but two of its collaborators come from other owners: the People
// Directory student lookup behind the duplicate guard and the "Zuordnen"
// hint, and the fail-closed Audit command behind the change trail. The root
// adapts them to the owner's ports and composes the administration over them.

// composeClassListEntries composes the observed School Membership owner with
// its audited class-list administration. The Klassenliste routes mount it.
func composeClassListEntries(db transactionruntime.Database, persons peopledirectory.Capability, audit auditModels.Command) (schoolmembership.Owner, error) {
	if persons == nil || audit == nil {
		return nil, errors.New("class list entry administration: people directory and audit command are required")
	}
	dependencies, err := owners.SchoolMembershipDependencies(db)
	if err != nil {
		return nil, err
	}
	dependencies.ClassListStudents, dependencies.ClassListTrail = classListEntryStudents{persons: persons}, classListEntryAuditTrail{audit: audit}
	return schoolMembershipCompose.New(dependencies)
}

// classListEntryStudents answers "is this name already a child of that class"
// through the People Directory, matching like the legacy student lookup did:
// case-insensitive and trimmed on all three keys, alumni excluded — a
// graduate must not block a new child of the same name. It is the same
// two-step the class-list import already resolves its guard with.
//
// One difference to the legacy SQL, which joined users.persons directly: the
// directory leaves soft-deleted persons out, so a student whose person row is
// deleted no longer blocks an entry. That row is a broken state either way,
// and the owner is the only one allowed to decide which persons exist.
type classListEntryStudents struct {
	persons peopledirectory.NamesakeDirectory
}

func (d classListEntryStudents) ListStudentIDsByNameAndClass(ctx context.Context, firstName, lastName, schoolClass string) ([]int64, error) {
	return peopledirectory.EnrolledStudentIDsByNameAndClass(ctx, d.persons, firstName, lastName, schoolClass)
}

func (d classListEntryStudents) IsEnrolledStudent(ctx context.Context, studentID int64) (bool, error) {
	return peopledirectory.IsEnrolledStudent(ctx, d.persons, studentID)
}

// classListEntryAuditTrail appends the change through the single Audit command, so
// the row joins the producer's transaction and a refused append aborts it.
type classListEntryAuditTrail struct{ audit auditModels.Command }

func (t classListEntryAuditTrail) AppendClassListEntryChange(ctx context.Context, change schoolmembership.ClassListEntryChange) error {
	return t.audit.Append(ctx, &auditClassList.ClassListEntryChange{
		EntryID: change.EntryID, Action: change.Action, OldValue: change.OldValue,
		NewValue: change.NewValue, MatchedStudentID: change.MatchedStudentID, ChangedBy: change.ChangedBy,
	})
}
