package common

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	configModel "github.com/moto-nrw/project-phoenix/models/config"
	configService "github.com/moto-nrw/project-phoenix/services/config"
	"github.com/moto-nrw/project-phoenix/services/config/configtest"
	"github.com/stretchr/testify/assert"
)

func TestRequireWebAttendanceEnabled(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		settings   configService.SettingsService
		wantStatus int
		wantBody   string
		wantNext   bool
	}{
		{name: "missing service fails closed", wantStatus: http.StatusServiceUnavailable},
		{
			name: "resolution failure is not treated as disabled",
			settings: &configtest.Mock{ResolveBoolFn: func(context.Context, string) (bool, error) {
				return false, errors.New("repository unavailable")
			}},
			wantStatus: http.StatusServiceUnavailable,
		},
		{
			name: "disabled returns stable code",
			settings: &configtest.Mock{ResolveBoolFn: func(_ context.Context, key string) (bool, error) {
				assert.Equal(t, configModel.KeyAttendanceWebEnabled, key)
				return false, nil
			}},
			wantStatus: http.StatusForbidden,
			wantBody:   ErrCodeAttendanceWebDisabled,
		},
		{
			name: "enabled reaches handler",
			settings: &configtest.Mock{ResolveBoolFn: func(context.Context, string) (bool, error) {
				return true, nil
			}},
			wantStatus: http.StatusNoContent,
			wantNext:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			called := false
			next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				called = true
				w.WriteHeader(http.StatusNoContent)
			})
			handler := RequireWebAttendanceEnabled(tt.settings)(next)
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/attendance", nil))

			assert.Equal(t, tt.wantStatus, w.Code)
			assert.Equal(t, tt.wantNext, called)
			if tt.wantBody != "" {
				assert.True(t, strings.Contains(w.Body.String(), tt.wantBody), w.Body.String())
			}
		})
	}
}
