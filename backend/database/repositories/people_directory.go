package repositories

import (
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/uptrace/bun"
)

// peopleRuntime binds the retained People Directory repositories
// (database/repositories/users) to the tenant runtime (#2727).
func peopleRuntime(db *bun.DB) peopleCompose.LegacyRepositoryRuntime {
	return peopleCompose.NewLegacyRepositoryRuntime(db)
}

// peopleDirectoryReads composes the People Directory for the student reads and
// the row lock the adapters still in this package take. The full owner, with
// its write owners and audit seams, is composed by services (#2743); none of
// these reads reaches them.
func peopleDirectoryReads(db *bun.DB) *peopledirectory.Module {
	module, err := peopleCompose.New(peopleCompose.Dependencies{DB: db, Observe: func(peopleCompose.Observation) {}})
	if err != nil {
		panic(fmt.Sprintf("compose people directory reads: %v", err))
	}
	return module
}
