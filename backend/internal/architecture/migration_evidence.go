package architecture

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const migrationTicketSchemaVersion = 1
const migrationTicketMaxBytes = 1 << 20

type migrationTicket struct {
	SchemaVersion      int             `json:"schema_version"`
	Prerequisites      []string        `json:"prerequisites"`
	OwnerAndCapability string          `json:"owner_and_capability"`
	AffectedPackages   []string        `json:"affected_packages"`
	AffectedTables     []string        `json:"affected_tables"`
	ExactRatchetKeys   *[]string       `json:"exact_ratchet_keys"`
	AtomicCutover      string          `json:"atomic_cutover"`
	Tests              []string        `json:"tests"`
	RuntimeEvidence    runtimeEvidence `json:"runtime_evidence"`
	RollbackAndCleanup string          `json:"rollback_and_cleanup"`
	ExitCriterion      string          `json:"exit_criterion"`
}

type runtimeEvidence struct {
	Source      string `json:"source"`
	Workload    string `json:"workload"`
	Thresholds  string `json:"thresholds"`
	QueryCount  string `json:"query_count"`
	LatencyP50  string `json:"latency_p50"`
	LatencyP95  string `json:"latency_p95"`
	Errors      string `json:"errors"`
	PoolWait    string `json:"pool_wait"`
	LockWait    string `json:"lock_wait"`
	Deadlocks   string `json:"deadlocks"`
	JobDuration string `json:"job_duration"`
	JobRetries  string `json:"job_retries"`
	JobBacklog  string `json:"job_backlog"`
}

func runValidateMigrationTicket(args []string, dependencies CLIDependencies) error {
	flags := flag.NewFlagSet("validate-ticket", flag.ContinueOnError)
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
	fmt.Println("migration ticket passed")
	return nil
}

func loadMigrationTicket(path string) (*migrationTicket, error) {
	file, err := os.Open(path) // #nosec G304 -- path is an explicit CLI input
	if err != nil {
		return nil, fmt.Errorf("open migration ticket: %w", err)
	}
	defer func() { _ = file.Close() }()
	contents, err := io.ReadAll(io.LimitReader(file, migrationTicketMaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read migration ticket: %w", err)
	}
	if len(contents) > migrationTicketMaxBytes {
		return nil, fmt.Errorf("migration ticket must not exceed %d bytes", migrationTicketMaxBytes)
	}
	decoder := json.NewDecoder(bytes.NewReader(contents))
	decoder.DisallowUnknownFields()
	var ticket migrationTicket
	if err := decoder.Decode(&ticket); err != nil {
		return nil, fmt.Errorf("decode migration ticket: %w", err)
	}
	if err := requireJSONEnd(decoder); err != nil {
		return nil, err
	}
	return &ticket, nil
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
	return ticket.RuntimeEvidence.validate()
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
		"query_count": evidence.QueryCount, "latency_p50": evidence.LatencyP50, "latency_p95": evidence.LatencyP95,
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
