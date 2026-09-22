package presence_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/services"
	"github.com/stretchr/testify/assert"
)

type sessionWindowSettings struct {
	has    bool
	hasErr error
	intVal int
	intErr error
}

func (s *sessionWindowSettings) HasTenantOverride(context.Context, string) (bool, error) {
	return s.has, s.hasErr
}
func (s *sessionWindowSettings) ResolveString(context.Context, string) (string, error) {
	return "", nil
}
func (s *sessionWindowSettings) ResolveInt(context.Context, string) (int, error) {
	return s.intVal, s.intErr
}

func TestSessionDeviceDirectoryOnlineWindow(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		// The directory takes the device-fleet module's own settings port;
		// spelled inline so this test names no package it may not import.
		settings interface {
			HasTenantOverride(context.Context, string) (bool, error)
			ResolveInt(context.Context, string) (int, error)
		}
		want time.Duration
	}{
		{name: "without settings"},
		{name: "override check failure", settings: &sessionWindowSettings{hasErr: errors.New("settings db down")}},
		{name: "without override", settings: &sessionWindowSettings{intVal: 2}},
		{name: "invalid override", settings: &sessionWindowSettings{has: true}},
		{name: "resolution failure", settings: &sessionWindowSettings{has: true, intErr: errors.New("missing")}},
		{name: "valid override", settings: &sessionWindowSettings{has: true, intVal: 2}, want: 2 * time.Minute},
	} {
		t.Run(tc.name, func(t *testing.T) {
			directory := services.NewSessionDeviceDirectory(nil, tc.settings, nil)
			assert.Equal(t, tc.want, directory.OnlineWindow(context.Background()))
		})
	}
}
