package architecture

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

// Checkpoints retain schema 2 strings. Migration schema 3 requires an explicit
// state; accepting a legacy string during decoding never grants migration validity.
type evidenceMetric struct {
	State    string `json:"state"`
	Result   string `json:"result,omitempty"`
	Reason   string `json:"reason,omitempty"`
	Original string `json:"original,omitempty"`
	Legacy   string `json:"-"`
}

func (metric *evidenceMetric) UnmarshalJSON(data []byte) error {
	if bytes.HasPrefix(bytes.TrimSpace(data), []byte(`"`)) {
		return json.Unmarshal(data, &metric.Legacy)
	}
	type plainMetric evidenceMetric
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode((*plainMetric)(metric)); err != nil {
		return err
	}
	return requireJSONEnd(decoder)
}

func (metric evidenceMetric) validate() error {
	if metric.State == "" {
		return fmt.Errorf("is required as structured evidence (measured, not_applicable or historical_gap)")
	}
	switch metric.State {
	case "measured":
		if !meaningfulEvidence(metric.Result) || metric.Reason != "" || metric.Original != "" {
			return fmt.Errorf("measured requires an observed result, without reason or original")
		}
	case "not_applicable", "historical_gap":
		if !meaningfulEvidence(metric.Reason) || metric.Result != "" {
			return fmt.Errorf("%s requires a concrete reason, without a result", metric.State)
		}
		if metric.State == "not_applicable" && metric.Original != "" {
			return fmt.Errorf("not_applicable must not supply historical original text")
		}
	default:
		return fmt.Errorf("has unknown evidence state %q", metric.State)
	}
	return nil
}

func meaningfulEvidence(value string) bool {
	value = strings.TrimSpace(value)
	switch strings.ToLower(strings.Trim(value, ".:;")) {
	case "", "n/a", "na", "not applicable", "todo", "tbd", "pending", "unknown", "none", "-":
		return false
	}
	return true
}

func (evidence runtimeEvidence) metrics() map[string]evidenceMetric {
	return map[string]evidenceMetric{
		"affected_rows": evidence.AffectedRows, "query_count": evidence.QueryCount,
		"latency_p50": evidence.LatencyP50, "latency_p95": evidence.LatencyP95,
		"errors": evidence.Errors, "pool_wait": evidence.PoolWait, "lock_wait": evidence.LockWait,
		"deadlocks": evidence.Deadlocks, "job_duration": evidence.JobDuration,
		"job_retries": evidence.JobRetries, "job_backlog": evidence.JobBacklog,
		"failure": evidence.Failure, "rollback": evidence.Rollback, "smoke": evidence.Smoke,
	}
}

func (ticket migrationTicket) hasHistoricalGaps() bool {
	for _, metric := range ticket.RuntimeEvidence.metrics() {
		if metric.State == "historical_gap" {
			return true
		}
	}
	return false
}
