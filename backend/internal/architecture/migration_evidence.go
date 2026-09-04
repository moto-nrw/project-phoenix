package architecture

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const migrationTicketSchemaVersion = 2
const migrationTicketMaxBytes = 1 << 20

type migrationTicket struct {
	TicketKind          string                 `json:"ticket_kind"`
	CheckpointReference string                 `json:"checkpoint_reference,omitempty"`
	Checkpoint          *checkpointMeasurement `json:"checkpoint,omitempty"`
	SchemaVersion       int                    `json:"schema_version"`
	Prerequisites       []string               `json:"prerequisites"`
	OwnerAndCapability  string                 `json:"owner_and_capability"`
	AffectedPackages    []string               `json:"affected_packages"`
	AffectedTables      []string               `json:"affected_tables"`
	ExactRatchetKeys    *[]string              `json:"exact_ratchet_keys"`
	AtomicCutover       string                 `json:"atomic_cutover"`
	Tests               []string               `json:"tests"`
	RuntimeEvidence     runtimeEvidence        `json:"runtime_evidence"`
	RollbackAndCleanup  string                 `json:"rollback_and_cleanup"`
	ExitCriterion       string                 `json:"exit_criterion"`
}

type runtimeEvidence struct {
	AffectedRows string `json:"affected_rows"`
	Failure      string `json:"failure"`
	Rollback     string `json:"rollback"`
	Smoke        string `json:"smoke"`
	Source       string `json:"source"`
	Workload     string `json:"workload"`
	Thresholds   string `json:"thresholds"`
	QueryCount   string `json:"query_count"`
	LatencyP50   string `json:"latency_p50"`
	LatencyP95   string `json:"latency_p95"`
	Errors       string `json:"errors"`
	PoolWait     string `json:"pool_wait"`
	LockWait     string `json:"lock_wait"`
	Deadlocks    string `json:"deadlocks"`
	JobDuration  string `json:"job_duration"`
	JobRetries   string `json:"job_retries"`
	JobBacklog   string `json:"job_backlog"`
}

func runValidateMigrationTicket(args []string, dependencies CLIDependencies) error {
	flags := flag.NewFlagSet("validate-ticket", flag.ContinueOnError)
	checkpointsPath := flags.String("checkpoints", "backend/architecture/runtime-checkpoints.json", "reviewed checkpoint acceptance registry (relative to repository root)")
	ticketPath := flags.String("ticket", "", "migration ticket JSON")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *ticketPath == "" {
		return fmt.Errorf("validate-ticket requires --ticket and no positional arguments")
	}
	path := *ticketPath
	if !filepath.IsAbs(path) {
		path = filepath.Join(dependencies.ProjectRoot, path)
	}
	ticket, err := loadMigrationTicket(path)
	if err != nil {
		return err
	}
	if err := ticket.validate(); err != nil {
		return fmt.Errorf("validate migration ticket: %w", err)
	}
	if ticket.TicketKind == "migration" {
		registryPath := *checkpointsPath
		if !filepath.IsAbs(registryPath) {
			registryPath = filepath.Join(dependencies.ProjectRoot, registryPath)
		}
		var registry checkpointRegistry
		if err := loadTicketJSON(registryPath, &registry); err != nil {
			return err
		}
		if err := registry.validateReference(ticket.CheckpointReference); err != nil {
			return err
		}
	}
	fmt.Println("migration ticket passed")
	return nil
}

func loadMigrationTicket(path string) (*migrationTicket, error) {
	var ticket migrationTicket
	if err := loadTicketJSON(path, &ticket); err != nil {
		return nil, err
	}
	return &ticket, nil
}

func loadTicketJSON(path string, target any) error {
	file, err := os.Open(path) // #nosec G304 -- path is an explicit CLI input
	if err != nil {
		return fmt.Errorf("open migration ticket: %w", err)
	}
	defer func() { _ = file.Close() }()
	contents, err := io.ReadAll(io.LimitReader(file, migrationTicketMaxBytes+1))
	if err != nil {
		return fmt.Errorf("read migration ticket: %w", err)
	}
	if len(contents) > migrationTicketMaxBytes {
		return fmt.Errorf("migration ticket must not exceed %d bytes", migrationTicketMaxBytes)
	}
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return fmt.Errorf("decode migration ticket: %w", err)
	}
	if err := requireJSONEnd(decoder); err != nil {
		return err
	}
	return nil
}

func (ticket migrationTicket) validate() error {
	if ticket.SchemaVersion != migrationTicketSchemaVersion {
		return fmt.Errorf("schema_version must be %d, got %d", migrationTicketSchemaVersion, ticket.SchemaVersion)
	}
	if ticket.ExactRatchetKeys == nil {
		return fmt.Errorf("exact_ratchet_keys is required; use an empty array for a zero-key ticket")
	}
	if err := requireRatchetKeys(*ticket.ExactRatchetKeys); err != nil {
		return err
	}
	for name, value := range map[string]string{
		"owner_and_capability": ticket.OwnerAndCapability,
		"atomic_cutover":       ticket.AtomicCutover,
		"rollback_and_cleanup": ticket.RollbackAndCleanup,
		"exit_criterion":       ticket.ExitCriterion,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s is required", name)
		}
	}
	for name, values := range map[string][]string{
		"prerequisites":     ticket.Prerequisites,
		"affected_packages": ticket.AffectedPackages,
		"affected_tables":   ticket.AffectedTables,
		"tests":             ticket.Tests,
	} {
		if err := requireTicketList(name, values); err != nil {
			return err
		}
	}
	switch ticket.TicketKind {
	case "migration":
		if ticket.Checkpoint != nil {
			return fmt.Errorf("migration tickets must not supply checkpoint measurements")
		}
		if checkpointIndex(ticket.CheckpointReference) < 0 {
			return fmt.Errorf("checkpoint_reference must be a canonical checkpoint issue URL (#3019, #3020, #3021)")
		}
		return ticket.RuntimeEvidence.validateFlow()
	case "checkpoint":
		if ticket.CheckpointReference != "" {
			return fmt.Errorf("checkpoint tickets must not use checkpoint_reference")
		}
		if ticket.Checkpoint == nil {
			return fmt.Errorf("checkpoint is required")
		}
		if err := ticket.RuntimeEvidence.validate(); err != nil {
			return err
		}
		return ticket.Checkpoint.validate()
	default:
		return fmt.Errorf("ticket_kind must be migration or checkpoint")
	}
}

func requireRatchetKeys(keys []string) error {
	seen := make(map[string]struct{}, len(keys))
	for _, key := range keys {
		if strings.TrimSpace(key) == "" {
			return fmt.Errorf("exact_ratchet_keys contains an empty value")
		}
		parts := strings.Split(key, "|")
		if len(parts) != 4 || slicesContainBlankOrPadded(parts) {
			return fmt.Errorf("exact_ratchet_keys entry %q must use scope|rule|source|target", key)
		}
		if err := validateLegacyViolationFields(Violation{
			Scope: Scope(parts[0]), Rule: parts[1], Source: parts[2], Target: parts[3],
		}); err != nil {
			return fmt.Errorf("exact_ratchet_keys entry %q: %w", key, err)
		}
		if _, exists := seen[key]; exists {
			return fmt.Errorf("exact_ratchet_keys contains duplicate key %q", key)
		}
		seen[key] = struct{}{}
	}
	return nil
}

func slicesContainBlankOrPadded(values []string) bool {
	for _, value := range values {
		if strings.TrimSpace(value) == "" || strings.TrimSpace(value) != value {
			return true
		}
	}
	return false
}

func requireTicketList(name string, values []string) error {
	if len(values) == 0 {
		return fmt.Errorf("%s must not be empty", name)
	}
	for _, value := range values {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("%s contains an empty value", name)
		}
	}
	return nil
}

func (evidence runtimeEvidence) validate() error {
	for name, value := range map[string]string{
		"source": evidence.Source, "workload": evidence.Workload, "thresholds": evidence.Thresholds,
		"affected_rows": evidence.AffectedRows,
		"query_count":   evidence.QueryCount, "latency_p50": evidence.LatencyP50, "latency_p95": evidence.LatencyP95,
		"errors": evidence.Errors, "pool_wait": evidence.PoolWait, "lock_wait": evidence.LockWait,
		"deadlocks": evidence.Deadlocks, "job_duration": evidence.JobDuration,
		"job_retries": evidence.JobRetries, "job_backlog": evidence.JobBacklog,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("runtime_evidence.%s is required", name)
		}
	}
	return nil
}

// Acceptance is reviewed separately from the ticket being validated.
type checkpointRegistry struct {
	SchemaVersion int                    `json:"schema_version"`
	Accepted      []checkpointAcceptance `json:"accepted"`
}

type checkpointAcceptance struct {
	Issue      string `json:"issue"`
	Acceptance string `json:"acceptance"`
}

var checkpointIssues = []string{
	"https://github.com/moto-nrw/project-phoenix/issues/3019",
	"https://github.com/moto-nrw/project-phoenix/issues/3020",
	"https://github.com/moto-nrw/project-phoenix/issues/3021",
}

func checkpointIndex(issue string) int {
	for i, known := range checkpointIssues {
		if issue == known {
			return i
		}
	}
	return -1
}

func (registry checkpointRegistry) validateReference(reference string) error {
	if registry.SchemaVersion != 1 {
		return fmt.Errorf("checkpoint registry schema_version must be 1")
	}
	if len(registry.Accepted) == 0 {
		return fmt.Errorf("no accepted runtime checkpoint; acceptance of #3019 is required first")
	}
	for i, entry := range registry.Accepted {
		if i >= len(checkpointIssues) || entry.Issue != checkpointIssues[i] {
			return fmt.Errorf("accepted checkpoints must be a contiguous ordered prefix of #3019, #3020, #3021")
		}
		if !regexp.MustCompile(`^` + regexp.QuoteMeta(entry.Issue) + `#issuecomment-[1-9][0-9]*$`).MatchString(entry.Acceptance) {
			return fmt.Errorf("checkpoint acceptance must link an explicit acceptance comment on %s", entry.Issue)
		}
	}
	if reference != registry.Accepted[len(registry.Accepted)-1].Issue {
		return fmt.Errorf("checkpoint_reference must identify the current accepted checkpoint (not a future, unaccepted, or superseded checkpoint)")
	}
	return nil
}

func (evidence runtimeEvidence) validateFlow() error {
	for name, value := range map[string]string{
		"source": evidence.Source, "workload": evidence.Workload, "thresholds": evidence.Thresholds,
		"query_count": evidence.QueryCount, "errors": evidence.Errors,
		"failure": evidence.Failure, "rollback": evidence.Rollback, "smoke": evidence.Smoke,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("runtime_evidence.%s is required", name)
		}
	}
	return nil
}

type checkpointMeasurement struct {
	Issue            string            `json:"issue"`
	Commit           string            `json:"commit"`
	Environment      string            `json:"environment"`
	Toolchain        string            `json:"toolchain"`
	WorkloadVersion  string            `json:"workload_version"`
	DataVolume       string            `json:"data_volume"`
	Concurrency      string            `json:"concurrency"`
	WarmUp           string            `json:"warm_up"`
	Runs             []runtimeEvidence `json:"runs"`
	Median           runtimeEvidence   `json:"median"`
	Worst            runtimeEvidence   `json:"worst"`
	Comparison       string            `json:"comparison"`
	WorkloadBridge   string            `json:"workload_bridge"`
	RegressionIssues string            `json:"regression_issues"`
	Decision         string            `json:"decision"`
}

func (measurement checkpointMeasurement) validate() error {
	if checkpointIndex(measurement.Issue) < 0 {
		return fmt.Errorf("checkpoint.issue must be #3019, #3020, or #3021 as a canonical issue URL")
	}
	if !regexp.MustCompile(`^[0-9a-f]{40}$`).MatchString(measurement.Commit) {
		return fmt.Errorf("checkpoint.commit must be a full lowercase commit SHA")
	}
	for name, value := range map[string]string{
		"environment": measurement.Environment, "toolchain": measurement.Toolchain,
		"workload_version": measurement.WorkloadVersion, "data_volume": measurement.DataVolume,
		"concurrency": measurement.Concurrency, "warm_up": measurement.WarmUp,
		"comparison":      measurement.Comparison,
		"workload_bridge": measurement.WorkloadBridge, "regression_issues": measurement.RegressionIssues,
		"decision": measurement.Decision,
	} {
		if strings.TrimSpace(value) == "" {
			return fmt.Errorf("checkpoint.%s is required", name)
		}
	}
	if len(measurement.Runs) != 3 {
		return fmt.Errorf("checkpoint.runs must contain exactly three measured runs after warm-up")
	}
	for i, run := range measurement.Runs {
		if err := run.validate(); err != nil {
			return fmt.Errorf("checkpoint.runs[%d]: %w", i, err)
		}
	}
	if err := measurement.Median.validate(); err != nil {
		return fmt.Errorf("checkpoint.median: %w", err)
	}
	if err := measurement.Worst.validate(); err != nil {
		return fmt.Errorf("checkpoint.worst: %w", err)
	}
	return nil
}
