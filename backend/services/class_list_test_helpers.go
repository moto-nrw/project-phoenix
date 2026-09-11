package services

import (
	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/services/users"
	"github.com/uptrace/bun"
)

type ClassListTestModule struct{ service users.ClassListEntryService }

func NewClassListTestModule(db *bun.DB) (ClassListTestModule, error) {
	r, err := repositories.NewClassListTestRepositories(db, repositories.NewTestAuditStore(db))
	if err != nil {
		return ClassListTestModule{}, err
	}
	return ClassListTestModule{service: users.NewClassListEntryService(r.Entry, r.Student, r.Audit)}, nil
}

func (m ClassListTestModule) NewClassListEntryRuntime() ClassListEntryRuntime {
	// Reuse the production closure mapping with a single capability. This
	// carrier does not invoke any factory constructor or leave composition.
	return (&Factory{ClassListEntries: m.service}).NewClassListEntryRuntime()
}
