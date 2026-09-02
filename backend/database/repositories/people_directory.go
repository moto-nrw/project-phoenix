package repositories

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/uptrace/bun"
)

// NewPeopleDirectory composes the person and student owner behind the
// legacy composition seam for test graphs and CLI roots. The observed
// instance of the serve root replaces it through BindPeopleDirectory.
func NewPeopleDirectory(db *bun.DB) (peopledirectory.Capability, error) {
	return peopleCompose.New(peopleCompose.Dependencies{
		DB:      db,
		Observe: func(peopleCompose.Observation) {},
	})
}

// bindDefaultPeopleDirectory gives every student port an unobserved People
// Directory as soon as the factory exists (#2662): a legacy repository never
// answers a student read without the owner, and graphs that stop at
// NewFactory (CLI roots, repository tests) keep working. BindPeopleDirectory
// rebinds the ports with the observed capability and adds the person
// projections.
func (f *Factory) bindDefaultPeopleDirectory(db *bun.DB) {
	students, err := NewPeopleDirectory(db)
	if err != nil {
		panic(fmt.Sprintf("repository factory: compose people directory: %v", err))
	}
	f.students = students
	f.bindStudentDirectories(students, students)
}

// countPrivacyConsents serves the student-deletion preview through the
// consent owner (student-presence, #2662). A child carries a handful of
// consent rows at most, so the existing listing is the count.
func (f *Factory) countPrivacyConsents(ctx context.Context, studentID int64) (int, error) {
	consents, err := f.PrivacyConsent.FindByStudentID(ctx, studentID)
	if err != nil {
		return 0, err
	}
	return len(consents), nil
}
