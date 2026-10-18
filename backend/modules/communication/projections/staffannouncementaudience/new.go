package projection

import (
	postgres "github.com/moto-nrw/project-phoenix/modules/communication/internal/adapters/audience"
	"github.com/moto-nrw/project-phoenix/modules/staffannouncementaudience/contract"
)

func New(database postgres.Database) *postgres.Projection { return postgres.New(database) }

func Map[A, V, S any](source *postgres.Projection, rows contract.Rows[A, V, S]) postgres.Mapped[A, V, S] {
	return postgres.Map(source, rows)
}
