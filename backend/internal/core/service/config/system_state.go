package config

import "context"

// RequiresRestart checks if any modified settings require a system restart
func (s *service) RequiresRestart(ctx context.Context) (bool, error) {
	filters := map[string]interface{}{
		"requires_restart": true,
	}

	settings, err := s.settingRepo.List(ctx, filters)
	if err != nil {
		return false, &ConfigError{Op: "RequiresRestart", Err: err}
	}

	return len(settings) > 0, nil
}

// RequiresDatabaseReset checks if any modified settings require a database reset
func (s *service) RequiresDatabaseReset(ctx context.Context) (bool, error) {
	filters := map[string]interface{}{
		"requires_db_reset": true,
	}

	settings, err := s.settingRepo.List(ctx, filters)
	if err != nil {
		return false, &ConfigError{Op: "RequiresDatabaseReset", Err: err}
	}

	return len(settings) > 0, nil
}
