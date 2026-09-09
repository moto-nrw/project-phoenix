// Package peopletest composes the People Directory owner for behavior tests.
package peopletest

import (
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/enrollment"
	"github.com/uptrace/bun"
)

func NewEnrollment(db *bun.DB) (enrollment.Commands, error) {
	return compose.New(compose.Dependencies{DB: db, Observe: func(compose.Observation) {}})
}
