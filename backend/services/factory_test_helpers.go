package services

import (
	"log/slog"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/uptrace/bun"
)

// NewFactoryForTests creates the partial graph used by legacy package tests.
// Production composition must provide every migrated module explicitly.
func NewFactoryForTests(repos *repositories.Factory, db *bun.DB, logger *slog.Logger, clocks ...func() time.Time) (*Factory, error) {
	return newFactory(repos, db, logger, nil, nil, clocks...)
}
