package architecture

// Frozen at migrationHistoryAnchor by ADR 0026. Entries may only be retired.
var historicalMetricGaps = map[string][]string{
	"account-sessions-2720.json":       {"lock_wait"},
	"device-scan-2698.json":            {"affected_rows", "deadlocks", "job_backlog", "job_duration", "job_retries", "latency_p50", "latency_p95", "lock_wait", "pool_wait"},
	"emergency-snapshot-2704.json":     {"affected_rows", "deadlocks", "job_backlog", "job_duration", "job_retries", "latency_p50", "latency_p95", "lock_wait", "pool_wait"},
	"identity-membership-2721.json":    {"lock_wait"},
	"operator-sessions-2720.json":      {"lock_wait"},
	"plan-export-2706.json":            {"affected_rows", "deadlocks", "job_backlog", "job_duration", "job_retries", "latency_p50", "latency_p95", "lock_wait", "pool_wait"},
	"request-review-2705.json":         {"affected_rows", "deadlocks", "job_backlog", "job_duration", "job_retries", "latency_p50", "latency_p95", "lock_wait", "pool_wait"},
	"session-end-2697.json":            {"affected_rows", "deadlocks", "job_backlog", "job_duration", "job_retries", "latency_p50", "latency_p95", "lock_wait", "pool_wait"},
	"staff-offboarding-2709.json":      {"affected_rows", "deadlocks", "job_backlog", "job_duration", "job_retries", "latency_p50", "latency_p95", "lock_wait", "pool_wait"},
	"staff-owner-backfill-2752.json":   {"deadlocks", "job_backlog", "job_retries", "latency_p50", "latency_p95", "lock_wait", "query_count"},
	"student-owner-backfill-2758.json": {"deadlocks", "job_backlog", "job_retries", "latency_p50", "latency_p95", "lock_wait", "query_count"},
	"student-owner-cutover-2759.json":  {"deadlocks", "job_backlog", "latency_p50", "latency_p95", "lock_wait", "query_count"},
}
