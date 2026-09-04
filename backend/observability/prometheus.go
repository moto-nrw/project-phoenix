package observability

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// DBStats is the database-capacity snapshot consumed by metrics. It keeps the
// public observability interface independent of database/sql and ORM types.
type DBStats struct {
	OpenConnections   int
	InUse             int
	Idle              int
	WaitCount         int64
	WaitDuration      time.Duration
	MaxIdleClosed     int64
	MaxLifetimeClosed int64
}

type SSEStats struct {
	ClientsByTenant map[int64]int
}

type SSEStatsProvider interface {
	SnapshotSSEClientsByTenant() map[int64]int
}

// PWAUsageStat is one (tenant, portal) bucket of PWA standalone-usage
// counts (#2189).
type PWAUsageStat struct {
	TenantID        int64
	Portal          string
	StandaloneUsers int
	EligibleUsers   int
}

// PWAUsageStatsProvider supplies the standalone-usage counts on scrape.
// Implementations are expected to cache internally because the metrics
// adapter calls this on every scrape.
type PWAUsageStatsProvider interface {
	SnapshotUsageStats() ([]PWAUsageStat, error)
}

// PWAUsageStatsProviderFunc adapts a function to PWAUsageStatsProvider.
type PWAUsageStatsProviderFunc func() ([]PWAUsageStat, error)

// SnapshotUsageStats implements PWAUsageStatsProvider.
func (f PWAUsageStatsProviderFunc) SnapshotUsageStats() ([]PWAUsageStat, error) { return f() }

var (
	appHTTPRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "phoenix_backend_http_requests_total",
			Help: "Backend HTTP requests by route and status class.",
		},
		[]string{"method", "route", "status_class"},
	)
	appHTTPDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "phoenix_backend_http_request_duration_seconds",
			Help:    "Backend HTTP request duration by route.",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"method", "route"},
	)
	appHTTPActive = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "phoenix_backend_http_active_requests",
			Help: "Currently active backend HTTP requests.",
		},
	)
	tenantHTTPRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "phoenix_tenant_http_requests_total",
			Help: "Tenant-scoped backend requests by tenant, route, and status class.",
		},
		[]string{"tenant_id", "scope", "method", "route", "status_class", "tx_outcome"},
	)
	tenantHTTPDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "phoenix_tenant_http_request_duration_seconds",
			Help:    "Tenant-scoped backend request duration.",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"tenant_id", "scope", "method", "route"},
	)
	tenantRuntimeEvents = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "phoenix_tenant_runtime_events_total",
			Help: "Rejected tenant entry points and tenant transaction failures.",
		},
		[]string{"entry_point", "outcome"},
	)
	unitOfWorkDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "phoenix_unit_of_work_duration_seconds",
			Help:    "Transaction duration by entry point and result.",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"entry_point", "result"},
	)
	unitOfWorkRollbacks = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "phoenix_unit_of_work_rollbacks_total",
			Help: "Rolled-back transactions by entry point.",
		},
		[]string{"entry_point"},
	)
	unitOfWorkRetries = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "phoenix_unit_of_work_retries_total",
			Help: "Deadlock and serialization retries owned by an outer UnitOfWork.",
		},
		[]string{"entry_point"},
	)
	unitOfWorkPoolWait = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "phoenix_unit_of_work_pool_wait_seconds",
			Help:    "Database-pool wait attributed to UnitOfWork execution.",
			Buckets: []float64{0, 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
		},
		[]string{"entry_point"},
	)
	unitOfWorkLockWait = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "phoenix_unit_of_work_lock_wait_seconds",
			Help:    "Explicit transaction-lock acquisition time by entry point.",
			Buckets: []float64{0.0001, 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
		},
		[]string{"entry_point"},
	)
	workerJobDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "phoenix_worker_job_duration_seconds",
			Help:    "Embedded Worker job run duration by stable job ID and outcome.",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60, 300},
		},
		[]string{"job_id", "outcome"},
	)
	workerJobMaxDuration = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "phoenix_worker_job_max_duration_seconds",
			Help: "Longest observed embedded Worker job run by stable job ID.",
		},
		[]string{"job_id"},
	)
	workerTenantBatchDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "phoenix_worker_tenant_batch_duration_seconds",
			Help:    "Bounded tenant batch duration by stable job ID.",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60},
		},
		[]string{"job_id"},
	)
	workerTenantBatchTenants = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "phoenix_worker_tenant_batch_tenants_total",
			Help: "Tenants processed by bounded Worker batches, split by result.",
		},
		[]string{"job_id", "result"},
	)
	workerTenantBatchRetries = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "phoenix_worker_tenant_batch_retries_total",
			Help: "Deadlock and serialization retries within bounded tenant batches.",
		},
		[]string{"job_id"},
	)
	workerTenantBatchBacklog = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "phoenix_worker_tenant_batch_backlog",
			Help: "Tenants not yet attempted in the current Worker job run.",
		},
		[]string{"job_id"},
	)
	workerTenantBatchPoolWait = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "phoenix_worker_tenant_batch_pool_wait_seconds",
			Help:    "Database-pool wait attributed to a bounded tenant batch.",
			Buckets: []float64{0, 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
		},
		[]string{"job_id"},
	)
	settingsLookups = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "phoenix_settings_lookups_total",
			Help: "Settings lookups by registry key, cache path, and outcome.",
		},
		[]string{"key", "cache", "outcome"},
	)
	settingsLookupDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "phoenix_settings_lookup_duration_seconds",
			Help:    "Settings resolution duration by registry key.",
			Buckets: []float64{0.0001, 0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25},
		},
		[]string{"key"},
	)
	settingsSideEffectFailures = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "phoenix_settings_side_effect_failures_total",
			Help: "Transactional settings side-effect failures by registry key.",
		},
		[]string{"key"},
	)
	mealPlanOperations = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "phoenix_meal_plan_operations_total", Help: "Meal Plan operations by operation and outcome."},
		[]string{"operation", "outcome"},
	)
	mealPlanDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "phoenix_meal_plan_operation_duration_seconds", Help: "Meal Plan read and write duration by operation.", Buckets: []float64{0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25}},
		[]string{"operation"},
	)
	mealPlanQueries = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "phoenix_meal_plan_queries_total", Help: "Persistence queries issued by Meal Plan operations."},
		[]string{"operation"},
	)
	mealPlanRowsChanged = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "phoenix_meal_plan_rows_changed_total", Help: "Rows changed by Meal Plan commands."},
		[]string{"operation"},
	)
	mealPlanStatementDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "phoenix_meal_plan_statement_duration_seconds",
			Help:    "Cumulative Meal Plan write-statement duration by operation.",
			Buckets: []float64{0.0001, 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
		},
		[]string{"operation"},
	)
	organizationTenancyOperations = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "phoenix_organization_tenancy_operations_total", Help: "Organization and Tenancy operations by operation, outcome, and stable error code."},
		[]string{"operation", "outcome", "code"},
	)
	organizationTenancyDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "phoenix_organization_tenancy_operation_duration_seconds", Help: "Organization and Tenancy operation duration by operation.", Buckets: []float64{0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25}},
		[]string{"operation"},
	)
	organizationTenancyQueries = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "phoenix_organization_tenancy_queries_total", Help: "Persistence queries issued by Organization and Tenancy operations."},
		[]string{"operation"},
	)
	organizationTenancyRowsChanged = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "phoenix_organization_tenancy_rows_changed_total", Help: "Rows changed by Organization and Tenancy commands."},
		[]string{"operation"},
	)
	organizationTenancyStatementDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "phoenix_organization_tenancy_statement_duration_seconds", Help: "Cumulative Organization and Tenancy database-statement duration by operation, used as a lock-wait upper bound.", Buckets: []float64{0.0001, 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5}},
		[]string{"operation"},
	)
	peopleDirectoryOperations = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "phoenix_people_directory_operations_total", Help: "People Directory operations by operation, outcome, and stable error code."},
		[]string{"operation", "outcome", "code"},
	)
	peopleDirectoryDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "phoenix_people_directory_operation_duration_seconds", Help: "People Directory operation duration by operation.", Buckets: []float64{0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25}},
		[]string{"operation"},
	)
	peopleDirectoryQueries = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "phoenix_people_directory_queries_total", Help: "Persistence queries issued by People Directory operations."},
		[]string{"operation"},
	)
	peopleDirectoryRowsChanged = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "phoenix_people_directory_rows_changed_total", Help: "Rows changed by People Directory commands."},
		[]string{"operation"},
	)
	peopleDirectoryStatementDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "phoenix_people_directory_statement_duration_seconds", Help: "Cumulative People Directory database-statement duration by operation, used as a lock-wait upper bound.", Buckets: []float64{0.0001, 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5}},
		[]string{"operation"},
	)
	peopleDirectoryHTTPResponses = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "phoenix_people_directory_http_responses_total", Help: "People Directory HTTP responses by actual status class and stable code."},
		[]string{"status_class", "code"},
	)
	guardianDirectoryHTTPResponses = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "phoenix_guardian_directory_http_responses_total", Help: "Guardian directory (/api/guardians) HTTP responses by actual status class and stable code."},
		[]string{"status_class", "code"},
	)
	schoolStructureOperations = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "phoenix_school_structure_operations_total", Help: "School Structure operations by operation, outcome, and stable error code."},
		[]string{"operation", "outcome", "code"},
	)
	schoolStructureDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "phoenix_school_structure_operation_duration_seconds", Help: "School Structure operation duration by operation.", Buckets: []float64{0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25}},
		[]string{"operation"},
	)
	schoolStructureQueries = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "phoenix_school_structure_queries_total", Help: "Persistence queries issued by School Structure operations."},
		[]string{"operation"},
	)
	schoolStructureRows = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "phoenix_school_structure_rows_total", Help: "Rows returned or changed by School Structure operations."},
		[]string{"operation"},
	)
	schoolStructureStatementDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "phoenix_school_structure_statement_duration_seconds", Help: "Cumulative School Structure database-statement duration by operation, used as a lock-wait upper bound.", Buckets: []float64{0.0001, 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5}},
		[]string{"operation"},
	)
	facilitiesOperations = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "phoenix_facilities_operations_total", Help: "Facilities operations by operation, outcome, and stable error code."},
		[]string{"operation", "outcome", "code"},
	)
	facilitiesDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "phoenix_facilities_operation_duration_seconds", Help: "Facilities operation duration by operation.", Buckets: []float64{0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25}},
		[]string{"operation"},
	)
	facilitiesQueries = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "phoenix_facilities_queries_total", Help: "Persistence queries issued by Facilities operations."},
		[]string{"operation"},
	)
	facilitiesRows = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "phoenix_facilities_rows_total", Help: "Rows returned or changed by Facilities operations."},
		[]string{"operation"},
	)
	facilitiesStatementDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "phoenix_facilities_statement_duration_seconds", Help: "Cumulative Facilities database-statement duration by operation.", Buckets: []float64{0.0001, 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5}},
		[]string{"operation"},
	)
	timetableActivitiesOperations = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "phoenix_timetable_activities_operations_total", Help: "Timetable & Activities operations by operation, outcome, and stable error code."},
		[]string{"operation", "outcome", "code"},
	)
	timetableActivitiesDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "phoenix_timetable_activities_operation_duration_seconds", Help: "Timetable & Activities operation duration by operation.", Buckets: []float64{0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25}},
		[]string{"operation"},
	)
	timetableActivitiesQueries = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "phoenix_timetable_activities_queries_total", Help: "Persistence queries issued by Timetable & Activities operations."},
		[]string{"operation"},
	)
	timetableActivitiesRows = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "phoenix_timetable_activities_rows_total", Help: "Rows returned or changed by Timetable & Activities operations."},
		[]string{"operation"},
	)
	timetableActivitiesDuplicatePreventionConflicts = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "phoenix_timetable_activities_duplicate_prevention_conflicts_total", Help: "Timetable & Activities writes rejected by database uniqueness constraints."},
		[]string{"operation"},
	)
	timetableActivitiesStatementDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "phoenix_timetable_activities_statement_duration_seconds", Help: "Cumulative Timetable & Activities database-statement duration by operation, used as a lock-wait upper bound.", Buckets: []float64{0.0001, 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5}},
		[]string{"operation"},
	)
	schoolCalendarOperations = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "phoenix_school_calendar_operations_total", Help: "School Calendar operations by operation, outcome, and stable error code."},
		[]string{"operation", "outcome", "code"},
	)
	schoolCalendarDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "phoenix_school_calendar_operation_duration_seconds", Help: "School Calendar operation duration by operation.", Buckets: []float64{0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25}},
		[]string{"operation"},
	)
	schoolCalendarQueries = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "phoenix_school_calendar_queries_total", Help: "Persistence queries issued by School Calendar operations."},
		[]string{"operation"},
	)
	schoolCalendarRows = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "phoenix_school_calendar_rows_total", Help: "Rows returned or changed by School Calendar operations."},
		[]string{"operation"},
	)
	schoolCalendarStatementDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "phoenix_school_calendar_statement_duration_seconds", Help: "Cumulative School Calendar database-statement duration by operation, used as a lock-wait upper bound.", Buckets: []float64{0.0001, 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5}},
		[]string{"operation"},
	)
	appointmentsOperations = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "phoenix_appointments_operations_total", Help: "Appointments operations by operation, outcome, and stable error code."},
		[]string{"operation", "outcome", "code"},
	)
	appointmentsDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "phoenix_appointments_operation_duration_seconds", Help: "Appointments operation duration by operation.", Buckets: []float64{0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25}},
		[]string{"operation"},
	)
	appointmentsQueries = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "phoenix_appointments_queries_total", Help: "Persistence queries issued by Appointments operations."},
		[]string{"operation"},
	)
	appointmentsRows = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "phoenix_appointments_rows_total", Help: "Rows returned or changed by Appointments operations."},
		[]string{"operation"},
	)
	appointmentsDuplicatePreventionConflicts = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "phoenix_appointments_duplicate_prevention_conflicts_total", Help: "Idempotent Appointments writes resolved by a database uniqueness conflict."},
		[]string{"operation"},
	)
	appointmentsStatementDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "phoenix_appointments_statement_duration_seconds", Help: "Cumulative Appointments database-statement duration by operation, used as a lock-wait upper bound.", Buckets: []float64{0.0001, 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5}},
		[]string{"operation"},
	)
	communicationOperations = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "phoenix_communication_operations_total", Help: "Communication operations by operation, outcome, and stable error code."},
		[]string{"operation", "outcome", "code"},
	)
	communicationDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "phoenix_communication_operation_duration_seconds", Help: "Communication operation duration by operation.", Buckets: []float64{0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25}},
		[]string{"operation"},
	)
	communicationQueries = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "phoenix_communication_queries_total", Help: "Persistence queries issued by Communication operations."},
		[]string{"operation"},
	)
	communicationRows = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "phoenix_communication_rows_total", Help: "Rows returned or changed by Communication operations."},
		[]string{"operation"},
	)
	communicationDuplicatePreventionConflicts = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "phoenix_communication_duplicate_prevention_conflicts_total", Help: "Idempotent Communication writes resolved by a database uniqueness conflict."},
		[]string{"operation"},
	)
	communicationStatementDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "phoenix_communication_statement_duration_seconds", Help: "Cumulative Communication database-statement duration by operation, used as a lock-wait upper bound.", Buckets: []float64{0.0001, 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5}},
		[]string{"operation"},
	)
	carePlanOperations = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "phoenix_care_plan_operations_total", Help: "Care Plan operations by operation, outcome, and stable error code."},
		[]string{"operation", "outcome", "code"},
	)
	carePlanDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "phoenix_care_plan_operation_duration_seconds", Help: "Care Plan operation duration by operation.", Buckets: []float64{0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25}},
		[]string{"operation"},
	)
	carePlanQueries = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "phoenix_care_plan_queries_total", Help: "Persistence queries issued by Care Plan operations."},
		[]string{"operation"},
	)
	carePlanRows = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "phoenix_care_plan_rows_total", Help: "Rows returned or changed by Care Plan operations."},
		[]string{"operation"},
	)
	carePlanDuplicateConflicts = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "phoenix_care_plan_duplicate_conflicts_total", Help: "Duplicate writes prevented by Care Plan operations."},
		[]string{"operation"},
	)
	carePlanStatementDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "phoenix_care_plan_statement_duration_seconds", Help: "Cumulative Care Plan database-statement duration by operation, used as a lock-wait upper bound.", Buckets: []float64{0.0001, 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5}},
		[]string{"operation"},
	)
	schoolMembershipOperations = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "phoenix_school_membership_operations_total", Help: "School Membership operations by operation, outcome, and stable error code."},
		[]string{"operation", "outcome", "code"},
	)
	schoolMembershipDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "phoenix_school_membership_operation_duration_seconds", Help: "School Membership operation duration by operation.", Buckets: []float64{0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25}},
		[]string{"operation"},
	)
	schoolMembershipQueries = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "phoenix_school_membership_queries_total", Help: "Persistence queries issued by School Membership operations."},
		[]string{"operation"},
	)
	schoolMembershipRows = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "phoenix_school_membership_rows_total", Help: "Rows returned or changed by School Membership operations."},
		[]string{"operation"},
	)
	schoolMembershipStatementDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "phoenix_school_membership_statement_duration_seconds", Help: "Cumulative School Membership database-statement duration by operation, used as a lock-wait upper bound.", Buckets: []float64{0.0001, 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5}},
		[]string{"operation"},
	)
	schoolMembershipHTTPResponses = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "phoenix_school_membership_http_responses_total", Help: "School Membership HTTP responses under /api/staff by actual status class and stable code."},
		[]string{"status_class", "code"},
	)
	feedbackOperations = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "phoenix_feedback_operations_total", Help: "Feedback operations by operation, outcome, and stable error code."},
		[]string{"operation", "outcome", "code"},
	)
	feedbackHTTPResponses = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "phoenix_feedback_http_responses_total", Help: "Feedback HTTP responses by surface, actual status class, and stable code."},
		[]string{"surface", "status_class", "code"},
	)
	feedbackDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{Name: "phoenix_feedback_operation_duration_seconds", Help: "Feedback operation duration by operation.", Buckets: []float64{0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25}},
		[]string{"operation"},
	)
	feedbackQueries = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "phoenix_feedback_queries_total", Help: "Persistence queries issued by Feedback operations."},
		[]string{"operation"},
	)
	feedbackRowsChanged = prometheus.NewCounterVec(
		prometheus.CounterOpts{Name: "phoenix_feedback_rows_changed_total", Help: "Rows changed by Feedback operations, including retention."},
		[]string{"operation"},
	)
	feedbackStatementDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "phoenix_feedback_statement_duration_seconds",
			Help:    "Cumulative Feedback write-statement duration by operation, used as a lock-wait upper bound.",
			Buckets: []float64{0.0001, 0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5},
		},
		[]string{"operation"},
	)
	auditAppends = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "phoenix_audit_appends_total",
			Help: "Audit append attempts by stable event type and outcome.",
		},
		[]string{"event_type", "outcome"},
	)
	auditAppendDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "phoenix_audit_append_duration_seconds",
			Help:    "Audit append duration by stable event type.",
			Buckets: []float64{0.0001, 0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1},
		},
		[]string{"event_type"},
	)
	auditRows = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "phoenix_audit_rows_total",
			Help: "Rows appended to Audit ledgers by stable event type.",
		},
		[]string{"event_type"},
	)
	synchronousDeliveries = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "phoenix_synchronous_deliveries_total",
			Help: "Fail-closed delivery calls by transport, template, caller, and outcome.",
		},
		[]string{"transport", "template", "caller", "outcome"},
	)
	synchronousDeliveryDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "phoenix_synchronous_delivery_duration_seconds",
			Help:    "Fail-closed delivery duration by transport, template, and caller.",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 20, 45, 60},
		},
		[]string{"transport", "template", "caller"},
	)
	durableDeliveryOperations = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "phoenix_delivery_operations_total",
			Help: "Durable Delivery intents processed by transport, template, operation, and outcome.",
		},
		[]string{"transport", "template", "operation", "outcome"},
	)
	durableDeliveryDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "phoenix_delivery_operation_duration_seconds",
			Help:    "Durable Delivery operation duration, including provider and status-query latency.",
			Buckets: []float64{0.001, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 20, 45, 60},
		},
		[]string{"transport", "template", "operation"},
	)
	durableDeliveryOldestPendingAge = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "phoenix_delivery_oldest_pending_age_seconds",
			Help: "Age in seconds of the oldest pending or claimed Delivery intent.",
		},
	)
	rateLimitRejections = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "phoenix_rate_limit_rejections_total",
			Help: "Requests rejected by the API rate limiter, split by quota bucket.",
		},
		[]string{"bucket"},
	)
	authorizationDenials = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "phoenix_authorization_denials_total",
			Help: "Authorization denials by stable reason code.",
		},
		[]string{"reason"},
	)
	authMiddlewareDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "phoenix_auth_middleware_duration_seconds",
			Help:    "Security-principal middleware duration by outcome.",
			Buckets: []float64{0.0001, 0.00025, 0.0005, 0.001, 0.0025, 0.005, 0.01, 0.025, 0.05},
		},
		[]string{"outcome"},
	)
	iotRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "phoenix_iot_requests_total",
			Help: "IoT requests by tenant, route, device type, and outcome.",
		},
		[]string{"tenant_id", "method", "route", "status_class", "device_type", "outcome"},
	)
	iotDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "phoenix_iot_request_duration_seconds",
			Help:    "IoT request duration by tenant, route, and device type.",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
		},
		[]string{"tenant_id", "method", "route", "device_type"},
	)
	sseBroadcasts = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "phoenix_sse_broadcasts_total",
			Help: "SSE broadcasts by tenant, event type, and target.",
		},
		[]string{"tenant_id", "event_type", "target"},
	)
	sseDropped = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "phoenix_sse_dropped_events_total",
			Help: "SSE events dropped because a client channel was full.",
		},
		[]string{"tenant_id", "event_type", "target"},
	)
	sseConnections = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "phoenix_sse_connections_total",
			Help: "SSE connection lifecycle events.",
		},
		[]string{"tenant_id", "event"},
	)
	sseClients = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "phoenix_sse_clients",
			Help: "Currently connected SSE clients by tenant.",
		},
		[]string{"tenant_id"},
	)
	pwaStandaloneUsers = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "phoenix_pwa_standalone_users",
			Help: "Accounts that used the app in PWA standalone mode within the last 30 days, by tenant and portal.",
		},
		[]string{"tenant_id", "portal"},
	)
	pwaEligibleUsers = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "phoenix_pwa_eligible_users",
			Help: "Accounts with an active mapping matching the portal's role predicate, by tenant and portal.",
		},
		[]string{"tenant_id", "portal"},
	)

	dbStatsMu        sync.RWMutex
	dbStatsProvider  func() DBStats
	sseStatsMu       sync.RWMutex
	sseStatsProvider SSEStatsProvider
	sseGaugeMu       sync.Mutex
	sseGaugeTenants  = make(map[string]struct{})

	pwaStatsMu       sync.RWMutex
	pwaStatsProvider PWAUsageStatsProvider
	pwaGaugeMu       sync.Mutex
	pwaGaugeLabels   = make(map[[2]string]struct{})
	workerJobMaxMu   sync.Mutex
	workerJobMax     = make(map[string]time.Duration)

	dbOpenConnectionsDesc      = prometheus.NewDesc("phoenix_db_open_connections", "Open DB connections.", nil, nil)
	dbInUseConnectionsDesc     = prometheus.NewDesc("phoenix_db_in_use_connections", "DB connections currently in use.", nil, nil)
	dbIdleConnectionsDesc      = prometheus.NewDesc("phoenix_db_idle_connections", "Idle DB connections.", nil, nil)
	dbWaitCountDesc            = prometheus.NewDesc("phoenix_db_wait_count_total", "Total waits for a DB connection.", nil, nil)
	dbWaitDurationDesc         = prometheus.NewDesc("phoenix_db_wait_duration_seconds_total", "Total wait time for DB connections.", nil, nil)
	dbMaxIdleClosedDesc        = prometheus.NewDesc("phoenix_db_max_idle_closed_total", "DB connections closed due to max idle.", nil, nil)
	dbMaxLifetimeClosedDesc    = prometheus.NewDesc("phoenix_db_max_lifetime_closed_total", "DB connections closed due to max lifetime.", nil, nil)
	errMetricsBearerTokenEmpty = errors.New("METRICS_BEARER_TOKEN is required")
)

func init() {
	prometheus.MustRegister(
		appHTTPRequests,
		appHTTPDuration,
		appHTTPActive,
		tenantHTTPRequests,
		tenantHTTPDuration,
		tenantRuntimeEvents,
		unitOfWorkDuration,
		unitOfWorkRollbacks,
		unitOfWorkRetries,
		unitOfWorkPoolWait,
		unitOfWorkLockWait,
		workerJobDuration,
		workerJobMaxDuration,
		workerTenantBatchDuration,
		workerTenantBatchTenants,
		workerTenantBatchRetries,
		workerTenantBatchBacklog,
		workerTenantBatchPoolWait,
		settingsLookups,
		settingsLookupDuration,
		settingsSideEffectFailures,
		mealPlanOperations,
		mealPlanDuration,
		mealPlanQueries,
		mealPlanRowsChanged,
		mealPlanStatementDuration,
		organizationTenancyOperations,
		organizationTenancyDuration,
		organizationTenancyQueries,
		organizationTenancyRowsChanged,
		organizationTenancyStatementDuration,
		peopleDirectoryOperations,
		peopleDirectoryDuration,
		peopleDirectoryQueries,
		peopleDirectoryRowsChanged,
		peopleDirectoryStatementDuration,
		peopleDirectoryHTTPResponses,
		guardianDirectoryHTTPResponses,
		schoolStructureOperations,
		schoolStructureDuration,
		schoolStructureQueries,
		schoolStructureRows,
		schoolStructureStatementDuration,
		facilitiesOperations,
		facilitiesDuration,
		facilitiesQueries,
		facilitiesRows,
		facilitiesStatementDuration,
		timetableActivitiesOperations,
		timetableActivitiesDuration,
		timetableActivitiesQueries,
		timetableActivitiesRows,
		timetableActivitiesDuplicatePreventionConflicts,
		timetableActivitiesStatementDuration,
		schoolCalendarOperations,
		schoolCalendarDuration,
		schoolCalendarQueries,
		schoolCalendarRows,
		schoolCalendarStatementDuration,
		appointmentsOperations,
		appointmentsDuration,
		appointmentsQueries,
		appointmentsRows,
		appointmentsDuplicatePreventionConflicts,
		appointmentsStatementDuration,
		communicationOperations,
		communicationDuration,
		communicationQueries,
		communicationRows,
		communicationDuplicatePreventionConflicts,
		communicationStatementDuration,
		carePlanOperations,
		carePlanDuration,
		carePlanQueries,
		carePlanRows,
		carePlanDuplicateConflicts,
		carePlanStatementDuration,
		schoolMembershipOperations,
		schoolMembershipDuration,
		schoolMembershipQueries,
		schoolMembershipRows,
		schoolMembershipStatementDuration,
		schoolMembershipHTTPResponses,
		feedbackOperations,
		feedbackHTTPResponses,
		feedbackDuration,
		feedbackQueries,
		feedbackRowsChanged,
		feedbackStatementDuration,
		auditAppends,
		auditAppendDuration,
		auditRows,
		synchronousDeliveries,
		synchronousDeliveryDuration,
		durableDeliveryOperations,
		durableDeliveryDuration,
		durableDeliveryOldestPendingAge,
		rateLimitRejections,
		authorizationDenials,
		authMiddlewareDuration,
		iotRequests,
		iotDuration,
		sseBroadcasts,
		sseDropped,
		sseConnections,
		sseClients,
		pwaStandaloneUsers,
		pwaEligibleUsers,
		dbStatsCollector{},
	)
}

func MetricsBearerTokenFromEnv(getenv func(string) string) (string, error) {
	token := strings.TrimSpace(getenv("METRICS_BEARER_TOKEN"))
	if token == "" {
		return "", errMetricsBearerTokenEmpty
	}
	return token, nil
}

func RegisterDBStatsProvider(provider func() DBStats) {
	dbStatsMu.Lock()
	defer dbStatsMu.Unlock()
	dbStatsProvider = provider
}

func RegisterSSEStatsProvider(provider SSEStatsProvider) {
	sseStatsMu.Lock()
	defer sseStatsMu.Unlock()
	sseStatsProvider = provider
}

// RegisterPWAUsageStatsProvider wires the PWA standalone-usage source
// (#2189). The provider runs on every scrape and must cache internally.
func RegisterPWAUsageStatsProvider(provider PWAUsageStatsProvider) {
	pwaStatsMu.Lock()
	defer pwaStatsMu.Unlock()
	pwaStatsProvider = provider
}

// RefreshGauges updates scrape-time gauges before the HTTP adapter serves the
// Prometheus registry.
func RefreshGauges() {
	refreshSSEGauges()
	refreshPWAGauges()
}

func IncActiveHTTPRequests() {
	appHTTPActive.Inc()
}

func DecActiveHTTPRequests() {
	appHTTPActive.Dec()
}

func ObserveHTTPRequest(method, route string, status int, duration time.Duration) {
	method = normalizeHTTPMethod(method)
	statusClass := StatusClass(status)
	appHTTPRequests.WithLabelValues(method, route, statusClass).Inc()
	appHTTPDuration.WithLabelValues(method, route).Observe(duration.Seconds())
}

func ObserveMealPlanOperation(operation string, duration time.Duration, queries, rows int64, statementDuration time.Duration, err error) {
	outcome := "success"
	if err != nil {
		outcome = "error"
	}
	mealPlanOperations.WithLabelValues(operation, outcome).Inc()
	mealPlanDuration.WithLabelValues(operation).Observe(duration.Seconds())
	if queries > 0 {
		mealPlanQueries.WithLabelValues(operation).Add(float64(queries))
	}
	if rows > 0 {
		mealPlanRowsChanged.WithLabelValues(operation).Add(float64(rows))
	}
	if statementDuration > 0 {
		mealPlanStatementDuration.WithLabelValues(operation).Observe(statementDuration.Seconds())
	}
}

func ObserveFeedbackOperation(operation string, duration time.Duration, queries, rows int64, statementDuration time.Duration, code string, err error) {
	outcome := "success"
	if err == nil {
		code = "none"
	} else {
		outcome = "error"
	}
	feedbackOperations.WithLabelValues(sanitizeLabel(operation), outcome, sanitizeLabel(code)).Inc()
	feedbackDuration.WithLabelValues(sanitizeLabel(operation)).Observe(duration.Seconds())
	if queries > 0 {
		feedbackQueries.WithLabelValues(sanitizeLabel(operation)).Add(float64(queries))
	}
	if rows > 0 {
		feedbackRowsChanged.WithLabelValues(sanitizeLabel(operation)).Add(float64(rows))
	}
	if statementDuration > 0 {
		feedbackStatementDuration.WithLabelValues(sanitizeLabel(operation)).Observe(statementDuration.Seconds())
	}
}

func ObserveOrganizationTenancyOperation(operation string, duration time.Duration, queries, rows int64, statementDuration time.Duration, code string, err error) {
	outcome := "success"
	if err == nil {
		code = "none"
	} else {
		outcome = "error"
	}
	operation = sanitizeLabel(operation)
	organizationTenancyOperations.WithLabelValues(operation, outcome, sanitizeLabel(code)).Inc()
	organizationTenancyDuration.WithLabelValues(operation).Observe(duration.Seconds())
	if queries > 0 {
		organizationTenancyQueries.WithLabelValues(operation).Add(float64(queries))
	}
	if rows > 0 {
		organizationTenancyRowsChanged.WithLabelValues(operation).Add(float64(rows))
	}
	if statementDuration > 0 {
		organizationTenancyStatementDuration.WithLabelValues(operation).Observe(statementDuration.Seconds())
	}
}

// ObservePeopleDirectoryOperation records the runtime evidence of one People
// Directory capability call: outcome and stable code, duration, query count,
// changed rows, and cumulative statement duration.
func ObservePeopleDirectoryOperation(operation string, duration time.Duration, queries, rows int64, statementDuration time.Duration, code string, err error) {
	outcome := "success"
	if err == nil {
		code = "none"
	} else {
		outcome = "error"
	}
	operation = sanitizeLabel(operation)
	peopleDirectoryOperations.WithLabelValues(operation, outcome, sanitizeLabel(code)).Inc()
	peopleDirectoryDuration.WithLabelValues(operation).Observe(duration.Seconds())
	if queries > 0 {
		peopleDirectoryQueries.WithLabelValues(operation).Add(float64(queries))
	}
	if rows > 0 {
		peopleDirectoryRowsChanged.WithLabelValues(operation).Add(float64(rows))
	}
	if statementDuration > 0 {
		peopleDirectoryStatementDuration.WithLabelValues(operation).Observe(statementDuration.Seconds())
	}
}

// ObservePeopleDirectoryHTTPResponse counts one /api/users response by the
// status class actually written and the stable outcome code.
func ObservePeopleDirectoryHTTPResponse(status int, code string) {
	statusClass := strconv.Itoa(status/100) + "xx"
	peopleDirectoryHTTPResponses.WithLabelValues(statusClass, sanitizeLabel(code)).Inc()
}

// ObserveGuardianDirectoryHTTPResponse counts one /api/guardians response by
// the status class actually written and the stable outcome code (#2663).
func ObserveGuardianDirectoryHTTPResponse(status int, code string) {
	statusClass := strconv.Itoa(status/100) + "xx"
	guardianDirectoryHTTPResponses.WithLabelValues(statusClass, sanitizeLabel(code)).Inc()
}

// ObserveSchoolMembershipOperation records the runtime evidence of one School
// Membership capability call: outcome and stable code, duration, query
// count, rows, and cumulative statement duration.
func ObserveSchoolMembershipOperation(operation string, duration time.Duration, queries, rows int64, statementDuration time.Duration, code string, err error) {
	outcome := "success"
	if err == nil {
		code = "none"
	} else {
		outcome = "error"
	}
	operation = sanitizeLabel(operation)
	schoolMembershipOperations.WithLabelValues(operation, outcome, sanitizeLabel(code)).Inc()
	schoolMembershipDuration.WithLabelValues(operation).Observe(duration.Seconds())
	if queries > 0 {
		schoolMembershipQueries.WithLabelValues(operation).Add(float64(queries))
	}
	if rows > 0 {
		schoolMembershipRows.WithLabelValues(operation).Add(float64(rows))
	}
	if statementDuration > 0 {
		schoolMembershipStatementDuration.WithLabelValues(operation).Observe(statementDuration.Seconds())
	}
}

// ObserveSchoolMembershipHTTPResponse counts one School Membership HTTP
// adapter response (/api/staff membership routes, /api/class-list-entries)
// by the status class actually written and the stable outcome code.
func ObserveSchoolMembershipHTTPResponse(status int, code string) {
	statusClass := strconv.Itoa(status/100) + "xx"
	schoolMembershipHTTPResponses.WithLabelValues(statusClass, sanitizeLabel(code)).Inc()
}

func ObserveSchoolStructureOperation(operation string, duration time.Duration, queries, rows int64, statementDuration time.Duration, code string, err error) {
	outcome := "success"
	if err == nil {
		code = "none"
	} else {
		outcome = "error"
	}
	operation = sanitizeLabel(operation)
	schoolStructureOperations.WithLabelValues(operation, outcome, sanitizeLabel(code)).Inc()
	schoolStructureDuration.WithLabelValues(operation).Observe(duration.Seconds())
	if queries > 0 {
		schoolStructureQueries.WithLabelValues(operation).Add(float64(queries))
	}
	if rows > 0 {
		schoolStructureRows.WithLabelValues(operation).Add(float64(rows))
	}
	if statementDuration > 0 {
		schoolStructureStatementDuration.WithLabelValues(operation).Observe(statementDuration.Seconds())
	}
}

func ObserveFacilitiesOperation(operation string, duration time.Duration, queries, rows int64, statementDuration time.Duration, code string, err error) {
	outcome := "success"
	if err == nil {
		code = "none"
	} else {
		outcome = "error"
	}
	operation = sanitizeLabel(operation)
	facilitiesOperations.WithLabelValues(operation, outcome, sanitizeLabel(code)).Inc()
	facilitiesDuration.WithLabelValues(operation).Observe(duration.Seconds())
	if queries > 0 {
		facilitiesQueries.WithLabelValues(operation).Add(float64(queries))
	}
	if rows > 0 {
		facilitiesRows.WithLabelValues(operation).Add(float64(rows))
	}
	if statementDuration > 0 {
		facilitiesStatementDuration.WithLabelValues(operation).Observe(statementDuration.Seconds())
	}
}

func ObserveTimetableActivitiesOperation(operation string, duration time.Duration, queries, rows, duplicatePreventionConflicts int64, statementDuration time.Duration, code string, err error) {
	outcome := "success"
	if err == nil {
		code = "none"
	} else {
		outcome = "error"
	}
	operation = sanitizeLabel(operation)
	timetableActivitiesOperations.WithLabelValues(operation, outcome, sanitizeLabel(code)).Inc()
	timetableActivitiesDuration.WithLabelValues(operation).Observe(duration.Seconds())
	if queries > 0 {
		timetableActivitiesQueries.WithLabelValues(operation).Add(float64(queries))
	}
	if rows > 0 {
		timetableActivitiesRows.WithLabelValues(operation).Add(float64(rows))
	}
	if duplicatePreventionConflicts > 0 {
		timetableActivitiesDuplicatePreventionConflicts.WithLabelValues(operation).Add(float64(duplicatePreventionConflicts))
	}
	if statementDuration > 0 {
		timetableActivitiesStatementDuration.WithLabelValues(operation).Observe(statementDuration.Seconds())
	}
}

func ObserveSchoolCalendarOperation(operation string, duration time.Duration, queries, rows int64, statementDuration time.Duration, code string, err error) {
	outcome := "success"
	if err == nil {
		code = "none"
	} else {
		outcome = "error"
	}
	operation = sanitizeLabel(operation)
	schoolCalendarOperations.WithLabelValues(operation, outcome, sanitizeLabel(code)).Inc()
	schoolCalendarDuration.WithLabelValues(operation).Observe(duration.Seconds())
	if queries > 0 {
		schoolCalendarQueries.WithLabelValues(operation).Add(float64(queries))
	}
	if rows > 0 {
		schoolCalendarRows.WithLabelValues(operation).Add(float64(rows))
	}
	if statementDuration > 0 {
		schoolCalendarStatementDuration.WithLabelValues(operation).Observe(statementDuration.Seconds())
	}
}

func ObserveAppointmentsOperation(operation string, duration time.Duration, queries, rows, duplicatePreventionConflicts int64, statementDuration time.Duration, code string, err error) {
	outcome := "success"
	if err == nil {
		code = "none"
	} else {
		outcome = "error"
	}
	operation = sanitizeLabel(operation)
	appointmentsOperations.WithLabelValues(operation, outcome, sanitizeLabel(code)).Inc()
	appointmentsDuration.WithLabelValues(operation).Observe(duration.Seconds())
	if queries > 0 {
		appointmentsQueries.WithLabelValues(operation).Add(float64(queries))
	}
	if rows > 0 {
		appointmentsRows.WithLabelValues(operation).Add(float64(rows))
	}
	if duplicatePreventionConflicts > 0 {
		appointmentsDuplicatePreventionConflicts.WithLabelValues(operation).Add(float64(duplicatePreventionConflicts))
	}
	if statementDuration > 0 {
		appointmentsStatementDuration.WithLabelValues(operation).Observe(statementDuration.Seconds())
	}
}

func ObserveCommunicationOperation(operation string, duration time.Duration, queries, rows, duplicatePreventionConflicts int64, statementDuration time.Duration, code string, err error) {
	outcome := "success"
	if err == nil {
		code = "none"
	} else {
		outcome = "error"
	}
	operation = sanitizeLabel(operation)
	communicationOperations.WithLabelValues(operation, outcome, sanitizeLabel(code)).Inc()
	communicationDuration.WithLabelValues(operation).Observe(duration.Seconds())
	if queries > 0 {
		communicationQueries.WithLabelValues(operation).Add(float64(queries))
	}
	if rows > 0 {
		communicationRows.WithLabelValues(operation).Add(float64(rows))
	}
	if duplicatePreventionConflicts > 0 {
		communicationDuplicatePreventionConflicts.WithLabelValues(operation).Add(float64(duplicatePreventionConflicts))
	}
	if statementDuration > 0 {
		communicationStatementDuration.WithLabelValues(operation).Observe(statementDuration.Seconds())
	}
}

func ObserveCarePlanOperation(operation string, duration time.Duration, queries, rows, conflicts int64, statementDuration time.Duration, code string, err error) {
	outcome := "success"
	if err == nil {
		code = "none"
	} else {
		outcome = "error"
	}
	operation = sanitizeLabel(operation)
	carePlanOperations.WithLabelValues(operation, outcome, sanitizeLabel(code)).Inc()
	carePlanDuration.WithLabelValues(operation).Observe(duration.Seconds())
	if queries > 0 {
		carePlanQueries.WithLabelValues(operation).Add(float64(queries))
	}
	if rows > 0 {
		carePlanRows.WithLabelValues(operation).Add(float64(rows))
	}
	if conflicts > 0 {
		carePlanDuplicateConflicts.WithLabelValues(operation).Add(float64(conflicts))
	}
	if statementDuration > 0 {
		carePlanStatementDuration.WithLabelValues(operation).Observe(statementDuration.Seconds())
	}
}

func ObserveFeedbackHTTPResponse(surface string, status int, code string) {
	statusClass := strconv.Itoa(status/100) + "xx"
	feedbackHTTPResponses.WithLabelValues(sanitizeLabel(surface), statusClass, sanitizeLabel(code)).Inc()
}

func ObserveAuditAppend(eventType string, duration time.Duration, rows int, err error) {
	eventType = sanitizeLabel(eventType)
	outcome := "success"
	if err != nil {
		outcome = "error"
	}
	auditAppends.WithLabelValues(eventType, outcome).Inc()
	auditAppendDuration.WithLabelValues(eventType).Observe(duration.Seconds())
	if rows > 0 {
		auditRows.WithLabelValues(eventType).Add(float64(rows))
	}
}

// ObserveSynchronousDelivery records fail-closed sends without recipient or
// payload labels. The outcome separates timeouts/cancellation from transport
// failures so operators can alert on each class independently.
func ObserveSynchronousDelivery(transport, template, caller string, duration time.Duration, err error) {
	outcome := "success"
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		outcome = "timeout"
	case errors.Is(err, context.Canceled):
		outcome = "canceled"
	case err != nil:
		outcome = "failure"
	}
	transport = sanitizeLabel(transport)
	template = sanitizeLabel(template)
	caller = sanitizeLabel(caller)
	synchronousDeliveries.WithLabelValues(transport, template, caller, outcome).Inc()
	synchronousDeliveryDuration.WithLabelValues(transport, template, caller).Observe(duration.Seconds())
}

func ObserveDurableDelivery(transport, template, operation string, duration time.Duration, count int, err error) {
	if operation == "oldest_pending_age" {
		if err == nil {
			durableDeliveryOldestPendingAge.Set(duration.Seconds())
		}
		return
	}
	outcome := "success"
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		outcome = "timeout"
	case errors.Is(err, context.Canceled):
		outcome = "canceled"
	case err != nil:
		outcome = "failure"
	}
	transport = sanitizeLabel(transport)
	template = sanitizeLabel(template)
	operation = sanitizeLabel(operation)
	amount := count
	if amount <= 0 {
		amount = 1
	}
	durableDeliveryOperations.WithLabelValues(transport, template, operation, outcome).Add(float64(amount))
	durableDeliveryDuration.WithLabelValues(transport, template, operation).Observe(duration.Seconds())
}

func ObserveTenantRequest(tenantID int64, scope, method, route string, status int, duration time.Duration, txOutcome string) {
	method = normalizeHTTPMethod(method)
	tenant := strconv.FormatInt(tenantID, 10)
	statusClass := StatusClass(status)
	tenantHTTPRequests.WithLabelValues(tenant, scope, method, route, statusClass, txOutcome).Inc()
	tenantHTTPDuration.WithLabelValues(tenant, scope, method, route).Observe(duration.Seconds())
}

func normalizeHTTPMethod(method string) string {
	switch strings.ToUpper(method) {
	case "CONNECT", "DELETE", "GET", "HEAD", "OPTIONS", "PATCH", "POST", "PUT", "TRACE":
		return strings.ToUpper(method)
	default:
		return "other"
	}
}

func RecordTenantRuntimeEvent(entryPoint, outcome string) {
	tenantRuntimeEvents.WithLabelValues(sanitizeLabel(entryPoint), sanitizeLabel(outcome)).Inc()
}

func RecordUnitOfWorkEvent(entryPoint, kind, result string, duration time.Duration, retries int) {
	entryPoint = sanitizeLabel(entryPoint)
	switch kind {
	case "transaction":
		result = sanitizeLabel(result)
		unitOfWorkDuration.WithLabelValues(entryPoint, result).Observe(duration.Seconds())
		if result == "rollback" || result == "panic" {
			unitOfWorkRollbacks.WithLabelValues(entryPoint).Inc()
		}
		if retries > 0 {
			unitOfWorkRetries.WithLabelValues(entryPoint).Add(float64(retries))
		}
	case "pool_wait":
		unitOfWorkPoolWait.WithLabelValues(entryPoint).Observe(duration.Seconds())
	case "lock_wait":
		unitOfWorkLockWait.WithLabelValues(entryPoint).Observe(duration.Seconds())
	}
}

// RecordWorkerRunEvent records one bounded embedded-job outcome.
func RecordWorkerRunEvent(jobID, outcome string, duration time.Duration) {
	jobID = sanitizeLabel(jobID)
	workerJobDuration.WithLabelValues(jobID, sanitizeLabel(outcome)).Observe(duration.Seconds())
	workerJobMaxMu.Lock()
	if duration > workerJobMax[jobID] {
		workerJobMax[jobID] = duration
		workerJobMaxDuration.WithLabelValues(jobID).Set(duration.Seconds())
	}
	workerJobMaxMu.Unlock()
}

// RecordWorkerTenantBatchEvent records one bounded group of isolated tenant
// commands. Labels contain only the registered job ID and fixed outcomes.
func RecordWorkerTenantBatchEvent(jobID string, duration time.Duration, processed, failed, retries, backlog int, poolWait time.Duration) {
	jobID = sanitizeLabel(jobID)
	workerTenantBatchDuration.WithLabelValues(jobID).Observe(duration.Seconds())
	workerTenantBatchTenants.WithLabelValues(jobID, "success").Add(float64(processed - failed))
	workerTenantBatchTenants.WithLabelValues(jobID, "failure").Add(float64(failed))
	workerTenantBatchRetries.WithLabelValues(jobID).Add(float64(retries))
	workerTenantBatchBacklog.WithLabelValues(jobID).Set(float64(backlog))
	workerTenantBatchPoolWait.WithLabelValues(jobID).Observe(poolWait.Seconds())
}

// SetWorkerTenantBatchBacklog records backlog when a job completes no tenant
// batch, so an earlier non-zero value does not remain visible indefinitely.
func SetWorkerTenantBatchBacklog(jobID string, backlog int) {
	workerTenantBatchBacklog.WithLabelValues(sanitizeLabel(jobID)).Set(float64(backlog))
}

func ObserveSettingsLookup(key, cache, outcome string, duration time.Duration) {
	settingsLookups.WithLabelValues(sanitizeLabel(key), sanitizeLabel(cache), sanitizeLabel(outcome)).Inc()
	settingsLookupDuration.WithLabelValues(sanitizeLabel(key)).Observe(duration.Seconds())
}

func RecordSettingsSideEffectFailure(key string) {
	settingsSideEffectFailures.WithLabelValues(sanitizeLabel(key)).Inc()
}

func RecordRateLimitRejection(bucket string) {
	rateLimitRejections.WithLabelValues(sanitizeLabel(bucket)).Inc()
}

func RecordAuthorizationEvent(outcome, reason string, duration time.Duration) {
	switch reason {
	case "invalid_principal", "missing_principal", "permission_denied":
		authorizationDenials.WithLabelValues(reason).Inc()
	case "":
	default:
		authorizationDenials.WithLabelValues("unknown").Inc()
	}
	if outcome == "resolved" || outcome == "invalid" {
		authMiddlewareDuration.WithLabelValues(outcome).Observe(duration.Seconds())
	}
}

func ObserveIoTRequest(tenantID int64, method, route string, status int, duration time.Duration, deviceType string) {
	method = normalizeHTTPMethod(method)
	tenant := strconv.FormatInt(tenantID, 10)
	if tenantID <= 0 {
		tenant = "unknown"
	}
	if deviceType == "" {
		deviceType = "unknown"
	}
	statusClass := StatusClass(status)
	iotRequests.WithLabelValues(tenant, method, route, statusClass, sanitizeLabel(deviceType), outcomeForStatus(status)).Inc()
	iotDuration.WithLabelValues(tenant, method, route, sanitizeLabel(deviceType)).Observe(duration.Seconds())
}

func RecordSSEConnection(tenantID int64, event string) {
	sseConnections.WithLabelValues(strconv.FormatInt(tenantID, 10), event).Inc()
}

func RecordSSEBroadcast(tenantID int64, eventType, target string, dropped int) {
	tenant := strconv.FormatInt(tenantID, 10)
	if tenantID == 0 {
		tenant = "all"
	}
	sseBroadcasts.WithLabelValues(tenant, sanitizeLabel(eventType), sanitizeLabel(target)).Inc()
	if dropped > 0 {
		sseDropped.WithLabelValues(tenant, sanitizeLabel(eventType), sanitizeLabel(target)).Add(float64(dropped))
	}
}

func StatusClass(status int) string {
	if status <= 0 {
		return "unknown"
	}
	return strconv.Itoa(status/100) + "xx"
}

func sanitizeLabel(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	if len(value) > 64 {
		return value[:64]
	}
	return value
}

func outcomeForStatus(status int) string {
	switch {
	case status >= 500:
		return "server_error"
	case status == 401 || status == 403:
		return "auth_error"
	case status >= 400:
		return "validation_error"
	case status >= 200 && status < 300:
		return "success"
	default:
		return "other"
	}
}

type dbStatsCollector struct{}

func (dbStatsCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- dbOpenConnectionsDesc
	ch <- dbInUseConnectionsDesc
	ch <- dbIdleConnectionsDesc
	ch <- dbWaitCountDesc
	ch <- dbWaitDurationDesc
	ch <- dbMaxIdleClosedDesc
	ch <- dbMaxLifetimeClosedDesc
}

func (dbStatsCollector) Collect(ch chan<- prometheus.Metric) {
	dbStatsMu.RLock()
	provider := dbStatsProvider
	dbStatsMu.RUnlock()
	if provider == nil {
		return
	}
	stats := provider()
	emitDBGauge(ch, dbOpenConnectionsDesc, float64(stats.OpenConnections))
	emitDBGauge(ch, dbInUseConnectionsDesc, float64(stats.InUse))
	emitDBGauge(ch, dbIdleConnectionsDesc, float64(stats.Idle))
	emitDBGauge(ch, dbWaitCountDesc, float64(stats.WaitCount))
	emitDBGauge(ch, dbWaitDurationDesc, stats.WaitDuration.Seconds())
	emitDBGauge(ch, dbMaxIdleClosedDesc, float64(stats.MaxIdleClosed))
	emitDBGauge(ch, dbMaxLifetimeClosedDesc, float64(stats.MaxLifetimeClosed))
}

func emitDBGauge(ch chan<- prometheus.Metric, desc *prometheus.Desc, value float64) {
	ch <- prometheus.MustNewConstMetric(desc, prometheus.GaugeValue, value)
}

func refreshSSEGauges() {
	sseStatsMu.RLock()
	provider := sseStatsProvider
	sseStatsMu.RUnlock()
	if provider == nil {
		return
	}
	stats := SSEStats{ClientsByTenant: provider.SnapshotSSEClientsByTenant()}
	currentTenants := make(map[string]struct{}, len(stats.ClientsByTenant))
	sseGaugeMu.Lock()
	defer sseGaugeMu.Unlock()
	for tenantID, count := range stats.ClientsByTenant {
		tenant := strconv.FormatInt(tenantID, 10)
		currentTenants[tenant] = struct{}{}
		sseClients.WithLabelValues(tenant).Set(float64(count))
	}

	for tenant := range sseGaugeTenants {
		if _, ok := currentTenants[tenant]; !ok {
			sseClients.WithLabelValues(tenant).Set(0)
		}
	}
	sseGaugeTenants = currentTenants
	if sseGaugeTenants == nil {
		sseGaugeTenants = make(map[string]struct{})
	}
}

// refreshPWAGauges pulls the standalone-usage counts on scrape, zeroing
// label pairs that disappeared. A provider error keeps the previous values
// — a failed refresh must not turn into a fake zero.
func refreshPWAGauges() {
	pwaStatsMu.RLock()
	provider := pwaStatsProvider
	pwaStatsMu.RUnlock()
	if provider == nil {
		return
	}
	stats, err := provider.SnapshotUsageStats()
	if err != nil {
		return
	}
	current := make(map[[2]string]struct{}, len(stats))
	pwaGaugeMu.Lock()
	defer pwaGaugeMu.Unlock()
	for _, stat := range stats {
		labels := [2]string{strconv.FormatInt(stat.TenantID, 10), stat.Portal}
		current[labels] = struct{}{}
		pwaStandaloneUsers.WithLabelValues(labels[0], labels[1]).Set(float64(stat.StandaloneUsers))
		pwaEligibleUsers.WithLabelValues(labels[0], labels[1]).Set(float64(stat.EligibleUsers))
	}
	for labels := range pwaGaugeLabels {
		if _, ok := current[labels]; !ok {
			pwaStandaloneUsers.WithLabelValues(labels[0], labels[1]).Set(0)
			pwaEligibleUsers.WithLabelValues(labels[0], labels[1]).Set(0)
		}
	}
	pwaGaugeLabels = current
}
