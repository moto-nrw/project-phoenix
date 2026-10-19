package compose

import (
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/internal/application"
	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/internal/ports"
	"github.com/uptrace/bun"
)

func retainedNewService(db *bun.DB, observe func(ports.Observation), administration application.Administration) *application.Service {
	store := postgres.New(postgres.NewCompositionDatabase())
	return application.New(store, application.NewCompositionObserver(observe), administration)
}
