package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/moto-nrw/project-phoenix/analytics"
	"github.com/moto-nrw/project-phoenix/database"
	"github.com/moto-nrw/project-phoenix/observability"
	"github.com/moto-nrw/project-phoenix/services"
	"github.com/moto-nrw/project-phoenix/services/scheduler"
)

const shutdownTimeout = 30 * time.Second

// ServeConfig contains the typed inputs for the production Serve root.
type ServeConfig struct {
	Port       string
	EnableCORS bool
	Logger     *slog.Logger
}

// Runtime owns the assembled HTTP graph and its process-scoped resources.
// A Runtime may be served once.
type Runtime struct {
	server          *http.Server
	api             *API
	scheduler       backgroundScheduler
	capacityLogger  *capacityLogger
	tracker         analytics.Tracker
	logger          *slog.Logger
	resourcesUnsafe bool
}

// backgroundScheduler is the lifecycle contract Runtime needs from its worker
// graph. Serve must wait for Stop before it releases shared resources.
type backgroundScheduler interface {
	Start()
	Stop()
}

// startupListener signals after http.Server has registered the listener and is
// ready to accept connections. Shutdown must not begin before that point.
type startupListener struct {
	net.Listener
	started chan struct{}
	once    sync.Once
}

func (listener *startupListener) Accept() (net.Conn, error) {
	listener.once.Do(func() { close(listener.started) })
	return listener.Listener.Accept()
}

// WithRuntime constructs one production Serve graph, runs fn, and releases all
// process-scoped resources in reverse ownership order.
func WithRuntime(ctx context.Context, config ServeConfig, fn func(*Runtime) error) (resultErr error) {
	if ctx == nil {
		return fmt.Errorf("serve context is required")
	}
	if fn == nil {
		return fmt.Errorf("serve callback is required")
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("serve context ended before build: %w", err)
	}

	runtime, err := newRuntime(config)
	if err != nil {
		return err
	}
	defer func() {
		resultErr = errors.Join(resultErr, runtime.closeResources())
	}()
	return fn(runtime)
}

func newRuntime(config ServeConfig) (*Runtime, error) {
	if config.Logger == nil {
		return nil, fmt.Errorf("serve dependency logger is required")
	}
	if strings.TrimSpace(config.Port) == "" {
		return nil, fmt.Errorf("serve dependency port is required")
	}

	config.Logger.Info("initializing API server")

	api, err := New(config.EnableCORS, config.Logger)
	if err != nil {
		return nil, err
	}

	runtime := &Runtime{
		api:            api,
		scheduler:      newScheduler(api, config.Logger),
		capacityLogger: newRuntimeCapacityLogger(api, config.Logger),
		tracker:        api.Services.Tracker,
		logger:         config.Logger,
	}
	runtime.server = &http.Server{
		Addr:    resolveListenAddr(config.Port),
		Handler: runtime.Handler(),
		// ReadTimeout stays modest to protect against slowloris attacks,
		// but WriteTimeout must be disabled to allow long-lived SSE streams.
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 0,
		IdleTimeout:  0,
	}

	return runtime, nil
}

func newRuntimeCapacityLogger(api *API, logger *slog.Logger) *capacityLogger {
	return newCapacityLogger(func() dbCapacityStats {
		stats := database.SnapshotCapacity(api.db)
		return dbCapacityStats{
			openConnections:   stats.OpenConnections,
			inUse:             stats.InUse,
			idle:              stats.Idle,
			waitCount:         stats.WaitCount,
			waitDuration:      stats.WaitDuration,
			maxIdleClosed:     stats.MaxIdleClosed,
			maxLifetimeClosed: stats.MaxLifetimeClosed,
		}
	}, api.Services.RealtimeHub, api.metrics, logger.With("component", "capacity"))
}

// Handler returns the fully assembled production HTTP graph.
func (runtime *Runtime) Handler() http.Handler {
	return runtime.api
}

// resolveListenAddr turns a configured port into a listen address. A value
// containing ":" (e.g. "localhost:8080" in dev) is used verbatim; a bare port
// is prefixed with ":".
func resolveListenAddr(port string) string {
	if strings.Contains(port, ":") {
		return port
	}
	return ":" + port
}

// newScheduler builds and wires the background scheduler, or returns nil when
// cleanup dependencies are absent (session cleanup is one of its tasks).
func newScheduler(api *API, logger *slog.Logger) *scheduler.Scheduler {
	if api.Services == nil || api.Services.ActiveCleanup == nil || api.Services.Active == nil {
		return nil
	}

	// OperatorAuth implements EmailChangeTokenCleaner (5th arg).
	// OperatorInvitation implements OperatorInvitationCleaner (6th arg).
	// Both are backed by the same concrete struct exposed through the
	// two narrower interfaces defined in services/platform.
	sched := scheduler.NewScheduler(api.Services.Active, api.Services.ActiveCleanup, api.Services.Auth, api.Services.Invitation, api.Services.OperatorAuth, api.Services.OperatorInvitation, logger.With("service", "scheduler"))
	sched.SetDB(api.db)
	sched.SetSchoolRepo(api.repos.School)
	sched.SetTenantRuntime(api.tenantRuntime)
	sched.SetTenantRuntimeObserver(observability.RecordTenantRuntimeEvent)
	sched.SetUnitOfWorkObserver(observability.RecordUnitOfWorkEvent)
	sched.SetWorkerTracer(scheduler.WorkerTracer{
		StartJob: func(ctx context.Context, operation string) (context.Context, error) {
			ctx, _, err := api.tracer.StartJob(ctx, operation)
			return ctx, err
		},
		Logger: api.tracer.Logger,
		Failure: func(ctx context.Context, operation, outcome string, err error) {
			api.tracer.Failure(ctx, "worker", operation, outcome, err)
		},
	})

	configureSchedulerServices(sched, api.Services)
	configureSchedulerRepos(sched, api)
	if api.Staff != nil {
		sched.SetStaffDocumentFileCleaner(api.Staff)
	}
	if api.Students != nil {
		sched.SetStudentDocumentFileCleaner(api.Students)
	}
	if api.FileStore != nil {
		sched.SetFileStoreCleaner(api.FileStore)
	}

	return sched
}

// configureSchedulerServices attaches the optional service-backed scheduler
// tasks; each is skipped when its backing service is nil.
func configureSchedulerServices(sched *scheduler.Scheduler, svc *services.Factory) {
	if svc.Settings != nil {
		sched.SetSettingsService(svc.Settings)
	}
	if svc.WorkSession != nil {
		sched.SetWorkSessionCleaner(svc.WorkSession)
		sched.SetBreakAutoEnder(svc.WorkSession)
		// #1798: auto-checkout at planned shift end (per-tenant opt-in).
		sched.SetAutoCheckouter(svc.WorkSession)
	}
	if svc.Feedback != nil {
		sched.SetFeedbackCleaner(svc.Feedback)
	}
	if svc.UnregisteredTagScans != nil {
		sched.SetUnregisteredTagScanCleaner(svc.UnregisteredTagScans)
	}
	if svc.Materialization != nil {
		sched.SetMaterializer(svc.Materialization)
	}
	if svc.AutoStart != nil {
		sched.SetAutoStartService(svc.AutoStart)
	}
	sched.SetAutoEndService(svc.AutoEnd)
	// WP-B14: timetable GDPR cleanup. Nil service → task does not register.
	if svc.TimetableCleanup != nil {
		sched.SetTimetableCleanup(svc.TimetableCleanup)
	}
	sched.SetCalendarFeedCleanup(svc.CalendarFeedCleanup)
	// Tranche 0b: time-tracking GDPR cleanup. Same nil-safe wiring.
	if svc.TimeTrackingCleanup != nil {
		sched.SetTimeTrackingCleanup(svc.TimeTrackingCleanup)
	}
	// Issue #1455: per-child change-history GDPR cleanup. Same nil-safe wiring.
	if svc.StudentChangeLogCleanup != nil {
		sched.SetStudentChangeLogCleanup(svc.StudentChangeLogCleanup)
	}
	// Issue #2189: PWA standalone-usage GDPR cleanup. Same nil-safe wiring.
	if svc.PWAUsage != nil {
		sched.SetPWAUsageCleanup(svc.PWAUsage)
	}
	// Issue #2598: Team-Chat-Aufbewahrung. Eigener nil-Wachposten wie jede
	// andere Scheduler-Registrierung hier - verschachtelt unter PWAUsage haette
	// sie stillschweigend ausgesetzt, sobald jene Konstruktion bedingt wird.
	if svc.StaffMessaging != nil {
		sched.SetStaffMessageCleanup(svc.StaffMessaging)
	}
	if svc.EnrollmentRejectedCleanup != nil {
		sched.SetEnrollmentRejectedCleanup(svc.EnrollmentRejectedCleanup)
	}
	// Parent-enrollment PR 5: platform email outbox worker.
	if svc.EmailOutboxWorker != nil {
		sched.SetOutboxWorker(svc.EmailOutboxWorker)
	}
	// Issue #1671: per-tenant guardian appointment reminders. The calendar
	// service already satisfies the narrow queuer interface.
	if svc.Calendar != nil {
		sched.SetAppointmentReminderQueuer(svc.Calendar)
	}
	// Phase rollover slice 1: per-tenant deadline resolver tick.
	// The adapter narrows the typed return value behind `any` so
	// the scheduler doesn't import the enrollment package.
	if svc.EnrollmentRollover != nil {
		rolloverSvc := svc.EnrollmentRollover
		sched.SetRolloverDeadlineRunner(scheduler.NewRolloverDeadlineRunner(
			func(ctx context.Context, asOf time.Time) (any, error) {
				return rolloverSvc.RunDeadlineWorker(ctx, asOf)
			},
		))
	}
}

// configureSchedulerRepos attaches the repository-backed scheduler tasks; each
// is skipped when its repositories (or the broadcaster) are absent.
func configureSchedulerRepos(sched *scheduler.Scheduler, api *API) {
	repos := api.repos
	if repos == nil {
		return
	}
	// WP-B9: overdue instance tick. Requires the activity-instance repo,
	// room repo, and broadcaster; partial wiring disables the tick.
	if api.Services.RealtimeHub != nil {
		sched.SetInstanceOverdueDeps(repos.ActivityInstance, repos.Room, api.Services.RealtimeHub)
	}
	// Personal reminder notifications: dispatches one event per person and
	// reminder kind through the channel-agnostic abstraction. Gated per tenant
	// by notifications.dispatch_enabled and the reminders.* settings, and per
	// person by their own consent plus notifications.on_duty_only.
	sched.SetReminderNotificationDeps(scheduler.ReminderNotificationDeps{
		Computer:     api.Services.Reminders,
		Notifier:     api.Services.Notifications,
		Preferences:  api.Services.NotificationPreferences,
		Staff:        repos.Staff,
		Accounts:     repos.Account,
		WorkSessions: repos.WorkSession,
	})
	// Daily session-end bridge: closes schedule-side rows for ended
	// active.groups via repositories (issue #585 layering).
	sched.SetTimetableBridgeRepos(repos.InstanceStudent, repos.ActivityInstance, api.Services.TimetableBridge)
	sched.SetStudentStatusDayRepo(repos.StudentStatusDay)
	sched.SetBookingConsistencyAudit(repos.BookingConsistency)
	// Parent-enrollment PR 2: activate-students tick.
	if repos.Student != nil {
		sched.SetStudentLifecycleRepo(repos.Student)
		if api.Services.StudentAudit != nil {
			sched.SetStudentLifecycleAudit(api.Services.StudentAudit)
		}
		// Effect day of "Betreuung beenden" (#2487): runs inside the same tick,
		// before the status transition it shares its candidate set with.
		if api.Services.CareLifecycle != nil {
			sched.SetCareExitEffector(api.Services.CareLifecycle)
		}
	}
}

// Serve runs the HTTP server and embedded worker until ctx is cancelled.
func (runtime *Runtime) Serve(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("serve context is required")
	}
	listener, err := net.Listen("tcp", runtime.server.Addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", runtime.server.Addr, err)
	}

	capacityCtx, stopCapacityLogger := context.WithCancel(context.Background())
	capacityStopped := runtime.startBackground(capacityCtx)
	defer func() {
		stopCapacityLogger()
		<-capacityStopped
	}()

	serveErr := make(chan error, 1)
	servingListener := &startupListener{Listener: listener, started: make(chan struct{})}
	go func() {
		runtime.logger.Info("server listening", slog.String("addr", runtime.server.Addr))
		serveErr <- runtime.server.Serve(servingListener)
	}()
	select {
	case <-servingListener.started:
	case err := <-serveErr:
		return runtime.handleServeExit(err)
	}

	var runErr error
	select {
	case err := <-serveErr:
		runErr = runtime.handleServeExit(err)
	case <-ctx.Done():
		runErr = runtime.shutdown(ctx.Err())
	}

	if runErr == nil {
		runtime.logger.Info("server gracefully stopped")
	}
	return runErr
}

func (runtime *Runtime) handleServeExit(err error) error {
	shutdownErr := runtime.shutdown(err)
	if errors.Is(err, http.ErrServerClosed) {
		return shutdownErr
	}
	return errors.Join(fmt.Errorf("serve HTTP: %w", err), shutdownErr)
}

func (runtime *Runtime) startBackground(capacityCtx context.Context) <-chan struct{} {
	capacityStopped := make(chan struct{})
	if runtime.capacityLogger != nil {
		runtime.capacityLogger.LogSnapshot()
		go func() {
			runtime.capacityLogger.Start(capacityCtx)
			close(capacityStopped)
		}()
	} else {
		close(capacityStopped)
	}
	if runtime.scheduler != nil {
		runtime.scheduler.Start()
	}
	return capacityStopped
}

func (runtime *Runtime) shutdown(reason error) error {
	return runtime.shutdownWithTimeout(reason, shutdownTimeout)
}

func (runtime *Runtime) shutdownWithTimeout(reason error, timeout time.Duration) error {
	runtime.logger.Info("server shutting down", slog.String("reason", reason.Error()))

	// Scheduler jobs can still use the tracker and database pool. Keep Serve
	// alive until they stop, while giving HTTP requests their own full drain
	// deadline.
	schedulerStopped := make(chan struct{})
	go func() {
		runtime.stopScheduler()
		close(schedulerStopped)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	if err := runtime.server.Shutdown(ctx); err != nil {
		closeErr := runtime.server.Close()
		runtime.resourcesUnsafe = true
		return errors.Join(
			fmt.Errorf("shutdown HTTP server: %w", err),
			closeErr,
		)
	}
	select {
	case <-schedulerStopped:
		return nil
	case <-ctx.Done():
		runtime.resourcesUnsafe = true
		return fmt.Errorf("shutdown scheduler: %w", ctx.Err())
	}
}

func (runtime *Runtime) stopScheduler() {
	if runtime.scheduler == nil {
		return
	}
	scheduler := runtime.scheduler
	runtime.scheduler = nil
	scheduler.Stop()
}

func (runtime *Runtime) closeResources() error {
	if runtime.resourcesUnsafe {
		// cmd.Execute exits after this fatal shutdown error, which stops the
		// remaining handler or worker before the operating system releases its
		// resources. Closing the pool here would race those goroutines.
		return nil
	}
	var err error
	if runtime.tracker != nil {
		err = runtime.tracker.Close()
	}
	err = errors.Join(err, database.ClosePool(runtime.api.db))
	return err
}
