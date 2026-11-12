package compose

import presenceprojection "github.com/moto-ogs/project-phoenix/modules/presenceprojection/internal/adapters/postgres"

func New() presenceprojection.Reader {
	return presenceprojection.Reader{}
}
