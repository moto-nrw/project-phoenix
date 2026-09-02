package repositories

import (
	"time"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/uptrace/bun"
)

// NewFactoryWithPeopleDirectory builds the repository factory with the
// People Directory already bound, so repository tests read the same
// person-enriched rows the service graph does.
func NewFactoryWithPeopleDirectory(db *bun.DB, clocks ...func() time.Time) (*Factory, error) {
	persons, err := NewPeopleDirectory(db)
	if err != nil {
		return nil, err
	}
	factory := NewFactory(db, clocks...)
	factory.BindPeopleDirectory(persons)
	return factory, nil
}

// NewPeopleDirectory composes the person owner behind the legacy
// composition seam for test graphs and CLI roots.
func NewPeopleDirectory(db *bun.DB) (peopledirectory.Capability, error) {
	return peopleCompose.New(peopleCompose.Dependencies{
		DB:      db,
		Observe: func(peopleCompose.Observation) {},
	})
}
