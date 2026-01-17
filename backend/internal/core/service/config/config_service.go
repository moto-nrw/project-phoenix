package config

import (
	"github.com/moto-nrw/project-phoenix/internal/core/domain/base"
	configPort "github.com/moto-nrw/project-phoenix/internal/core/port/config"
	"github.com/uptrace/bun"
)

// service implements the Service interface
// and wires the configuration repositories with transactional support.
type service struct {
	settingRepo configPort.SettingRepository
	db          *bun.DB
	txHandler   *base.TxHandler
}

// NewService creates a new config service
func NewService(settingRepo configPort.SettingRepository, db *bun.DB) Service {
	return &service{
		settingRepo: settingRepo,
		db:          db,
		txHandler:   base.NewTxHandler(db),
	}
}

// WithTx returns a new service that uses the provided transaction
func (s *service) WithTx(tx bun.Tx) any {
	// Get repositories with transaction if they implement the TransactionalRepository interface
	var settingRepo = s.settingRepo

	// Try to cast repository to TransactionalRepository and apply the transaction
	if txRepo, ok := s.settingRepo.(base.TransactionalRepository); ok {
		settingRepo = txRepo.WithTx(tx).(configPort.SettingRepository)
	}

	// Return a new service with the transaction
	return &service{
		settingRepo: settingRepo,
		db:          s.db,
		txHandler:   s.txHandler.WithTx(tx),
	}
}
