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

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/analytics"
	"github.com/moto-nrw/project-phoenix/database"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	organizationModule "github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/observability"
	"github.com/moto-nrw/project-phoenix/services/scheduler"
)

const shutdownTimeout = 30 * time.Second

// ServeConfig contains the typed inputs for the production Serve root.
type ServeConfig struct {
	Port         string
	FrontendURL  string
	PublicAPIURL string
	EnableCORS   bool
	Logger       *slog.Logger
	// SentryPyrePortalDSN is the DSN of the pyreportal project the kiosks'
	// error reports go to (#3645); empty when the backend runs without
	// Sentry.
	SentryPyrePortalDSN string
	// Process selects what this root runs (#2726). The zero value keeps the
	// HTTP API with its embedded Worker.
	Process ServeProcess
}

// ServeProcess is what one Serve root runs. Every Worker, embedded or
// standalone, leads only while it holds the Worker lease, so the processes
// may overlap during a cutover.
type ServeProcess string

const (
	// ServeAPIWithWorker serves the HTTP API and runs the embedded Worker.
	ServeAPIWithWorker ServeProcess = ""
	// ServeAPIOnly serves the HTTP API once a standalone Worker runs the jobs.
	ServeAPIOnly ServeProcess = "api"
	// ServeWorkerOnly runs the standalone Worker behind its probes:
	// /health while the process lives, /ready only while it holds the lease,
	// and /internal/metrics.
	ServeWorkerOnly ServeProcess = "worker"
)

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
	if strings.TrimSpace(config.FrontendURL) == "" {
		return nil, fmt.Errorf("serve dependency frontend URL is required")
	}
	if strings.TrimSpace(config.PublicAPIURL) == "" {
		return nil, fmt.Errorf("serve dependency public API URL is required")
	}
	if err := validateServeProcess(config.Process); err != nil {
		return nil, err
	}

	config.Logger.Info("initializing API server")

	api, err := New(config.EnableCORS, config.PublicAPIURL, config.Logger, config.FrontendURL, config.SentryPyrePortalDSN)
	if err != nil {
		return nil, err
	}

	runtime := &Runtime{
		api:            api,
		capacityLogger: newRuntimeCapacityLogger(api, config.Logger),
		tracker:        api.Services.Tracker,
		logger:         config.Logger,
	}
	var worker *scheduler.Scheduler
	if config.Process == ServeAPIOnly {
		config.Logger.Info("embedded worker disabled; a standalone worker runs the jobs")
	} else {
		worker, err = newWorker(api, config.Logger)
		if err != nil {
			return nil, errors.Join(err, runtime.closeResources())
		}
		runtime.worker = worker
	}
	runtime.server = &http.Server{
		Addr:    resolveListenAddr(config.Port),
		Handler: processHandler(config.Process, runtime.Handler(), worker, api.metricsBearerToken),
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
	billing, err := newOperatorBilling(logger)
	if err != nil {
		return nil, fmt.Errorf("compose worker billing: %w", err)
	}
	lease, err := newWorkerLease(api.tenantRuntime)
	if err != nil {
		return nil, fmt.Errorf("compose worker lease: %w", err)
	}
	runtime, err := schedulerTenantRuntime(api.tenantRuntime)
	if err != nil {
		return nil, fmt.Errorf("compose worker tenant runtime: %w", err)
	}
	if err := verifySchedulerSettingKeys(); err != nil {
		return nil, err
	}
	settings, err := schedulerSettings(api)
	if err != nil {
		return nil, err
	}
	deps := workerRuntimeDependencies(api, logger, billing)
	deps.TenantRuntime = runtime
	deps.Settings = settings
	deps.Lease = lease
	addWorkerServiceDependencies(&deps, api)
	addWorkerRepositoryDependencies(&deps, api)
	return scheduler.NewWorker(deps)
}

func workerRuntimeDependencies(api *API, logger *slog.Logger, billing organizationModule.BillingReport) scheduler.WorkerDependencies {
	return scheduler.WorkerDependencies{
		Logger:                 logger.With("service", "scheduler"),
		Getenv:                 os.Getenv,
		SchoolRepo:             schedulerTenantDirectory{schools: api.Services.Schools, billing: billing},
		TenantRuntimeObserver:  observability.RecordTenantRuntimeEvent,
		UnitOfWorkObserver:     observability.RecordUnitOfWorkEvent,
		Tracer:                 workerTracer(api),
		StaffDocumentCleaner:   api.StaffAdmin,
		StudentDocumentCleaner: api.Students,
		FileStoreCleaner:       fileStoreCleaner(api),
	}
}

// fileStoreCleaner hands the File Storage sweep to the worker only when the
// module was composed. A nil module behind a non-nil interface would pass the
// scheduler's nil check and fail on the first tick.
func fileStoreCleaner(api *API) scheduler.FileStoreCleaner {
	if api.Services.FileStore == nil {
		return nil
	}
	return api.Services.FileStore
}

func addWorkerServiceDependencies(deps *scheduler.WorkerDependencies, api *API) {
	services := api.Services
	deps.Active = services.Active
	deps.ActiveCleanup = services.ActiveCleanup
	deps.AuthCleanup = services.AuthMaintenanceRuntime()
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
	deps.TimeTrackingCleanup = schedulerTimeTrackingCleanupPort(services.TimeTrackingCleanup)
	deps.StudentChangeLogCleanup = schedulerStudentChangeLogCleanup(api)
	deps.PWAUsageCleanup = services.PWAUsage
	deps.StaffMessageCleanup = staffMessageCleanup(services.StaffMessaging)
	deps.EnrollmentRejectedCleanup = services.EnrollmentRejectedCleanup
	deps.AutoStart = services.AutoStart
	deps.AutoEnd = services.AutoEnd
	deps.TimetableBridge = services.TimetableBridge
	deps.StudentLifecycleAudit = schedulerStudentAudit(api)
	deps.CareExitEffector = services.CareLifecycle
	deps.OutboxWorker = services.EmailOutboxWorker
	deps.AppointmentReminders = services.Reminders
	var rollover scheduler.RolloverDeadlineRunner
	if services.EnrollmentRollover != nil {
		rollover = scheduler.NewRolloverDeadlineRunner(func(ctx context.Context, asOf time.Time) (any, error) {
			return services.EnrollmentRollover.RunDeadlineWorker(ctx, asOf)
		})
	}
	deps.RolloverDeadlineRunner = rollover
}

func staffMessageCleanup(service communication.StaffMessagingRuntime) scheduler.StaffMessageCleanup {
	if service == nil {
		return nil
	}
	return func(ctx context.Context) (scheduler.StaffMessageCleanupResult, error) {
		result, err := service.CleanupExpiredStaffMessages(ctx)
		return scheduler.StaffMessageCleanupResult{
			MessagesDeleted: result.MessagesDeleted,
			ThreadsDeleted:  result.ThreadsDeleted,
			RetentionDays:   result.RetentionDays,
		}, err
	}
}

func addWorkerRepositoryDependencies(deps *scheduler.WorkerDependencies, api *API) {
	deps.BookingConsistency = schedulerBookingConsistency(api)
	deps.InstanceRepo = schedulerDayInstances(api)
	deps.InstanceRoomRepo = schedulerExistingRooms(api)
	deps.InstanceStudentRepo = api.repos.InstanceStudent
	deps.StudentStatusDayRepo = api.repos.StudentStatusDay
	deps.OverdueBroadcaster = api.Services.RealtimeHub
	deps.StudentLifecycleRepo = schedulerStudentLifecycle(api)
	deps.ReminderNotifications = scheduler.ReminderNotificationDeps{
		Computer:     api.Services.Reminders,
		Notifier:     api.Services.Notifications,
		Preferences:  api.Services.NotificationPreferences,
		Staff:        api.repos.Staff,
		Accounts:     api.Services.Auth,
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

func validateServeProcess(process ServeProcess) error {
	switch process {
	case ServeAPIWithWorker, ServeAPIOnly, ServeWorkerOnly:
		return nil
	default:
		return fmt.Errorf("unknown serve process %q", process)
	}
}

// processHandler selects what the listener serves: the HTTP API, or for the
// standalone Worker only its probes and metrics.
func processHandler(process ServeProcess, apiHandler http.Handler, worker readyWorker, metricsToken string) http.Handler {
	if process == ServeWorkerOnly {
		return workerProbeHandler(worker.Ready, metricsToken)
	}
	return apiHandler
}

// readyWorker is a Worker that reports whether it holds the lease.
type readyWorker interface {
	Ready() bool
}

func workerProbeHandler(ready func() bool, metricsToken string) http.Handler {
	router := chi.NewRouter()
	router.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("OK"))
	})
	router.Get("/ready", func(w http.ResponseWriter, _ *http.Request) {
		if !ready() {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("standby"))
			return
		}
		_, _ = w.Write([]byte("ready"))
	})
	if metricsToken != "" {
		router.With(metricsAuthMiddleware(metricsToken)).Handle("/internal/metrics", metricsHandler())
	}
	return router
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
