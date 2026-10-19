package compose

import timetableprojection "github.com/moto-nrw/project-phoenix/modules/timetableprojection/internal/adapters/postgres"

func New() timetableprojection.Reader {
	return timetableprojection.Reader{}
}
