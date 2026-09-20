package compose

import (
	"errors"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
)

type ClassArrivalExceptionStore = ports.ClassArrivalExceptionStore
type ClassArrivalStudents = ports.ClassArrivalStudents

func NewClassArrivalExceptions(store ClassArrivalExceptionStore, students ClassArrivalStudents) (careplan.ClassArrivalExceptions, error) {
	if store != nil && students == nil {
		return nil, errors.New("class arrival exceptions: student directory is required")
	}
	return application.NewClassArrivalExceptions(store, students), nil
}
