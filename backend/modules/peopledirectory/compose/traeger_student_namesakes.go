package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/adapters/postgres"
)

// NewStudentNamesakes composes the namesakes of the Träger-Büro's hint on a
// holiday registration (#6683, ADR 0214) on the caller's administrative
// transaction.
func NewStudentNamesakes() peopledirectory.StudentNamesakes {
	return postgres.NewStudentNamesakes(postgres.NewDatabase())
}
