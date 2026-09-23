package architecture

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
)

const migrationHistoryAnchor = "e4bb1a38c94180ae337a68726bd565bcfe23ea00"
const evidenceDirectory = "backend/architecture"

// These documents have other contracts; none can opt out by dropping ticket_kind.
var nonTicketDocuments = map[string]string{
	"policy.json":                        "architecture policy, checked by check",
	"composition.json":                   "composition inventory, checked by composition tests",
	"runtime-checkpoints.json":           "acceptance registry, checked by migration validation",
	"contract-active-2737-progress.json": "historical progress notes, not cutover evidence",
}

func evidenceTemplate(root, path string) bool {
	for _, name := range []string{"migration-ticket-template.json", "checkpoint-ticket-template.json"} {
		if filepath.Clean(path) == filepath.Join(root, evidenceDirectory, name) {
			return true
		}
	}
	return false
}

func evidenceFiles(root string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(root, evidenceDirectory))
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".json" {
			continue
		}
		if _, excluded := nonTicketDocuments[entry.Name()]; excluded {
			continue
		}
		if !entry.Type().IsRegular() {
			return nil, fmt.Errorf("evidence %s must be a regular file", entry.Name())
		}
		paths = append(paths, filepath.Join(root, evidenceDirectory, entry.Name()))
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("no migration evidence files discovered")
	}
	for _, name := range []string{"migration-ticket-template.json", "checkpoint-ticket-template.json"} {
		if !slices.Contains(paths, filepath.Join(root, evidenceDirectory, name)) {
			return nil, fmt.Errorf("missing shipped template %s", name)
		}
	}
	return paths, nil
}

func validateEvidenceInventory(root, checkpointsPath, baseRef string) error {
	paths, err := evidenceFiles(root)
	if err != nil {
		return err
	}
	tickets := make(map[string]*migrationTicket)
	for _, path := range paths {
		ticket, err := validateEvidenceFile(root, path, checkpointsPath)
		if err != nil {
			return fmt.Errorf("%s: %w", filepath.Base(path), err)
		}
		if !evidenceTemplate(root, path) {
			tickets[path] = ticket
		}
	}
	if baseRef != "" {
		if err := validateEvidenceDelta(root, baseRef, tickets); err != nil {
			return err
		}
	}
	fmt.Printf("migration ticket inventory passed: %d files (%d evidence records, 2 templates)\n", len(paths), len(tickets))
	return nil
}

func validateEvidenceProvenance(root, path string, ticket *migrationTicket) error {
	if evidenceTemplate(root, path) {
		return nil
	}
	if err := rejectTemplateEvidence(root, ticket); err != nil {
		return err
	}
	reports := []runtimeEvidence{ticket.RuntimeEvidence}
	if ticket.Checkpoint != nil {
		reports = append(reports, ticket.Checkpoint.Runs...)
		reports = append(reports, ticket.Checkpoint.Median, ticket.Checkpoint.Worst)
	}
	for _, report := range reports {
		for _, source := range report.Sources {
			if err := validateEvidenceSource(root, source); err != nil {
				return err
			}
		}
	}
	if ticket.hasHistoricalGaps() {
		return validateFrozenEvidence(root, path, ticket)
	}
	return nil
}

func validateEvidenceSource(root, source string) error {
	parsed, err := url.Parse(source)
	if err != nil {
		return fmt.Errorf("invalid evidence source %q: %w", source, err)
	}
	if parsed.Scheme != "" {
		if (parsed.Scheme != "https" && parsed.Scheme != "http") || parsed.Hostname() == "" || parsed.User != nil || strings.HasSuffix(parsed.Hostname(), ".invalid") {
			return fmt.Errorf("evidence source %q must be a local path or real HTTP(S) reference", source)
		}
		return nil
	}
	if filepath.IsAbs(parsed.Path) || parsed.RawQuery != "" || parsed.Path == "" {
		return fmt.Errorf("local evidence source %q must be repository-relative", source)
	}
	local := filepath.Join(root, filepath.FromSlash(parsed.Path))
	physical, err := filepath.EvalSymlinks(local)
	if err != nil {
		return fmt.Errorf("local evidence source %q: %w", source, err)
	}
	physicalRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	relative, err := filepath.Rel(physicalRoot, physical)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return fmt.Errorf("evidence source %q escapes the repository", source)
	}
	info, err := os.Stat(physical)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("evidence source %q must be a file", source)
	}
	return nil
}

func rejectTemplateEvidence(root string, ticket *migrationTicket) error {
	name := ticket.TicketKind + "-ticket-template.json"
	template, err := loadMigrationTicket(filepath.Join(root, evidenceDirectory, name))
	if err != nil {
		return err
	}
	if err := template.validate(); err != nil {
		return fmt.Errorf("invalid shipped template %s: %w", name, err)
	}
	if err := rejectExampleReport(ticket.RuntimeEvidence, template.RuntimeEvidence); err != nil {
		return err
	}
	if ticket.Checkpoint != nil && template.Checkpoint != nil {
		for i, report := range ticket.Checkpoint.Runs {
			if err := rejectExampleReport(report, template.Checkpoint.Runs[i]); err != nil {
				return fmt.Errorf("checkpoint.runs[%d]: %w", i, err)
			}
		}
		if err := rejectExampleReport(ticket.Checkpoint.Median, template.Checkpoint.Median); err != nil {
			return fmt.Errorf("checkpoint.median: %w", err)
		}
		if err := rejectExampleReport(ticket.Checkpoint.Worst, template.Checkpoint.Worst); err != nil {
			return fmt.Errorf("checkpoint.worst: %w", err)
		}
	}
	return nil
}

func rejectExampleReport(report, template runtimeEvidence) error {
	for field, value := range map[string]string{"source": report.Source, "workload": report.Workload, "thresholds": report.Thresholds} {
		example := map[string]string{"source": template.Source, "workload": template.Workload, "thresholds": template.Thresholds}[field]
		if value == example {
			return fmt.Errorf("runtime_evidence.%s contains unchanged template evidence", field)
		}
	}
	for field, metric := range report.metrics() {
		example := template.metrics()[field]
		if (metric.Result != "" && metric.Result == example.Result) || (metric.Reason != "" && metric.Reason == example.Reason) || (metric.Legacy != "" && metric.Legacy == example.Legacy) {
			return fmt.Errorf("runtime_evidence.%s contains unchanged template evidence", field)
		}
	}
	return nil
}

func validateFrozenEvidence(root, path string, ticket *migrationTicket) error {
	name := filepath.Base(path)
	fields, exists := historicalMetricGaps[name]
	if !exists || filepath.Clean(path) != filepath.Join(root, evidenceDirectory, name) {
		return fmt.Errorf("historical_gap is not authorized for %s", path)
	}
	data, err := readGitBlob(root, root, migrationHistoryAnchor, filepath.Join(evidenceDirectory, name))
	if err != nil {
		return fmt.Errorf("read frozen evidence: %w", err)
	}
	var original migrationTicket
	if err := json.Unmarshal(data, &original); err != nil {
		return err
	}
	// A filename is not permission to replace an old flow with a new one.
	oldMetadata, newMetadata := original, *ticket
	oldMetadata.RuntimeEvidence, newMetadata.RuntimeEvidence = runtimeEvidence{}, runtimeEvidence{}
	oldMetadata.SchemaVersion, newMetadata.SchemaVersion = 0, 0
	oldMetadata.CheckpointReference, newMetadata.CheckpointReference = "", ""
	if !reflect.DeepEqual(oldMetadata, newMetadata) || ticket.RuntimeEvidence.Source != original.RuntimeEvidence.Source || ticket.RuntimeEvidence.Workload != original.RuntimeEvidence.Workload || ticket.RuntimeEvidence.Thresholds != original.RuntimeEvidence.Thresholds {
		return fmt.Errorf("historical_gap cannot authorize changed scope or provenance in %s", name)
	}
	for field, metric := range ticket.RuntimeEvidence.metrics() {
		if metric.State == "historical_gap" && (!slices.Contains(fields, field) || metric.Original != original.RuntimeEvidence.metrics()[field].Legacy) {
			return fmt.Errorf("runtime_evidence.%s historical_gap differs from the frozen file/metric at %s", field, migrationHistoryAnchor)
		}
	}
	return nil
}

// Read declarations from an immutable tree, never from a ticket-local flag.
func previousEvidenceClaims(root, ref string) (map[string]map[string]bool, error) {
	listing, err := gitOutput(root, "ls-tree", "--name-only", ref, "--", evidenceDirectory+"/")
	if err != nil {
		return nil, err
	}
	claims := make(map[string]map[string]bool)
	for _, path := range strings.Split(strings.TrimSpace(listing), "\n") {
		if filepath.Ext(path) != ".json" {
			continue
		}
		data, err := readGitBlob(root, root, ref, path)
		if err != nil {
			return nil, err
		}
		var document struct {
			Keys []string `json:"exact_ratchet_keys"`
		}
		if err := json.NewDecoder(bytes.NewReader(data)).Decode(&document); err != nil {
			return nil, err
		}
		claims[filepath.Base(path)] = make(map[string]bool)
		for _, key := range document.Keys {
			claims[filepath.Base(path)][key] = true
		}
	}
	return claims, nil
}
