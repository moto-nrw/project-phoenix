package scheduler

import (
	"context"
	"maps"
	"strconv"
	"sync"
	"time"

	"github.com/getsentry/sentry-go"
)

// Job runs report to Sentry by one rule (#3640): a run that fails for good
// after all its retries, or panics, becomes exactly one event tagged with the
// job. Failed attempts before that are breadcrumbs of the run; on their own
// they stay a log line and a Prometheus sample. A run that ends well sends
// nothing, so the breadcrumbs of attempts that later succeeded are dropped
// with the run.
//
// Job events carry no user.id, portal or request_id: no session and no
// request stand behind them. school_id is set only when every standing
// failure of the run belongs to the same school; otherwise it is absent,
// never a placeholder.

const (
	jobAttemptBreadcrumbCategory = "job.attempt"
	jobPanicFlushTimeout         = 2 * time.Second
)

type jobRunReportKey struct{}

// jobRunReport collects which schools the standing failures of one run
// belong to. Tenant batches may record failures from several goroutines.
type jobRunReport struct {
	mu       sync.Mutex
	schools  map[int64]struct{}
	unscoped bool
}

func (report *jobRunReport) note(schoolID int64) {
	report.mu.Lock()
	defer report.mu.Unlock()
	if schoolID <= 0 {
		report.unscoped = true
		return
	}
	report.schools[schoolID] = struct{}{}
}

// school returns the one school all standing failures of the run belong to.
func (report *jobRunReport) school() (int64, bool) {
	report.mu.Lock()
	defer report.mu.Unlock()
	if report.unscoped || len(report.schools) != 1 {
		return 0, false
	}
	for schoolID := range report.schools {
		return schoolID, true
	}
	return 0, false
}

// startJobRunReport gives one run of job a Sentry hub of its own: an empty
// scope tagged with job, so its breadcrumbs never mix with another run's. It
// reuses only the client of the hub on ctx or of the global hub.
func startJobRunReport(ctx context.Context, job string) context.Context {
	parent := sentry.GetHubFromContext(ctx)
	if parent == nil {
		parent = sentry.CurrentHub()
	}
	hub := sentry.NewHub(parent.Client(), sentry.NewScope())
	hub.Scope().SetTag("job", job)
	ctx = sentry.SetHubOnContext(ctx, hub)
	return context.WithValue(ctx, jobRunReportKey{}, &jobRunReport{schools: map[int64]struct{}{}})
}

// recordFailedJobAttempt leaves a breadcrumb for an attempt that a retry may
// still make good. It never creates an event.
func recordFailedJobAttempt(ctx context.Context, message string, err error, data map[string]any) {
	addJobBreadcrumb(ctx, message, err, data)
}

// recordStandingJobFailure leaves a breadcrumb for a failure that stands in
// this run and notes the school it belongs to; schoolID 0 means none. The
// event follows once the run is over, from reportJobRunFailure.
func recordStandingJobFailure(ctx context.Context, schoolID int64, message string, err error, data map[string]any) {
	if schoolID > 0 {
		data = withBreadcrumbValue(data, "school_id", schoolID)
	}
	addJobBreadcrumb(ctx, message, err, data)
	if report, ok := ctx.Value(jobRunReportKey{}).(*jobRunReport); ok {
		report.note(schoolID)
	}
}

// reportJobRunFailure sends the one event of a run that failed for good, with
// the run's failed attempts as breadcrumbs.
func reportJobRunFailure(ctx context.Context, err error) {
	if err == nil {
		return
	}
	hub := jobHub(ctx)
	hub.WithScope(func(scope *sentry.Scope) {
		if report, ok := ctx.Value(jobRunReportKey{}).(*jobRunReport); ok {
			if schoolID, ok := report.school(); ok {
				scope.SetTag("school_id", strconv.FormatInt(schoolID, 10))
			}
		}
		hub.CaptureException(err)
	})
}

// reportJobPanic sends the event of a recovered panic and flushes it, since
// a panic may precede the end of the process.
func reportJobPanic(ctx context.Context, recovered any) {
	hub := jobHub(ctx)
	hub.RecoverWithContext(ctx, recovered)
	hub.Flush(jobPanicFlushTimeout)
}

func jobHub(ctx context.Context) *sentry.Hub {
	if hub := sentry.GetHubFromContext(ctx); hub != nil {
		return hub
	}
	return sentry.CurrentHub()
}

func addJobBreadcrumb(ctx context.Context, message string, err error, data map[string]any) {
	if _, ok := ctx.Value(jobRunReportKey{}).(*jobRunReport); !ok {
		// Only a run's own hub collects attempts. On a shared hub they would
		// pile up across runs and land on unrelated events.
		return
	}
	if err != nil {
		data = withBreadcrumbValue(data, "error", err.Error())
	}
	jobHub(ctx).AddBreadcrumb(&sentry.Breadcrumb{
		Type:      "error",
		Category:  jobAttemptBreadcrumbCategory,
		Message:   message,
		Data:      data,
		Level:     sentry.LevelWarning,
		Timestamp: time.Now(),
	}, nil)
}

func withBreadcrumbValue(data map[string]any, key string, value any) map[string]any {
	copied := make(map[string]any, len(data)+1)
	maps.Copy(copied, data)
	copied[key] = value
	return copied
}
