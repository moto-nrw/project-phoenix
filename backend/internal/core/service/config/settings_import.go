package config

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/internal/core/domain/config"
	"github.com/uptrace/bun"
)

// ImportSettings imports multiple settings in a batch operation
func (s *service) ImportSettings(ctx context.Context, settings []*config.Setting) ([]error, error) {
	if len(settings) == 0 {
		return nil, nil
	}

	var errors []error

	err := s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		txService := s.WithTx(tx).(Service)

		for _, setting := range settings {
			if err := processSettingImport(ctx, txService, setting); err != nil {
				errors = append(errors, &ConfigError{Op: "ImportSettings", Err: err})
			}
		}

		return checkImportErrors(errors)
	})

	return handleImportResult(errors, err)
}

// processSettingImport processes a single setting import (update or create)
func processSettingImport(ctx context.Context, txService Service, setting *config.Setting) error {
	existingSetting, err := txService.GetSettingByKeyAndCategory(ctx, setting.Key, setting.Category)
	if settingExists(err, existingSetting) {
		return updateExistingSetting(ctx, txService, existingSetting, setting)
	}
	return txService.CreateSetting(ctx, setting)
}

// settingExists checks if a setting was found successfully
func settingExists(err error, setting *config.Setting) bool {
	return err == nil && setting != nil && setting.ID > 0
}

// updateExistingSetting updates an existing setting with new values
func updateExistingSetting(ctx context.Context, txService Service, existing, new *config.Setting) error {
	existing.Value = new.Value
	existing.Description = new.Description
	existing.RequiresRestart = new.RequiresRestart
	existing.RequiresDBReset = new.RequiresDBReset
	return txService.UpdateSetting(ctx, existing)
}

// checkImportErrors checks if any errors occurred and returns appropriate error
func checkImportErrors(errors []error) error {
	if len(errors) > 0 {
		return fmt.Errorf("import failed with %d errors", len(errors))
	}
	return nil
}

// handleImportResult handles the final result of import operation
func handleImportResult(errors []error, txErr error) ([]error, error) {
	if txErr != nil {
		errors = append(errors, &ConfigError{Op: "ImportSettings", Err: txErr})
	}

	if len(errors) > 0 {
		return errors, &BatchOperationError{Errors: errors}
	}

	return nil, nil
}
