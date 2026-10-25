package config

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	config "github.com/moto-nrw/project-phoenix/modules/settings/records"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type traegerRuntimeFake struct{ err error }

func (r traegerRuntimeFake) WithinTraeger(ctx context.Context, _ int64, fn func(context.Context) error) error {
	if err := fn(ctx); err != nil {
		return err
	}
	return r.err
}

type traegerValuesFake struct {
	config.TraegerSettingRepository
	recordErr error
}

func (traegerValuesFake) LockByOrganizationAndKey(context.Context, int64, string) (*config.TraegerSettingValue, error) {
	return nil, nil
}
func (traegerValuesFake) Upsert(context.Context, *config.TraegerSettingValue) error { return nil }
func (traegerValuesFake) Delete(context.Context, int64, string) error               { return nil }
func (f traegerValuesFake) RecordChange(context.Context, *config.TraegerSettingAuditEntry, time.Time) error {
	return f.recordErr
}

// TestTraegerChangeNotifiesEverySchoolAfterTheCommitOnly: the schools of the
// Träger hear tenant_settings_changed once the change committed (ADR 0086
// section 8), never for a change that failed, its change-log entry included.
func TestTraegerChangeNotifiesEverySchoolAfterTheCommitOnly(t *testing.T) {
	t.Parallel()
	for name, test := range map[string]struct {
		runtimeErr, recordErr error
		notified              []int64
	}{
		"committed":      {notified: []int64{11, 12}},
		"change log":     {recordErr: errors.New("audit insert failed")},
		"commit refused": {runtimeErr: errors.New("commit failed")},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var notified []int64
			registry := config.NewRegistry()
			registry.Register(config.Definition{Key: "beide.key", Type: config.FieldBoolean, Default: true, Tab: "operations", Category: "test", Levels: config.LevelBoth})
			service, err := NewTraegerSettings(TraegerSettingsDependencies{
				AuditRetentionYears: 3, Values: traegerValuesFake{recordErr: test.recordErr}, Runtime: traegerRuntimeFake{err: test.runtimeErr},
				Schools: func(context.Context, int64) ([]int64, error) { return []int64{11, 12}, nil },
				Notify: func(_ context.Context, school int64, key string) {
					assert.Equal(t, "beide.key", key)
					notified = append(notified, school)
				},
				Rights: map[config.TraegerRight]string{config.TraegerRightSettings: "settings", config.TraegerRightStaff: "staff", config.TraegerRightBilling: "billing"},
				Logger: slog.Default(), Now: time.Now, Registry: registry,
			})
			require.NoError(t, err)
			err = service.SetValue(context.Background(), 5, 6, []string{"settings"}, "beide.key", false)
			if test.notified == nil {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			assert.Equal(t, test.notified, notified)
		})
	}
}
