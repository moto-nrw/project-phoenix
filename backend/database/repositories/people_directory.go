package repositories

import (
	"context"
	"fmt"

	auditRepositories "github.com/moto-nrw/project-phoenix/database/repositories/audit"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	careplanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/uptrace/bun"
)

// NewPeopleDirectory composes the person and student owner behind the
// legacy composition seam for test graphs and CLI roots. The observed
// instance of the serve root replaces it through BindPeopleDirectory.
func NewPeopleDirectory(db *bun.DB) (peopledirectory.Capability, error) {
	capability, _, err := NewPeopleDirectoryWithPhotos(db)
	return capability, err
}

// NewPeopleDirectoryWithPhotos additionally hands back the slot the photo
// lifecycle resolves its runtime from, so a caller that has the feature gate,
// the caller access and the file cleanup fills it after the directory exists.
func NewPeopleDirectoryWithPhotos(db *bun.DB) (peopledirectory.Capability, *peopleCompose.StudentPhotoRuntime, error) {
	photoRuntime := new(peopleCompose.StudentPhotoRuntime)
	// Care Plan's companion slice is constructible from the database alone, so
	// the directory's write path gets it here rather than waiting for the full
	// Care Plan — which needs this directory and therefore cannot be built yet.
	companions, err := careplanCompose.NewCompanionRecords(db, func(careplanCompose.Observation) {})
	if err != nil {
		return nil, nil, err
	}
	capability, err := peopleCompose.New(peopleCompose.Dependencies{
		DB:                    db,
		Observe:               func(peopleCompose.Observation) {},
		StudentFieldAudit:     NewStudentFieldAuditLog(db),
		StudentConsentHistory: NewStudentConsentHistory(db),
		StudentPhotoRuntime:   func() peopleCompose.StudentPhotoRuntime { return *photoRuntime },
		StudentCompanions:     NewStudentCompanionSeam(companions),
	})
	if err != nil {
		return nil, nil, err
	}
	return capability, photoRuntime, nil
}

// MustNewPeopleDirectory is NewPeopleDirectory for composition seams that
// cannot return an error; composing only fails on missing dependencies, which
// is a programming error at wiring time.
func MustNewPeopleDirectory(db *bun.DB) peopledirectory.Capability {
	capability, err := NewPeopleDirectory(db)
	if err != nil {
		panic(fmt.Sprintf("repository factory: compose people directory: %v", err))
	}
	return capability
}

// NewStudentFieldAuditLog binds the Audit Platform trail behind the People
// Directory change-history seam. Audit Platform owns
// audit.student_field_edits; this is the composition seam that hands the
// directory an append and a read of it.
func NewStudentFieldAuditLog(db *bun.DB) peopleCompose.StudentFieldAuditLog {
	return studentFieldAuditLog{repo: auditRepositories.NewStudentFieldEditRepository(auditRootRuntime(db))}
}

type studentFieldAuditLog struct {
	repo auditModels.StudentFieldEditRepository
}

func (l studentFieldAuditLog) Append(ctx context.Context, edits []peopledirectory.StudentFieldEdit) error {
	rows := make([]*auditModels.StudentFieldEdit, 0, len(edits))
	for _, edit := range edits {
		rows = append(rows, &auditModels.StudentFieldEdit{
			StudentID: edit.StudentID, EditedBy: edit.EditedBy, EditedByName: edit.EditedByName,
			FieldName: edit.FieldName, OldValue: edit.OldValue, NewValue: edit.NewValue,
		})
	}
	return l.repo.CreateBatch(ctx, rows)
}

func (l studentFieldAuditLog) ListByStudent(ctx context.Context, studentID int64) ([]peopledirectory.StudentFieldEdit, error) {
	rows, err := l.repo.GetByStudentID(ctx, studentID)
	if err != nil {
		return nil, err
	}
	result := make([]peopledirectory.StudentFieldEdit, 0, len(rows))
	for _, row := range rows {
		if row == nil {
			continue
		}
		result = append(result, peopledirectory.StudentFieldEdit{
			ID: row.ID, StudentID: row.StudentID, EditedBy: row.EditedBy, EditedByName: row.EditedByName,
			FieldName: row.FieldName, OldValue: row.OldValue, NewValue: row.NewValue, CreatedAt: row.CreatedAt,
		})
	}
	return result, nil
}

// bindDefaultPeopleDirectory gives every student port an unobserved People
// Directory as soon as the factory exists (#2662): a legacy repository never
// answers a student read without the owner, and graphs that stop at
// NewFactory (CLI roots, repository tests) keep working. BindPeopleDirectory
// rebinds the ports with the observed capability and adds the person
// projections.
func (f *Factory) bindDefaultPeopleDirectory(db *bun.DB) {
	students := MustNewPeopleDirectory(db)
	f.students = students
	f.bindStudentDirectories(students, students)
	f.bindGuardianDirectories(students)
}
