package compose

import presenceprojection "github.com/moto-nrw/project-phoenix/modules/presenceprojection/internal/adapters/postgres"

func New() presenceprojection.Reader {
	return presenceprojection.Reader{}
}
