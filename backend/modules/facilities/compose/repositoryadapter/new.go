package repositoryadapter

import (
	"github.com/moto-nrw/project-phoenix/modules/facilities"
	implementation "github.com/moto-nrw/project-phoenix/modules/facilities/internal/application/repositoryadapter"
)

func New(rooms facilities.Capability) *implementation.Repository { return implementation.New(rooms) }
func NewRecords(rooms facilities.Query) *implementation.Records {
	return implementation.NewRecords(rooms)
}
