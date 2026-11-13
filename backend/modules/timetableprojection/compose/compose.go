package compose

import timetableprojection "github.com/moto-ogs/project-phoenix/modules/timetableprojection/internal/adapters/postgres"

func New() timetableprojection.Reader {
	return timetableprojection.Reader{}
}
