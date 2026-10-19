package projection

import (
	"github.com/moto-nrw/project-phoenix/modules/staffannouncementaudience/contract"
	postgres "github.com/moto-nrw/project-phoenix/modules/staffannouncementaudience/internal/adapters/postgres"
)

func New(database postgres.Database) *postgres.Projection { return postgres.New(database) }

func Map[A, V, S any](source *postgres.Projection, rows contract.Rows[A, V, S]) postgres.Mapped[A, V, S] {
	return postgres.Map(source, rows)
}
