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
	apiCommon "github.com/moto-nrw/project-phoenix/api/common"
	carePlanCompose "github.com/moto-nrw/project-phoenix/modules/careplan/compose"
	"github.com/moto-nrw/project-phoenix/modules/communication"
	communicationCompose "github.com/moto-nrw/project-phoenix/modules/communication/composition"
	deliveryCompose "github.com/moto-nrw/project-phoenix/modules/delivery/compose"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	exportTransferCompose "github.com/moto-nrw/project-phoenix/modules/exporttransfer/compose"
	filestorageCompose "github.com/moto-nrw/project-phoenix/modules/filestorage/compose"
	identityaccessCompose "github.com/moto-nrw/project-phoenix/modules/identityaccess/compose"
	organizationModule "github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	schedulerRuntimeCompose "github.com/moto-nrw/project-phoenix/modules/schedulerruntime/compose"
	schedulerPorts "github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"
	settingsValues "github.com/moto-nrw/project-phoenix/modules/settings"
	settingsCompose "github.com/moto-nrw/project-phoenix/modules/settings/compose"
	presenceCompose "github.com/moto-nrw/project-phoenix/modules/studentpresence/compose"
	workforceCompose "github.com/moto-nrw/project-phoenix/modules/workforce/compose"
	"github.com/moto-nrw/project-phoenix/observability"
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
	graph           *serveGraph
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

	graph, reads, tracker, err := newServeGraph(config)
	if err != nil {
		return nil, err
	}

	runtime := &Runtime{
		graph:          graph,
		capacityLogger: newRuntimeCapacityLogger(graph, config.Logger),
		tracker:        tracker,
		logger:         config.Logger,
	}
	var worker schedulerPorts.Worker
	if config.Process == ServeAPIOnly {
		config.Logger.Info("embedded worker disabled; a standalone worker runs the jobs")
	} else {
		worker, err = newWorker(graph, reads, config.Logger)
		if err != nil {
			return nil, errors.Join(err, runtime.closeResources())
		}
		runtime.worker = worker
	}
	runtime.server = &http.Server{
		Addr:    resolveListenAddr(config.Port),
		Handler: processHandler(config.Process, runtime.Handler(), worker, graph.metricsBearerToken),
		// ReadTimeout stays modest to protect against slowloris attacks,
		// but WriteTimeout must be disabled to allow long-lived SSE streams.
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 0,
		IdleTimeout:  0,
	}

	return runtime, nil
}

func newRuntimeCapacityLogger(graph *serveGraph, logger *slog.Logger) *capacityLogger {
	return newCapacityLogger(func() dbCapacityStats {
		stats := graph.db.Capacity()
		return dbCapacityStats{
			openConnections:   stats.OpenConnections,
			inUse:             stats.InUse,
			idle:              stats.Idle,
			waitCount:         stats.WaitCount,
			waitDuration:      stats.WaitDuration,
			maxIdleClosed:     stats.MaxIdleClosed,
			maxLifetimeClosed: stats.MaxLifetimeClosed,
		}
	}, graph.services.RealtimeHub, graph.metrics, logger.With("component", "capacity"))
}

// Handler returns the fully assembled production HTTP graph.
func (runtime *Runtime) Handler() http.Handler {
	return runtime.graph
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
func newWorker(graph *serveGraph, reads workerReads, logger *slog.Logger) (schedulerPorts.Worker, error) {
	if graph == nil || graph.services == nil || reads.carePlan == nil {
		return nil, fmt.Errorf("worker serve graph is required")
	}
	billing, err := newOperatorBilling(logger)
	if err != nil {
		return nil, fmt.Errorf("compose worker billing: %w", err)
	}
	lease, err := newWorkerLease(graph.db.UnitOfWork())
	if err != nil {
		return nil, fmt.Errorf("compose worker lease: %w", err)
	}
	runtime, err := schedulerRuntimeCompose.NewTenantRuntime(graph.db.UnitOfWork())
	if err != nil {
		return nil, fmt.Errorf("compose worker tenant runtime: %w", err)
	}
	if err := verifySchedulerSettingKeys(); err != nil {
		return nil, err
	}
	settings, err := schedulerSettings(graph)
	if err != nil {
		return nil, err
	}
	students, err := newStudentRetention(graph.services.StudentDeletion, graph.services.Settings, logger.With("service", "student_retention"))
	if err != nil {
		return nil, fmt.Errorf("compose worker student retention: %w", err)
	}
	staff, err := newStaffRetention(graph.db, graph.services.Audit, graph.services.Settings, logger.With("service", "staff_retention"))
	if err != nil {
		return nil, fmt.Errorf("compose worker staff retention: %w", err)
	}
	presence, err := presenceCompose.New(presenceCompose.Dependencies{DB: graph.db.Pool(), Observe: presenceCompose.FailureLogger()})
	if err != nil {
		return nil, fmt.Errorf("compose worker attendance retention: %w", err)
	}
	retention, err := newWorkerRetention(retentionOwners{
		deletionLog:          graph.services.Audit,
		settings:             graph.services.Settings,
		unregisteredTagScans: graph.services.IoT.Fleet(),
		statusDays:           reads.statusDayRetention,
		parentMessages:       communicationCompose.NewParentMessageRetention(graph.db.Pool(), settingsValues.NewParentMessageRetentionValues(graph.services.Settings)),
		settledEnrollments:   graph.services.EnrollmentRejectedCleanup,
		staffSickNotes:       workforceCompose.NewSickNoteRetention(StaffSickNoteRetentionSettings{Settings: graph.services.Settings}),
		auditLedgers:         graph.services.Audit,
		deliveries:           deliveryCompose.NewRetention(),
		fileCleanups:         filestorageCompose.NewCleanupRetention(),
		staffCleanups:        workforceCompose.NewCleanupRetention(),
		carePlanCleanups:     carePlanCompose.NewCleanupRetention(),
		submissionLimits:     enrollmentCompose.New(),
		settingAudit:         settingsCompose.NewAuditRetention(),
		exportTransfers:      exportTransferCompose.NewJournalRetention(),
	}, students, staff, presence, newGuardianRetention(graph.db, reads.identity))
	if err != nil {
		return nil, err
	}
	bookings, err := schedulerBookingConsistency(graph, reads)
	if err != nil {
		return nil, err
	}
	reminderNotifications, err := newPersonalReminderTick(graph, reads, runtime, logger.With("service", "scheduler"))
	if err != nil {
		return nil, fmt.Errorf("compose personal reminder notifications: %w", err)
	}
	return schedulerRuntimeCompose.NewWorker(workerDependencies(graph, reads, reminderNotifications, workerRoots{
		logger:    logger.With("service", "scheduler"),
		billing:   billing,
		lease:     lease,
		runtime:   runtime,
		settings:  settings,
		retention: retention,
		bookings:  bookings,
	}))
}

// workerRoots are the Worker inputs newWorker composes before the literal.
type workerRoots struct {
	logger    *slog.Logger
	billing   organizationModule.BillingReport
	lease     schedulerPorts.WorkerLease
	runtime   schedulerPorts.TenantRuntime
	settings  schedulerPorts.SettingsResolver
	retention workerRetention
	bookings  schedulerPorts.BookingConsistencyAudit
}

// workerDependencies builds the Worker's complete typed input in one literal;
// no dependency is filled in after construction (#2749).
func workerDependencies(graph *serveGraph, reads workerReads, reminderNotifications schedulerPorts.ReminderNotificationTick, roots workerRoots) schedulerPorts.WorkerDependencies {
	services, identity := graph.services, reads.identity
	return schedulerPorts.WorkerDependencies{
		Logger:                    roots.logger,
		Getenv:                    os.Getenv,
		SchoolRepo:                schedulerTenantDirectory{schools: reads.schools, billing: roots.billing},
		TenantRuntime:             roots.runtime,
		TenantRuntimeObserver:     observability.RecordTenantRuntimeEvent,
		UnitOfWorkObserver:        observability.RecordUnitOfWorkEvent,
		Tracer:                    workerTracer(graph),
		Settings:                  roots.settings,
		Lease:                     roots.lease,
		RetentionSteps:            roots.retention.steps,
		RetentionLog:              roots.retention.log,
		CleanupRuns:               roots.retention.runs,
		StaffDocumentCleaner:      graph.documents.staff,
		StudentDocumentCleaner:    graph.documents.student,
		FileStoreCleaner:          fileStoreCleaner(graph),
		Active:                    newSchedulerSessionMaintenance(services.Active),
		ActiveCleanup:             newSchedulerPresenceCleanup(services.ActiveCleanup),
		AuthCleanup:               &identityaccessCompose.SessionMaintenance{AccountSessionMaintenance: identity, PasswordResets: identity},
		InvitationCleanup:         identity,
		EmailChangeCleanup:        identity,
		OperatorInvitationCleanup: identity,
		OperatorRetention:         schedulerPorts.OperatorRetention{Ledger: services.Audit, Accounts: identity},
		AccountRetention:          identity,
		WorkSessionCleanup:        services.WorkSession.Jobs,
		BreakAutoEnder:            services.WorkSession.Jobs,
		AutoCheckouter:            services.WorkSession.Jobs,
		FeedbackCleaner:           graph.feedback,
		Materializer:              newSchedulerMaterializer(services.Materialization),
		TimetableCleanup:          newSchedulerTimetableCleanup(services.TimetableCleanup),
		CalendarFeedCleanup:       services.CalendarFeedCleanup,
		TimeTrackingCleanup:       schedulerTimeTrackingCleanupPort(services.TimeTrackingCleanup),
		StudentChangeLogCleanup:   schedulerStudentChangeLogCleanup(graph),
		PWAUsageCleanup:           pwaUsageCleanup(services.PWAUsage),
		StaffMessageCleanup:       staffMessageCleanup(services.StaffMessaging),
		EnrollmentRejectedCleanup: enrollmentRejectedCleanup{cleaner: services.EnrollmentRejectedCleanup},
		AutoStart:                 newSchedulerAutoStart(services.AutoStart),
		AutoEnd:                   newSchedulerAutoEnd(services.AutoEnd),
		TimetableBridge:           services.TimetableBridge,
		StudentLifecycleAudit:     schedulerStudentAudit(graph),
		CareExitEffector:          services.CareLifecycle,
		OutboxWorker:              services.EmailOutboxWorker,
		AppointmentReminders:      services.Reminders,
		RolloverDeadlineRunner:    rolloverDeadlineRunner(services.EnrollmentRollover),
		BookingConsistency:        roots.bookings,
		InstanceRepo:              schedulerDayInstances(reads.timetable),
		InstanceRoomRepo:          reads.existingRooms,
		InstanceStudentRepo:       reads.timetable.InstanceStudent,
		StudentStatusDayRepo:      reads.studentStatusDays,
		OverdueBroadcaster:        newSchedulerOverdueBroadcaster(services.RealtimeHub),
		StudentLifecycleRepo:      schedulerStudentLifecycle(reads),
		ReminderNotifications:     reminderNotifications,
	}
}

// fileStoreCleaner hands the File Storage sweep to the worker only when the
// module was composed. A nil module behind a non-nil interface would pass the
// scheduler's nil check and fail on the first tick.
func fileStoreCleaner(graph *serveGraph) schedulerPorts.FileStoreCleaner {
	if graph.services.FileStore == nil {
		return nil
	}
	return graph.services.FileStore
}

// rolloverDeadlineRunner hands the rollover deadline job to the worker only
// when the module was composed, so a nil module never hides behind a non-nil
// runner.
func rolloverDeadlineRunner(rollovers enrollmentOwner.Rollovers) schedulerPorts.RolloverDeadlineRunner {
	if rollovers == nil {
		return nil
	}
	return rolloverDeadlines{rollovers: rollovers}
}

// rolloverDeadlines runs Enrollment's rollover deadline job for the Worker,
// which discards the job's summary.
type rolloverDeadlines struct {
	rollovers enrollmentOwner.Rollovers
}

func (r rolloverDeadlines) RunDeadlineWorker(ctx context.Context, asOf time.Time) (any, error) {
	return r.rollovers.RunDeadlineWorker(ctx, asOf)
}

func staffMessageCleanup(service communication.StaffMessagingRuntime) schedulerPorts.StaffMessageCleanup {
	if service == nil {
		return nil
	}
	return func(ctx context.Context) (schedulerPorts.StaffMessageCleanupResult, error) {
		result, err := service.CleanupExpiredStaffMessages(ctx)
		return schedulerPorts.StaffMessageCleanupResult{
			MessagesDeleted: result.MessagesDeleted,
			ThreadsDeleted:  result.ThreadsDeleted,
			RetentionDays:   result.RetentionDays,
		}, err
	}
}

func workerTracer(graph *serveGraph) schedulerPorts.WorkerTracer {
	return schedulerPorts.WorkerTracer{
		StartJob: func(ctx context.Context, operation string) (context.Context, error) {
			ctx, _, err := graph.tracer.StartJob(ctx, operation)
			return ctx, err
		},
		Logger: graph.tracer.Logger,
		Failure: func(ctx context.Context, operation, outcome string, err error) {
			graph.tracer.Failure(ctx, "worker", operation, outcome, err)
		},
		Run: func(jobID schedulerPorts.JobID, outcome string, duration time.Duration) {
			observability.RecordWorkerRunEvent(string(jobID), outcome, duration)
		},
		Batch: func(event schedulerPorts.TenantBatchEvidence) {
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
		Backlog: func(jobID schedulerPorts.JobID, backlog int) {
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
	err = errors.Join(err, runtime.graph.db.Close())
	return err
}
