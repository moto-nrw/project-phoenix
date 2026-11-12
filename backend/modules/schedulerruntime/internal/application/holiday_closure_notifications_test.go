package application

import (
	"bytes"
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-ogs/project-phoenix/modules/schedulerruntime/ports"
)

func TestHolidayClosureWatchdogRunsWhileProcessingIsStuck(t *testing.T) {
	t.Parallel()
	var logs bytes.Buffer
	started, release, finished := make(chan struct{}), make(chan struct{}), make(chan struct{})
	scheduler := newScheduler(ports.WorkerDependencies{
		Logger: slog.New(slog.NewJSONHandler(&logs, nil)),
		HolidayClosureNotifications: ports.HolidayClosureNotifications{
			ProcessDue: func(context.Context) error { close(started); <-release; return nil },
			Overdue: func(_ context.Context, threshold time.Duration) (int, error) {
				assert.Equal(t, 15*time.Minute, threshold)
				return 2, nil
			},
		},
	})
	task := &ScheduledTask{Name: "holiday-closure-notifications"}
	go func() {
		defer close(finished)
		scheduler.checkAndRunHolidayClosureNotifications(context.Background(), task)
	}()
	t.Cleanup(func() { close(release); <-finished })
	<-started
	scheduler.checkHolidayClosureWatchdog(context.Background(), &ScheduledTask{Name: "holiday-closure-watchdog"})
	require.Contains(t, logs.String(), `"msg":"holiday closure watchdog checked","overdue":2`)
	require.Contains(t, logs.String(), `"msg":"holiday closure commands overdue","overdue":2`)
}
