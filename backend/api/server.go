package api

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/moto-nrw/project-phoenix/analytics"
	"github.com/moto-nrw/project-phoenix/database"
	"github.com/moto-nrw/project-phoenix/observability"
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
	worker          backgroundWorker
	listen          func(network, address string) (net.Listener, error)
	capacityLogger  *capacityLogger
	tracker         analytics.Tracker
	logger          *slog.Logger
	resourcesUnsafe bool
}

// backgroundWorker is the lifecycle contract Runtime needs from its worker
// graph. Serve must wait for Stop before it releases shared resources.
type backgroundWorker interface {
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
		capacityLogger: newRuntimeCapacityLogger(api, config.Logger),
		tracker:        api.Services.Tracker,
		logger:         config.Logger,
	}
	worker, err := newWorker(api, config.Logger)
	if err != nil {
		return nil, errors.Join(err, runtime.closeResources())
	}
	runtime.worker = worker
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

// newWorker assembles the embedded Worker root from one typed dependency value.
func newWorker(api *API, logger *slog.Logger) (*scheduler.Scheduler, error) {
	if api == nil || api.Services == nil || api.repos == nil {
		return nil, fmt.Errorf("worker API graph is required")
	}
	deps := workerRuntimeDependencies(api, logger)
	addWorkerServiceDependencies(&deps, api)
	addWorkerRepositoryDependencies(&deps, api)
	return scheduler.NewWorker(deps)
}

func workerRuntimeDependencies(api *API, logger *slog.Logger) scheduler.WorkerDependencies {
	return scheduler.WorkerDependencies{
		Logger:                 logger.With("service", "scheduler"),
		Getenv:                 os.Getenv,
		DB:                     api.db,
		SchoolRepo:             api.repos.School,
		TenantRuntime:          &api.tenantRuntime,
		TenantRuntimeObserver:  observability.RecordTenantRuntimeEvent,
		UnitOfWorkObserver:     observability.RecordUnitOfWorkEvent,
		Tracer:                 workerTracer(api),
		Settings:               api.Services.Settings,
		StaffDocumentCleaner:   api.StaffAdmin,
		StudentDocumentCleaner: api.Students,
		FileStoreCleaner:       api.FileStore,
	}
}

func addWorkerServiceDependencies(deps *scheduler.WorkerDependencies, api *API) {
	services := api.Services
	deps.Active = services.Active
	deps.ActiveCleanup = services.ActiveCleanup
	deps.AuthCleanup = services.Auth
	deps.InvitationCleanup = services.Invitation
	deps.EmailChangeCleanup = services.OperatorAuth
	deps.OperatorInvitationCleanup = services.OperatorInvitation
	deps.WorkSessionCleanup = services.WorkSession
	deps.BreakAutoEnder = services.WorkSession
	deps.AutoCheckouter = services.WorkSession
	deps.FeedbackCleaner = api.feedback
	deps.UnregisteredScanCleaner = services.UnregisteredTagScans
	deps.Materializer = services.Materialization
	deps.TimetableCleanup = services.TimetableCleanup
	deps.CalendarFeedCleanup = services.CalendarFeedCleanup
	deps.TimeTrackingCleanup = services.TimeTrackingCleanup
	deps.StudentChangeLogCleanup = services.StudentChangeLogCleanup
	deps.PWAUsageCleanup = services.PWAUsage
	deps.StaffMessageCleanup = func(ctx context.Context) (scheduler.StaffMessageCleanupResult, error) {
		result, err := services.StaffMessaging.CleanupExpiredStaffMessages(ctx)
		return scheduler.StaffMessageCleanupResult{
			MessagesDeleted: result.MessagesDeleted,
			ThreadsDeleted:  result.ThreadsDeleted,
			RetentionDays:   result.RetentionDays,
		}, err
	}
	deps.EnrollmentRejectedCleanup = services.EnrollmentRejectedCleanup
	deps.AutoStart = services.AutoStart
	deps.AutoEnd = services.AutoEnd
	deps.TimetableBridge = services.TimetableBridge
	deps.StudentLifecycleAudit = services.StudentAudit
	deps.CareExitEffector = services.CareLifecycle
	deps.OutboxWorker = services.EmailOutboxWorker
	deps.AppointmentReminders = services.Calendar
	var rollover scheduler.RolloverDeadlineRunner
	if services.EnrollmentRollover != nil {
		rollover = scheduler.NewRolloverDeadlineRunner(func(ctx context.Context, asOf time.Time) (any, error) {
			return services.EnrollmentRollover.RunDeadlineWorker(ctx, asOf)
		})
	}
	deps.RolloverDeadlineRunner = rollover
}

func addWorkerRepositoryDependencies(deps *scheduler.WorkerDependencies, api *API) {
	deps.BookingConsistency = api.repos.BookingConsistency
	deps.InstanceRepo = api.repos.ActivityInstance
	deps.InstanceRoomRepo = api.repos.Room
	deps.InstanceStudentRepo = api.repos.InstanceStudent
	deps.StudentStatusDayRepo = api.repos.StudentStatusDay
	deps.OverdueBroadcaster = api.Services.RealtimeHub
	deps.StudentLifecycleRepo = api.repos.Student
	deps.ReminderNotifications = scheduler.ReminderNotificationDeps{
		Computer:     api.Services.Reminders,
		Notifier:     api.Services.Notifications,
		Preferences:  api.Services.NotificationPreferences,
		Staff:        api.repos.Staff,
		Accounts:     api.repos.Account,
		WorkSessions: api.repos.WorkSession,
	}
}

func workerTracer(api *API) scheduler.WorkerTracer {
	return scheduler.WorkerTracer{
		StartJob: func(ctx context.Context, operation string) (context.Context, error) {
			ctx, _, err := api.tracer.StartJob(ctx, operation)
			return ctx, err
		},
		Logger: api.tracer.Logger,
		Failure: func(ctx context.Context, operation, outcome string, err error) {
			api.tracer.Failure(ctx, "worker", operation, outcome, err)
		},
		Run: func(jobID scheduler.JobID, outcome string, duration time.Duration) {
			observability.RecordWorkerRunEvent(string(jobID), outcome, duration)
		},
		Batch: func(event scheduler.TenantBatchEvidence) {
			observability.RecordWorkerTenantBatchEvent(
				string(event.JobID),
				event.Duration,
				event.Processed,
				event.Failed,
				event.Retries,
				event.Backlog,
				event.PoolWait,
			)
		},
		Backlog: func(jobID scheduler.JobID, backlog int) {
			observability.SetWorkerTenantBatchBacklog(string(jobID), backlog)
		},
	}
}

// Serve runs the HTTP server and embedded worker until ctx is cancelled.
func (runtime *Runtime) Serve(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("serve context is required")
	}
	serveErr, err := runtime.startHTTP(ctx)
	if err != nil || serveErr == nil {
		return err
	}
	capacityCtx, stopCapacityLogger := context.WithCancel(ctx)
	capacityStopped := runtime.startBackground(capacityCtx)
	defer func() {
		stopCapacityLogger()
		<-capacityStopped
	}()

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

func (runtime *Runtime) startHTTP(ctx context.Context) (<-chan error, error) {
	listen := runtime.listen
	if listen == nil {
		listen = net.Listen
	}
	listener, err := listen("tcp", runtime.server.Addr)
	if err != nil {
		return nil, fmt.Errorf("listen on %s: %w", runtime.server.Addr, err)
	}
	if err := ctx.Err(); err != nil {
		if closeErr := listener.Close(); closeErr != nil {
			return nil, fmt.Errorf("close canceled listener: %w", closeErr)
		}
		return nil, nil
	}
	serveErr := make(chan error, 1)
	servingListener := &startupListener{Listener: listener, started: make(chan struct{})}
	go func() {
		runtime.logger.Info("server listening", slog.String("addr", runtime.server.Addr))
		serveErr <- runtime.server.Serve(servingListener)
	}()
	select {
	case <-servingListener.started:
	case err := <-serveErr:
		return nil, runtime.handleServeExit(err)
	}
	return serveErr, nil
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
	if capacityCtx.Err() != nil {
		close(capacityStopped)
		return capacityStopped
	}
	if runtime.capacityLogger != nil {
		runtime.capacityLogger.LogSnapshot()
		go func() {
			runtime.capacityLogger.Start(capacityCtx)
			close(capacityStopped)
		}()
	} else {
		close(capacityStopped)
	}
	if runtime.worker != nil && capacityCtx.Err() == nil {
		runtime.worker.Start()
	}
	return capacityStopped
}

func (runtime *Runtime) shutdown(reason error) error {
	return runtime.shutdownWithTimeout(reason, shutdownTimeout)
}

func (runtime *Runtime) shutdownWithTimeout(reason error, timeout time.Duration) error {
	runtime.logger.Info("server shutting down", slog.String("reason", reason.Error()))

	// Worker jobs can still use the tracker and database pool. Keep Serve
	// alive until they stop, while giving HTTP requests their own full drain
	// deadline.
	workerStopped := make(chan struct{})
	go func() {
		runtime.stopWorker()
		close(workerStopped)
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
	case <-workerStopped:
		return nil
	case <-ctx.Done():
		runtime.resourcesUnsafe = true
		return fmt.Errorf("shutdown worker: %w", ctx.Err())
	}
}

func (runtime *Runtime) stopWorker() {
	if runtime.worker == nil {
		return
	}
	worker := runtime.worker
	runtime.worker = nil
	worker.Stop()
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
