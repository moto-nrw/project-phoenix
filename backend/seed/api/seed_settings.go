package api

import (
	"errors"
	"fmt"
)

func withTemporarySeedSetting(rt *Runtime, auth AuthRef, key string, temporary, restored any, action func() error) (err error) {
	path := "/api/settings/values/" + key
	if _, err := rt.Client.PutWithAuth(auth, path, map[string]any{"value": temporary}); err != nil {
		return fmt.Errorf("set temporary %s: %w", key, err)
	}
	defer func() {
		if _, restoreErr := rt.Client.PutWithAuth(auth, path, map[string]any{"value": restored}); restoreErr != nil {
			err = errors.Join(err, fmt.Errorf("restore %s: %w", key, restoreErr))
		}
	}()
	return action()
}
