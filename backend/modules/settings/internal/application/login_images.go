package config

import (
	"context"
	"encoding/json"
	"fmt"

	config "github.com/moto-nrw/project-phoenix/modules/settings/records"
)

// GetLoginImageURL returns the loginImageUrl from the school's settings JSONB.
func (s *settingsService) GetLoginImageURL(ctx context.Context, tenantID int64) (string, error) {
	if tenantID <= 0 {
		return "", nil
	}

	if s.schoolStore == nil {
		return "", config.ErrRuntimeUnavailable
	}
	raw, err := s.schoolStore.FindSettings(ctx, tenantID)
	if err != nil {
		return "", fmt.Errorf("find school: %w", err)
	}

	settings, err := unmarshalSchoolSettings(raw)
	if err != nil {
		return "", err
	}

	url, _ := settings[loginImageKey].(string)
	return url, nil
}

// SetLoginImageURL sets the loginImageUrl in the school's settings JSONB.
func (s *settingsService) SetLoginImageURL(ctx context.Context, tenantID int64, imageURL string) (oldURL string, err error) {
	return s.updateSchoolSetting(ctx, tenantID, &imageURL)
}

// ClearLoginImageURL removes the loginImageUrl from the school's settings JSONB.
func (s *settingsService) ClearLoginImageURL(ctx context.Context, tenantID int64) (oldURL string, err error) {
	return s.updateSchoolSetting(ctx, tenantID, nil)
}

// updateSchoolSetting updates the loginImageUrl in the school's settings JSONB.
// Uses WithAdminTx because platform.schools requires the phoenix_admin role.
// Pass nil imageURL to remove the key.
func (s *settingsService) updateSchoolSetting(ctx context.Context, tenantID int64, imageURL *string) (oldURL string, err error) {
	if s.runtime == nil || s.schoolStore == nil {
		return "", config.ErrRuntimeUnavailable
	}
	err = s.runtime.WithinAdmin(ctx, func(adminCtx context.Context) error {
		return s.schoolStore.UpdateSettings(adminCtx, tenantID, func(raw string) (string, error) {
			settings, unmarshalErr := unmarshalSchoolSettings(raw)
			if unmarshalErr != nil {
				return "", unmarshalErr
			}

			oldURL, _ = settings[loginImageKey].(string)

			if imageURL != nil {
				settings[loginImageKey] = *imageURL
			} else {
				delete(settings, loginImageKey)
			}

			settingsJSON, marshalErr := json.Marshal(settings)
			if marshalErr != nil {
				return "", fmt.Errorf("marshal settings: %w", marshalErr)
			}
			return string(settingsJSON), nil
		})
	})
	return oldURL, err
}

// unmarshalSchoolSettings parses the school's settings JSONB string into a map.
// Empty or missing settings return an empty map. Corrupt JSON returns an error
// to prevent silent data loss on subsequent writes.
func unmarshalSchoolSettings(raw string) (map[string]interface{}, error) {
	if raw == "" || raw == "{}" || raw == "null" {
		return make(map[string]interface{}), nil
	}
	var settings map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		return nil, fmt.Errorf("corrupt school settings JSON: %w", err)
	}
	// json.Unmarshal into map returns nil for the JSON literal "null".
	// Coerce to an empty map so callers can safely assign keys.
	if settings == nil {
		return make(map[string]interface{}), nil
	}
	return settings, nil
}
