package scheduler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/moto-nrw/project-phoenix/tenant"
)

const tenantBatchSize = 25

// TenantCommand is the public command boundary used by tenant-scoped jobs.
// The scheduler owns cadence, batching, transactions, and observations; the
// command owns one tenant's business operation.
type TenantCommand interface {
	Execute(context.Context, tenant.TenantID) error
}

// TenantCommandFunc adapts an owner command function to TenantCommand.
type TenantCommandFunc func(context.Context, tenant.TenantID) error

func (command TenantCommandFunc) Execute(ctx context.Context, tenantID tenant.TenantID) error {
	return command(ctx, tenantID)
}

func adaptTenantCommand(command func(context.Context, int64) error) TenantCommand {
	return TenantCommandFunc(func(ctx context.Context, tenantID tenant.TenantID) error {
		return command(ctx, tenantID.Int64())
	})
}

// TenantOutcome classifies one isolated tenant command result.
type TenantOutcome struct {
	TenantID       int64
	Duration       time.Duration
	Retries        int
	Classification TenantOutcomeClassification
	Err            error
}

// TenantOutcomeClassification keeps retry and failure labels finite.
type TenantOutcomeClassification string

const (
	TenantOutcomeSuccess        TenantOutcomeClassification = "success"
	TenantOutcomeRetriedSuccess TenantOutcomeClassification = "retried_success"
	TenantOutcomeRetryExhausted TenantOutcomeClassification = "retry_exhausted"
	TenantOutcomeCommandFailure TenantOutcomeClassification = "command_failure"
	TenantOutcomeMissingTenant  TenantOutcomeClassification = "missing_tenant"
	TenantOutcomeCancelled      TenantOutcomeClassification = "cancelled"
)

// TenantBatchResult reports every attempted tenant without hiding failures.
type TenantBatchResult struct {
	Outcomes []TenantOutcome
	Batches  int
	Backlog  int
	Err      error
}

func (result TenantBatchResult) CompletedTenantIDs() []int64 {
	completed := make([]int64, 0, len(result.Outcomes))
	for _, outcome := range result.Outcomes {
		if outcome.Err == nil {
			completed = append(completed, outcome.TenantID)
		}
	}
	return completed
}

func (result TenantBatchResult) Failed() int {
	failed := 0
	for _, outcome := range result.Outcomes {
		if outcome.Err != nil {
			failed++
		}
	}
	return failed
}

type batchRuntimeEvidenceKey struct{}
type jobCommandFailuresKey struct{}
type workerJobIDKey struct{}

type batchRuntimeEvidence struct {
	mu       sync.Mutex
	retries  int
	poolWait time.Duration
}

type tenantBatchExecution struct {
	ctx       context.Context
	outcomes  []TenantOutcome
	evidence  TenantBatchEvidence
	err       error
	cancelled bool
}

type jobCommandFailures struct {
	mu  sync.Mutex
	err error
}

func (failures *jobCommandFailures) add(err error) {
	if err == nil {
		return
	}
	failures.mu.Lock()
	failures.err = errors.Join(failures.err, err)
	failures.mu.Unlock()
}

func (failures *jobCommandFailures) result() error {
	failures.mu.Lock()
	defer failures.mu.Unlock()
	return failures.err
}

func recordJobCommandFailure(ctx context.Context, err error) {
	failures, _ := ctx.Value(jobCommandFailuresKey{}).(*jobCommandFailures)
	if failures != nil {
		failures.add(err)
	}
}

func (evidence *batchRuntimeEvidence) observe(event tenant.UnitOfWorkEvent) {
	evidence.mu.Lock()
	defer evidence.mu.Unlock()
	if event.Kind == tenant.UnitOfWorkTransaction {
		evidence.retries += event.Retries
	}
	if event.Kind == tenant.UnitOfWorkPoolWait {
		evidence.poolWait += event.Duration
	}
}

func (evidence *batchRuntimeEvidence) snapshot() (int, time.Duration) {
	evidence.mu.Lock()
	defer evidence.mu.Unlock()
	return evidence.retries, evidence.poolWait
}

func (s *Scheduler) runTenantBatches(
	ctx context.Context,
	tenantIDs []int64,
	jobID string,
	command TenantCommand,
) TenantBatchResult {
	result := TenantBatchResult{Outcomes: make([]TenantOutcome, 0, len(tenantIDs))}
	if command == nil || isNilDependency(command) {
		result.Err = errors.New("tenant command is required")
		result.Backlog = len(tenantIDs)
		recordJobCommandFailure(ctx, result.Err)
		return result
	}

	stableJobID := workerJobID(ctx, JobID(jobID))
	tenantIDs = s.resumeTenantIDs(stableJobID, tenantIDs)
	for tenantBatch := range slices.Chunk(tenantIDs, tenantBatchSize) {
		if err := ctx.Err(); err != nil {
			result.Err = errors.Join(result.Err, err)
			break
		}

		batch := s.runTenantBatch(ctx, tenantBatch, stableJobID, jobID, command)
		result.Outcomes = append(result.Outcomes, batch.outcomes...)
		result.Err = errors.Join(result.Err, batch.err)
		result.Batches++
		batch.evidence.Backlog = len(tenantIDs) - len(result.Outcomes)
		s.observeTenantBatch(batch.evidence)
		s.logTenantBatch(batch.ctx, batch.evidence)
		if batch.cancelled {
			break
		}
	}

	result.Backlog = len(tenantIDs) - len(result.Outcomes)
	recordJobCommandFailure(ctx, result.Err)
	return result
}

func (s *Scheduler) runTenantBatch(
	ctx context.Context,
	tenantIDs []int64,
	jobID JobID,
	operation string,
	command TenantCommand,
) tenantBatchExecution {
	started := time.Now()
	runtimeEvidence := &batchRuntimeEvidence{}
	ctx = context.WithValue(ctx, batchRuntimeEvidenceKey{}, runtimeEvidence)
	ctx = s.withUnitOfWork(ctx)
	batch := tenantBatchExecution{ctx: ctx, outcomes: make([]TenantOutcome, 0, len(tenantIDs))}

	for _, tenantID := range tenantIDs {
		if err := ctx.Err(); err != nil {
			batch.err = errors.Join(batch.err, err)
			batch.cancelled = true
			break
		}
		outcome := s.runTenantCommand(ctx, jobID, tenantID, command, runtimeEvidence)
		batch.outcomes = append(batch.outcomes, outcome)
		if outcome.Err != nil {
			batch.err = errors.Join(batch.err, fmt.Errorf("%s tenant %d: %w", operation, tenantID, outcome.Err))
		}
	}

	retries, poolWait := runtimeEvidence.snapshot()
	batch.evidence = TenantBatchEvidence{
		JobID: jobID, Duration: time.Since(started), Processed: len(batch.outcomes),
		Failed: failedTenantOutcomes(batch.outcomes), Retries: retries, PoolWait: poolWait,
	}
	return batch
}

func failedTenantOutcomes(outcomes []TenantOutcome) int {
	failed := 0
	for _, outcome := range outcomes {
		if outcome.Err != nil {
			failed++
		}
	}
	return failed
}

func (s *Scheduler) runTenantCommand(
	ctx context.Context,
	jobID JobID,
	tenantID int64,
	command TenantCommand,
	evidence *batchRuntimeEvidence,
) TenantOutcome {
	started := time.Now()
	beforeRetries, _ := evidence.snapshot()
	outcome := TenantOutcome{TenantID: tenantID}

	id, err := tenant.NewTenantID(tenantID)
	if err != nil {
		s.observeTenantRuntime("missing_tenant")
		outcome.Err = err
		outcome.Classification = TenantOutcomeMissingTenant
	} else {
		outcome.Err = tenant.WithinTenantRetry(ctx, id, func(txCtx context.Context) error {
			return command.Execute(txCtx, id)
		})
		afterRetries, _ := evidence.snapshot()
		outcome.Retries = afterRetries - beforeRetries
		outcome.Classification = classifyTenantOutcome(outcome.Err, outcome.Retries)
	}
	outcome.Duration = time.Since(started)
	if outcome.Err == nil {
		s.tenantBatchCursors.Store(jobID, tenantID)
	}
	if outcome.Err != nil {
		s.reportTenantCommandFailure(jobID, outcome)
	}
	return outcome
}

func (s *Scheduler) resumeTenantIDs(jobID JobID, tenantIDs []int64) []int64 {
	value, ok := s.tenantBatchCursors.Load(jobID)
	if !ok {
		return tenantIDs
	}
	cursor, ok := value.(int64)
	if !ok {
		s.tenantBatchCursors.Delete(jobID)
		return tenantIDs
	}
	for index, tenantID := range tenantIDs {
		if tenantID == cursor {
			resumed := make([]int64, 0, len(tenantIDs))
			resumed = append(resumed, tenantIDs[index+1:]...)
			return append(resumed, tenantIDs[:index+1]...)
		}
	}
	s.tenantBatchCursors.Delete(jobID)
	return tenantIDs
}

func classifyTenantOutcome(err error, retries int) TenantOutcomeClassification {
	switch {
	case err == nil && retries > 0:
		return TenantOutcomeRetriedSuccess
	case err == nil:
		return TenantOutcomeSuccess
	case errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded):
		return TenantOutcomeCancelled
	case retries > 0:
		return TenantOutcomeRetryExhausted
	default:
		return TenantOutcomeCommandFailure
	}
}

func (s *Scheduler) reportTenantCommandFailure(jobID JobID, outcome TenantOutcome) {
	if outcome.Classification != TenantOutcomeMissingTenant {
		s.observeTenantRuntime("transaction_failure")
	}
	s.getLogger().Error("tenant operation failed, continuing to next tenant",
		slog.String("job_id", string(jobID)),
		slog.Int64("tenant_id", outcome.TenantID),
		slog.String("classification", string(outcome.Classification)),
		slog.Int("retries", outcome.Retries),
		slog.String("error", outcome.Err.Error()),
	)
}

func (s *Scheduler) observeTenantBatch(evidence TenantBatchEvidence) {
	if s.workerTracer.Batch != nil {
		s.workerTracer.Batch(evidence)
	}
}

func (s *Scheduler) logTenantBatch(ctx context.Context, evidence TenantBatchEvidence) {
	level := slog.LevelDebug
	if evidence.Failed > 0 {
		level = slog.LevelWarn
	}
	s.getLogger().Log(ctx, level, "tenant batch completed",
		slog.String("job_id", string(evidence.JobID)),
		slog.Duration("batch_duration", evidence.Duration),
		slog.Int("tenants_processed", evidence.Processed),
		slog.Int("tenants_failed", evidence.Failed),
		slog.Int("retries", evidence.Retries),
		slog.Int("backlog", evidence.Backlog),
		slog.Duration("db_pool_wait", evidence.PoolWait),
	)
}

func workerJobID(ctx context.Context, fallback JobID) JobID {
	jobID, _ := ctx.Value(workerJobIDKey{}).(JobID)
	if jobID == "" {
		return fallback
	}
	return jobID
}
