package migrations

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"
)

// ReadStudentContractEvidence reads exactly one bounded JSON document. Parsing
// is deliberately separate from operational validation: neither successful
// decoding nor a user-supplied policy in a file authorizes destructive DDL.
func ReadStudentContractEvidence(input io.Reader) (StudentContractEvidence, error) {
	var evidence StudentContractEvidence
	if input == nil {
		return evidence, errors.New("student contract: evidence reader is required")
	}
	const maximumBytes = 1024 * 1024
	raw, err := io.ReadAll(io.LimitReader(input, maximumBytes+1))
	if err != nil {
		return evidence, fmt.Errorf("student contract: read evidence: %w", err)
	}
	if len(raw) > maximumBytes {
		return evidence, errors.New("student contract: evidence exceeds 1 MiB")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var document *StudentContractEvidence
	if err := decoder.Decode(&document); err != nil {
		return StudentContractEvidence{}, fmt.Errorf("student contract: decode evidence: %w", err)
	}
	if document == nil {
		return evidence, errors.New("student contract: evidence must be a JSON object")
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return StudentContractEvidence{}, errors.New("student contract: evidence must contain exactly one JSON document")
	}
	return *document, nil
}

// StudentContractPolicy is the reviewed operating policy, not a value supplied
// by the evidence file. A zero policy never authorizes destructive DDL.
type StudentContractPolicy struct {
	MinimumRollbackWindow time.Duration
	RequireSchoolDay      bool
	MaximumEvidenceAge    time.Duration
	MaximumBackupAge      time.Duration
}

// StudentContractEvidence records operator observations, not predictions from
// source code. Validation checks consistency; it cannot authenticate an
// operator's claim. Retain the referenced raw artifacts for review.
type StudentContractEvidence struct {
	Database          string    `json:"database"`
	SystemIdentifier  string    `json:"system_identifier"`
	DatabaseOID       uint32    `json:"database_oid"`
	ReleaseCommit     string    `json:"release_commit"`
	CutoverAt         time.Time `json:"cutover_at"`
	WindowStart       time.Time `json:"window_start"`
	WindowEnd         time.Time `json:"window_end"`
	WindowEvidence    string    `json:"window_evidence"`
	SchoolDayStart    time.Time `json:"school_day_start"`
	SchoolDayEnd      time.Time `json:"school_day_end"`
	SchoolDayEvidence string    `json:"school_day_evidence"`
	JobsEvidence      string    `json:"jobs_evidence"`

	// Pointers distinguish an observed zero from an omitted measurement.
	CompatibilityReadsStart  *int64 `json:"compatibility_reads_start"`
	CompatibilityReadsEnd    *int64 `json:"compatibility_reads_end"`
	CompatibilityWritesStart *int64 `json:"compatibility_writes_start"`
	CompatibilityWritesEnd   *int64 `json:"compatibility_writes_end"`
	OldTableQueries          *int64 `json:"old_table_queries"`
	OldCallers               *int64 `json:"old_callers"`
	QueryEvidence            string `json:"query_evidence"`
	CallerEvidence           string `json:"caller_evidence"`
	OldQueryFingerprintEnd   string `json:"old_query_fingerprint_end"`

	// Both ends identify the same statistics epoch. A reset loses coverage.
	StatisticsResetStart   time.Time                     `json:"statistics_reset_start"`
	StatisticsResetEnd     time.Time                     `json:"statistics_reset_end"`
	StatisticsDeallocStart *int64                        `json:"statistics_dealloc_start"`
	StatisticsDeallocEnd   *int64                        `json:"statistics_dealloc_end"`
	Backup                 StudentContractBackupEvidence `json:"backup"`
}

// StudentContractBackupEvidence must identify the fresh pre-Contract snapshot
// and a successful restore of that exact snapshot with its prior image.
type StudentContractBackupEvidence struct {
	Reference       string    `json:"reference"`
	SHA256          string    `json:"sha256"`
	CompletedAt     time.Time `json:"completed_at"`
	RestoreTestedAt time.Time `json:"restore_tested_at"`
	RestoreEvidence string    `json:"restore_evidence"`
	PriorImage      string    `json:"prior_image"`
}

var studentContractCommitPattern = regexp.MustCompile(`^[0-9a-f]{40}$`)
var studentContractSHA256Pattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

// ValidateStudentContractEvidence refuses missing, stale or contradictory
// evidence. Callers must also compare the database identity and live counters,
// recheck data integrity under the DDL lock, and bind the caller inventory to
// the image being deployed. This function alone does not authorize Contract.
func ValidateStudentContractEvidence(e StudentContractEvidence, policy StudentContractPolicy, now time.Time) error {
	if policy.MinimumRollbackWindow <= 0 || policy.MaximumEvidenceAge <= 0 || policy.MaximumBackupAge <= 0 {
		return errors.New("student contract: rollback window policy is not configured")
	}
	if strings.TrimSpace(e.Database) == "" || !studentContractCommitPattern.MatchString(e.ReleaseCommit) {
		return errors.New("student contract: database and full release commit are required")
	}
	if e.SystemIdentifier == "" || e.DatabaseOID == 0 {
		return errors.New("student contract: cluster identifier and database OID are required")
	}
	if e.CutoverAt.IsZero() || e.WindowStart.Before(e.CutoverAt) || e.WindowEnd.After(now) ||
		e.WindowEnd.Sub(e.WindowStart) < policy.MinimumRollbackWindow {
		return errors.New("student contract: full rollback window has not been observed after cutover")
	}
	if now.Sub(e.WindowEnd) > policy.MaximumEvidenceAge {
		return errors.New("student contract: observation is stale")
	}
	if policy.RequireSchoolDay {
		if e.SchoolDayStart.IsZero() || !e.SchoolDayEnd.After(e.SchoolDayStart) ||
			e.SchoolDayStart.Before(e.WindowStart) || e.SchoolDayEnd.After(e.WindowEnd) ||
			strings.TrimSpace(e.SchoolDayEvidence) == "" {
			return errors.New("student contract: a complete regular school day within the observed window must be evidenced")
		}
	}
	if strings.TrimSpace(e.JobsEvidence) == "" {
		return errors.New("student contract: relevant jobs must have execution evidence within the observed window")
	}
	for _, measurement := range []struct {
		name  string
		value *int64
	}{
		{"compatibility reads at start", e.CompatibilityReadsStart},
		{"compatibility reads at end", e.CompatibilityReadsEnd},
		{"compatibility writes at start", e.CompatibilityWritesStart},
		{"compatibility writes at end", e.CompatibilityWritesEnd},
		{"old table queries", e.OldTableQueries},
		{"old callers", e.OldCallers},
	} {
		if measurement.value == nil || *measurement.value != 0 {
			return fmt.Errorf("student contract: %s must be measured and zero", measurement.name)
		}
	}
	if e.StatisticsResetStart.IsZero() || e.StatisticsResetStart.After(e.WindowStart) ||
		!e.StatisticsResetStart.Equal(e.StatisticsResetEnd) {
		return errors.New("student contract: query statistics do not cover the full window without a reset")
	}
	if e.StatisticsDeallocStart == nil || e.StatisticsDeallocEnd == nil ||
		*e.StatisticsDeallocStart < 0 || *e.StatisticsDeallocStart != *e.StatisticsDeallocEnd {
		return errors.New("student contract: query statistics eviction counters must be measured and unchanged")
	}
	if !studentContractSHA256Pattern.MatchString(e.OldQueryFingerprintEnd) {
		return errors.New("student contract: reviewed old-object query fingerprint is required")
	}
	for _, source := range []string{e.WindowEvidence, e.QueryEvidence, e.CallerEvidence, e.Backup.Reference, e.Backup.RestoreEvidence} {
		if strings.TrimSpace(source) == "" {
			return errors.New("student contract: raw observation, caller and backup evidence references are required")
		}
	}
	return validateStudentContractBackup(e.Backup, e.WindowEnd, policy.MaximumBackupAge, now)
}

func validateStudentContractBackup(e StudentContractBackupEvidence, windowEnd time.Time, maximumAge time.Duration, now time.Time) error {
	if !studentContractSHA256Pattern.MatchString(e.SHA256) ||
		!strings.Contains(e.PriorImage, "@sha256:") {
		return errors.New("student contract: backup checksum and immutable prior image are required")
	}
	_, digest, _ := strings.Cut(e.PriorImage, "@sha256:")
	if !studentContractSHA256Pattern.MatchString(digest) {
		return errors.New("student contract: prior image digest is invalid")
	}
	if e.CompletedAt.IsZero() || e.CompletedAt.Before(windowEnd) || e.CompletedAt.After(now) ||
		now.Sub(e.CompletedAt) > maximumAge {
		return errors.New("student contract: fresh backup after the observed window is required")
	}
	if e.RestoreTestedAt.Before(e.CompletedAt) || e.RestoreTestedAt.After(now) {
		return errors.New("student contract: successful restore evidence for this backup is required")
	}
	return nil
}
