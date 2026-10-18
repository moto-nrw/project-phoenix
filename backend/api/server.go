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

	schedulerRuntimeCompose "github.com/moto-nrw/project-phoenix/modules/schedulerruntime/compose"

	carePlanModule "github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	communicationCompose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	deliveryModule "github.com/moto-nrw/project-phoenix/modules/delivery"
	"github.com/moto-nrw/project-phoenix/modules/devicefleet"
	facilitiesModule "github.com/moto-nrw/project-phoenix/modules/facilities"
	feedbackModule "github.com/moto-nrw/project-phoenix/modules/feedback"
	filestorageModule "github.com/moto-nrw/project-phoenix/modules/filestorage"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	peopleCompose "github.com/moto-nrw/project-phoenix/modules/peopledirectory/compose"
	peoplePorts "github.com/moto-nrw/project-phoenix/modules/peopledirectory/ports"
	calendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal/compose"
	settingsValues "github.com/moto-nrw/project-phoenix/modules/settings"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	workforceModule "github.com/moto-nrw/project-phoenix/modules/workforce"
	reminder "github.com/moto-nrw/project-phoenix/workflows/reminderdelivery"
	"github.com/moto-nrw/project-phoenix/workflows/studentdeletion"

	"github.com/go-chi/chi/v5"
	"github.com/moto-nrw/project-phoenix/analytics"
	apiCommon "github.com/moto-nrw/project-phoenix/api/common"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	schedulerPorts "github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"
	"github.com/moto-nrw/project-phoenix/observability"
	"github.com/moto-nrw/project-phoenix/realtime"
	"github.com/spf13/viper"
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
	server         *http.Server
	handler        http.Handler
	db             transactionruntime.Database
	worker         backgroundWorker
	listen         func(network, address string) (net.Listener, error)
	capacityLogger *capacityLogger
	tracker        processResource
	// workerStop stops the Worker once, whichever shutdown path comes first.
	workerStop      sync.Once
	logger          *slog.Logger
	resourcesUnsafe bool
}

// processResource is a process-scoped resource the Runtime releases on
// shutdown.
type processResource interface {
	Close() error
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
	metricsToken, err := observability.MetricsBearerTokenFromEnv(os.Getenv)
	if err != nil {
		return nil, err
	}
	db, tracker, err := openRuntimeResources(config)
	if err != nil {
		return nil, err
	}
	handler, worker, capacity, err := composeProcess(config, db, tracker, metricsToken)
	if err != nil {
		return nil, errors.Join(err, closeRuntimeResources(db, tracker))
	}
	if config.Process == ServeAPIOnly {
		config.Logger.Info("embedded worker disabled; a standalone worker runs the jobs")
	}
	return &Runtime{
		server: &http.Server{
			Addr:    resolveListenAddr(config.Port),
			Handler: processHandler(config.Process, handler, worker, metricsToken),
			// ReadTimeout stays modest to protect against slowloris attacks,
			// but WriteTimeout must be disabled to allow long-lived SSE streams.
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 0,
			IdleTimeout:  0,
		},
		handler: handler, db: db, worker: runtimeWorker(worker), capacityLogger: capacity, tracker: tracker, logger: config.Logger,
	}, nil
}

// runtimeWorker keeps a missing Worker a nil interface, so an API-only
// process starts and stops none.
func runtimeWorker(worker schedulerPorts.Worker) backgroundWorker {
	if worker == nil {
		return nil
	}
	return worker
}

// openRuntimeResources opens the process-scoped resources every root runs
// over: the least-privilege pool, after the schema check, and the product
// analytics.
func openRuntimeResources(config ServeConfig) (transactionruntime.Database, analytics.Tracker, error) {
	// Get database connection as phoenix_auth (least-privilege for serve)
	db, err := transactionruntime.OpenServeDatabase()
	if err != nil {
		return transactionruntime.Database{}, nil, err
	}
	// Fail fast on a partially migrated schema: the student repository selects
	// and writes its departure columns unconditionally (no per-request
	// information_schema probes, #2059), so the server must only start against
	// a fully migrated database.
	if err := peopleCompose.VerifyRetainedStudentSchema(context.Background(), db.Pool()); err != nil {
		return transactionruntime.Database{}, nil, errors.Join(err, db.Close())
	}
	if viper.GetBool("db_debug") {
		db.LogQueries(config.Logger.With("component", "database"))
	}
	tracker, err := newProductAnalytics(config.Logger)
	if err != nil {
		return transactionruntime.Database{}, nil, errors.Join(err, db.Close())
	}
	return db, tracker, nil
}

// closeRuntimeResources releases the analytics and the pool.
func closeRuntimeResources(db transactionruntime.Database, tracker processResource) error {
	var err error
	if tracker != nil {
		err = tracker.Close()
	}
	return errors.Join(err, db.Close())
}

// composeProcess builds the root the process runs: the server root with or
// without its embedded Worker, or the standalone Worker root behind its
// probes. All of them share one realtime hub.
func composeProcess(config ServeConfig, db transactionruntime.Database, tracker analytics.Tracker, metricsToken string) (http.Handler, schedulerPorts.Worker, *capacityLogger, error) {
	env, err := newProductionEnvironment(config.Logger, config.PublicAPIURL)
	if err != nil {
		return nil, nil, nil, err
	}
	hub := deliveryCompose.NewRealtimeHub(config.Logger.With("component", "sse-hub"))
	metrics, tracer := newHTTPMetrics(), newRuntimeTracer(config.Logger)
	capacity := newRuntimeCapacityLogger(db, hub, metrics, config.Logger)
	observability.RegisterDBStatsProvider(func() observability.DBStats {
		stats := db.Capacity()
		return observability.DBStats{
			OpenConnections:   stats.OpenConnections,
			InUse:             stats.InUse,
			Idle:              stats.Idle,
			WaitCount:         stats.WaitCount,
			WaitDuration:      stats.WaitDuration,
			MaxIdleClosed:     stats.MaxIdleClosed,
			MaxLifetimeClosed: stats.MaxLifetimeClosed,
		}
	})
	if config.Process == ServeWorkerOnly {
		worker, err := composeWorkerRoot(db, env, hub, tracer, tracker)
		return nil, worker, capacity, err
	}
	handler, worker, err := composeServeRoot(db, env, hub, config, tracker, metrics, tracer, metricsToken, config.Process != ServeAPIOnly)
	return handler, worker, capacity, err
}

func newRuntimeCapacityLogger(db transactionruntime.Database, hub *realtime.Hub, metrics *httpMetrics, logger *slog.Logger) *capacityLogger {
	return newCapacityLogger(func() dbCapacityStats {
		stats := db.Capacity()
		return dbCapacityStats{
			openConnections:   stats.OpenConnections,
			inUse:             stats.InUse,
			idle:              stats.Idle,
			waitCount:         stats.WaitCount,
			waitDuration:      stats.WaitDuration,
			maxIdleClosed:     stats.MaxIdleClosed,
			maxLifetimeClosed: stats.MaxLifetimeClosed,
		}
	}, hub, metrics, logger.With("component", "capacity"))
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

// newWorker starts the Worker over the owners a root built: the server root
// passes the owners it mounts, the Worker root the owners its jobs need.
// Each argument is one owner's value or a single-owner bundle; the Worker's
// own collaborators (lease, tenant runtime, settings snapshots, retention,
// booking audit, reminder tick, document sweeps) are built here.
func newWorker(
	db transactionruntime.Database, logger *slog.Logger, tracer *observability.Tracer, hub *realtime.Hub,
	settings settingsCompose.BatchSettingsService, audit auditCommand, fleet devicefleet.Capability,
	carePlan *carePlanModule.Module, persons peopledirectory.Capability, people PeopleRows,
	organizations organizationtenancy.Query, identity *identityaccess.Module, rooms facilitiesModule.Capability,
	timetableRepos timetableCompose.Repositories, workTime workforceModule.Capability, staffAdmin *workforceModule.StaffAdmin,
	workforce WorkforcePlanning, active studentpresence.Presence, reports PresenceReports, lifecycle TimetableLifecycle,
	days TimetableDays, materialization timetable.MaterializationCapability, schedules CarePlanSchedules,
	documents carePlanModule.StudentDocuments, services PeopleServices, studentAudit peoplePorts.StudentAuditService,
	notices EnrollmentNotices, flows EnrollmentFlows, studentDeletion *studentdeletion.Workflow, feedback *feedbackModule.Module,
	calendarApp calendarCompose.Application, notifications DeliveryNotifications, usage deliveryCompose.PWAUsageService,
	staffMessaging communication.StaffMessagingRuntime, outbox *deliveryModule.Worker, reminders reminder.Module,
	fileStore *filestorageModule.Module,
) (schedulerPorts.Worker, error) {
	billing, err := newOperatorBilling(logger)
	if err != nil {
		return nil, fmt.Errorf("compose worker billing: %w", err)
	}
	lease, err := newWorkerLease(db.UnitOfWork())
	if err != nil {
		return nil, fmt.Errorf("compose worker lease: %w", err)
	}
	runtime, err := schedulerRuntimeCompose.NewTenantRuntime(db.UnitOfWork())
	if err != nil {
		return nil, fmt.Errorf("compose worker tenant runtime: %w", err)
	}
	if err := verifySchedulerSettingKeys(); err != nil {
		return nil, err
	}
	resolver, err := schedulerSettings(settings)
	if err != nil {
		return nil, err
	}
	retention, err := workerRetentionOf(db, logger, settings, audit, fleet, carePlan, communicationCompose.NewParentMessageRetention(db.Pool(), settingsValues.NewParentMessageRetentionValues(settings)), notices.RejectedCleanup, studentDeletion)
	if err != nil {
		return nil, err
	}
	bookings, err := schedulerBookingConsistency(db, persons, carePlan)
	if err != nil {
		return nil, err
	}
	reminderNotifications, err := newPersonalReminderTick(reminders, notifications, people.Staff, identity, workTime, runtime, logger.With("service", "scheduler"))
	if err != nil {
		return nil, fmt.Errorf("compose personal reminder notifications: %w", err)
	}
	staffDocuments, studentDocuments, err := workerDocumentSweeps(db, logger, services.Users, staffAdmin, workforce.TimeTracking, documents)
	if err != nil {
		return nil, err
	}
	return schedulerRuntimeCompose.NewWorker(schedulerPorts.WorkerDependencies{
		Logger:                    logger.With("service", "scheduler"),
		Getenv:                    os.Getenv,
		SchoolRepo:                schedulerTenantDirectory{schools: organizations, billing: billing},
		TenantRuntime:             runtime,
		TenantRuntimeObserver:     observability.RecordTenantRuntimeEvent,
		UnitOfWorkObserver:        observability.RecordUnitOfWorkEvent,
		Tracer:                    workerTracer(tracer),
		Settings:                  resolver,
		Lease:                     lease,
		RetentionSteps:            retention.steps,
		RetentionLog:              retention.log,
		CleanupRuns:               retention.runs,
		StaffDocumentCleaner:      staffDocuments,
		StudentDocumentCleaner:    studentDocuments,
		FileStoreCleaner:          fileStore,
		Active:                    newSchedulerSessionMaintenance(active),
		ActiveCleanup:             newSchedulerPresenceCleanup(reports.ActiveCleanup),
		AuthCleanup:               &identityaccessCompose.SessionMaintenance{AccountSessionMaintenance: identity, PasswordResets: identity},
		InvitationCleanup:         identity,
		EmailChangeCleanup:        identity,
		OperatorInvitationCleanup: identity,
		OperatorRetention:         schedulerPorts.OperatorRetention{Ledger: audit, Accounts: identity},
		WorkSessionCleanup:        workforce.TimeTracking.Jobs,
		BreakAutoEnder:            workforce.TimeTracking.Jobs,
		AutoCheckouter:            workforce.TimeTracking.Jobs,
		FeedbackCleaner:           feedback,
		Materializer:              newSchedulerMaterializer(materialization),
		TimetableCleanup:          newSchedulerTimetableCleanup(lifecycle.Cleanup),
		CalendarFeedCleanup:       calendarApp,
		TimeTrackingCleanup:       schedulerTimeTrackingCleanupPort(workforce.TimeTrackingCleanup),
		StudentChangeLogCleanup:   schedulerStudentChangeLogCleanup(services.StudentChangeLogCleanup),
		PWAUsageCleanup:           pwaUsageCleanup(usage),
		StaffMessageCleanup:       staffMessageCleanup(staffMessaging),
		EnrollmentRejectedCleanup: enrollmentRejectedCleanup{cleaner: notices.RejectedCleanup},
		AutoStart:                 newSchedulerAutoStart(lifecycle.AutoStart),
		AutoEnd:                   newSchedulerAutoEnd(lifecycle.AutoEnd),
		TimetableBridge:           days.Bridge,
		StudentLifecycleAudit:     schedulerStudentAudit(studentAudit),
		CareExitEffector:          schedules.Lifecycle,
		OutboxWorker:              outbox,
		AppointmentReminders:      reminders,
		RolloverDeadlineRunner:    rolloverDeadlines{rollovers: flows.Rollovers},
		BookingConsistency:        bookings,
		InstanceRepo:              schedulerDayInstances(timetableRepos),
		InstanceRoomRepo:          schedulerExistingRooms(rooms),
		InstanceStudentRepo:       timetableRepos.InstanceStudent,
		StudentStatusDayRepo:      NewStudentStatusDayRepository(carePlan),
		OverdueBroadcaster:        newSchedulerOverdueBroadcaster(hub),
		StudentLifecycleRepo:      schedulerStudentLifecycle(NewStudentRootReads(people.Students)),
		ReminderNotifications:     reminderNotifications,
	})
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
	router.Get("/ready", func(w http.ResponseWriter, r *http.Request) {
		if !ready() {
			// A standby answers 503 in the shared error envelope; the
			// probe reads the status, the detail names the state.
			apiCommon.RenderStatus(w, r, http.StatusServiceUnavailable, "standby")
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
	runtime.workerStop.Do(runtime.worker.Stop)
}

func (runtime *Runtime) closeResources() error {
	if runtime.resourcesUnsafe {
		// cmd.Execute exits after this fatal shutdown error, which stops the
		// remaining handler or worker before the operating system releases its
		// resources. Closing the pool here would race those goroutines.
		return nil
	}
	return closeRuntimeResources(runtime.db, runtime.tracker)
}
