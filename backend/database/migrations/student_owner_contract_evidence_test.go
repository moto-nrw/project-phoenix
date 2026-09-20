package migrations

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestStudentContractEvidenceReaderIsStrict(t *testing.T) {
	t.Parallel()
	want, policy, now := studentContractEvidenceFixture()
	raw, err := json.Marshal(want)
	require.NoError(t, err)
	got, err := ReadStudentContractEvidence(strings.NewReader(string(raw)))
	require.NoError(t, err)
	require.Equal(t, want, got)
	require.NoError(t, ValidateStudentContractEvidence(got, policy, now))
	for _, input := range []string{
		"", "{", "[]", "null", `{"minimum_rollback_window":0}`,
		string(raw) + " {}", string(raw) + " null", string(raw) + " invalid",
		`{"compatibility_reads_end":0.5}`, `{"database_oid":-1}`,
		strings.Repeat(" ", 1024*1024+1),
	} {
		_, err := ReadStudentContractEvidence(strings.NewReader(input))
		require.Error(t, err)
	}
	_, err = ReadStudentContractEvidence(nil)
	require.Error(t, err)
}

func TestStudentContractEvidenceRequiresAnAgreedRollbackWindow(t *testing.T) {
	t.Parallel()
	require.ErrorContains(t, ValidateStudentContractEvidence(StudentContractEvidence{}, StudentContractPolicy{}, time.Now()),
		"rollback window policy is not configured")
}

func TestStudentContractEvidenceAcceptsCompleteObservedWindow(t *testing.T) {
	t.Parallel()
	evidence, policy, now := studentContractEvidenceFixture()
	require.NoError(t, ValidateStudentContractEvidence(evidence, policy, now))
}

func TestStudentContractOperatingPolicyRequires24HoursSchoolDayAndJobs(t *testing.T) {
	t.Parallel()
	evidence, _, now := studentContractEvidenceFixture()
	policy := studentContractOperatingPolicy()
	require.Equal(t, 24*time.Hour, policy.MinimumRollbackWindow)
	require.True(t, policy.RequireSchoolDay)
	evidence.WindowStart = now.Add(-30 * time.Hour)
	evidence.WindowEnd = now.Add(-6 * time.Hour)
	evidence.SchoolDayStart = evidence.WindowStart
	require.NoError(t, ValidateStudentContractEvidence(evidence, policy, now), "exactly 24 hours with the complete evidenced day is sufficient")
	for _, scenario := range []struct {
		name   string
		change func(*StudentContractEvidence)
		want   string
	}{
		{"short window", func(e *StudentContractEvidence) { e.WindowStart = e.WindowStart.Add(time.Nanosecond) }, "full rollback window"},
		{"missing school day", func(e *StudentContractEvidence) { e.SchoolDayStart = time.Time{} }, "complete regular school day"},
		{"partial school day", func(e *StudentContractEvidence) { e.SchoolDayStart = e.WindowStart.Add(-time.Minute) }, "complete regular school day"},
		{"school day not finished", func(e *StudentContractEvidence) { e.SchoolDayEnd = e.WindowEnd.Add(time.Minute) }, "complete regular school day"},
		{"missing school evidence", func(e *StudentContractEvidence) { e.SchoolDayEvidence = " " }, "complete regular school day"},
		{"missing jobs", func(e *StudentContractEvidence) { e.JobsEvidence = "" }, "relevant jobs"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			changed := evidence
			scenario.change(&changed)
			require.ErrorContains(t, ValidateStudentContractEvidence(changed, policy, now), scenario.want)
		})
	}
}

func TestStudentContractEvidenceRejectsIncompleteOrContradictoryProof(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		name   string
		change func(*StudentContractEvidence)
		want   string
	}{
		{"missing database", func(e *StudentContractEvidence) { e.Database = "" }, "database and full release commit"},
		{"abbreviated commit", func(e *StudentContractEvidence) { e.ReleaseCommit = "ed76e88" }, "database and full release commit"},
		{"missing cluster", func(e *StudentContractEvidence) { e.SystemIdentifier = "" }, "cluster identifier and database OID"},
		{"missing database OID", func(e *StudentContractEvidence) { e.DatabaseOID = 0 }, "cluster identifier and database OID"},
		{"single measurement", func(e *StudentContractEvidence) { e.WindowStart = e.WindowEnd }, "full rollback window"},
		{"before cutover", func(e *StudentContractEvidence) { e.CutoverAt = e.WindowStart.Add(time.Second) }, "full rollback window"},
		{"future observation", func(e *StudentContractEvidence) { e.WindowEnd = e.WindowEnd.Add(2 * time.Hour) }, "full rollback window"},
		{"stale observation", func(e *StudentContractEvidence) {
			e.WindowStart = e.WindowStart.Add(-2 * time.Hour)
			e.WindowEnd = e.WindowEnd.Add(-2 * time.Hour)
			e.CutoverAt = e.WindowStart.Add(-time.Hour)
		}, "observation is stale"},
		{"omitted read count", func(e *StudentContractEvidence) { e.CompatibilityReadsEnd = nil }, "reads at end must be measured and zero"},
		{"compatibility read", func(e *StudentContractEvidence) { var value int64 = 2; e.CompatibilityReadsEnd = &value }, "reads at end must be measured and zero"},
		{"compatibility write", func(e *StudentContractEvidence) { var value int64 = 2; e.CompatibilityWritesEnd = &value }, "writes at end must be measured and zero"},
		{"negative count", func(e *StudentContractEvidence) { value := int64(-2); e.OldTableQueries = &value }, "old table queries must be measured and zero"},
		{"old table access", func(e *StudentContractEvidence) { var value int64 = 2; e.OldTableQueries = &value }, "old table queries must be measured and zero"},
		{"old caller", func(e *StudentContractEvidence) { var value int64 = 2; e.OldCallers = &value }, "old callers must be measured and zero"},
		{"statistics reset", func(e *StudentContractEvidence) { e.StatisticsResetEnd = e.WindowEnd }, "without a reset"},
		{"statistics began late", func(e *StudentContractEvidence) {
			e.StatisticsResetStart = e.WindowStart.Add(time.Second)
			e.StatisticsResetEnd = e.StatisticsResetStart
		}, "without a reset"},
		{"missing caller artifact", func(e *StudentContractEvidence) { e.CallerEvidence = " " }, "evidence references are required"},
		{"missing query snapshot", func(e *StudentContractEvidence) { e.OldQueryFingerprintEnd = "" }, "old-object query fingerprint"},
		{"statistics evicted", func(e *StudentContractEvidence) { var value int64 = 3; e.StatisticsDeallocEnd = &value }, "eviction counters must be measured and unchanged"},
		{"statistics eviction unmeasured", func(e *StudentContractEvidence) { e.StatisticsDeallocStart = nil }, "eviction counters must be measured and unchanged"},
		{"missing backup checksum", func(e *StudentContractEvidence) { e.Backup.SHA256 = "" }, "backup checksum"},
		{"mutable prior image", func(e *StudentContractEvidence) { e.Backup.PriorImage = "phoenix:latest" }, "immutable prior image"},
		{"invalid image digest", func(e *StudentContractEvidence) { e.Backup.PriorImage = "phoenix@sha256:bad" }, "prior image digest is invalid"},
		{"old release backup", func(e *StudentContractEvidence) { e.Backup.CompletedAt = e.CutoverAt }, "fresh backup"},
		{"future backup", func(e *StudentContractEvidence) { e.Backup.CompletedAt = e.WindowEnd.Add(2 * time.Hour) }, "fresh backup"},
		{"missing restore", func(e *StudentContractEvidence) { e.Backup.RestoreTestedAt = time.Time{} }, "successful restore evidence"},
		{"other backup restored", func(e *StudentContractEvidence) { e.Backup.RestoreTestedAt = e.Backup.CompletedAt.Add(-time.Second) }, "successful restore evidence"},
		{"future restore", func(e *StudentContractEvidence) { e.Backup.RestoreTestedAt = e.WindowEnd.Add(2 * time.Hour) }, "successful restore evidence"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			evidence, policy, now := studentContractEvidenceFixture()
			scenario.change(&evidence)
			require.ErrorContains(t, ValidateStudentContractEvidence(evidence, policy, now), scenario.want)
		})
	}
}

// This fixture deliberately uses a stricter window than the production policy.
func studentContractEvidenceFixture() (StudentContractEvidence, StudentContractPolicy, time.Time) {
	now := time.Date(2026, time.September, 25, 12, 0, 0, 0, time.UTC)
	zero := int64(0)
	return StudentContractEvidence{
		Database: "contract_test", ReleaseCommit: strings.Repeat("a", 40),
		SystemIdentifier: "12345", DatabaseOID: 12345,
		CutoverAt: now.Add(-72 * time.Hour), WindowStart: now.Add(-49 * time.Hour), WindowEnd: now.Add(-time.Hour),
		WindowEvidence: "observations.json", QueryEvidence: "queries.json", CallerEvidence: "callers.json",
		SchoolDayStart: now.Add(-28 * time.Hour), SchoolDayEnd: now.Add(-20 * time.Hour),
		SchoolDayEvidence: "all-schools-regular-day.json", JobsEvidence: "job-executions.json",
		CompatibilityReadsStart: &zero, CompatibilityReadsEnd: &zero,
		CompatibilityWritesStart: &zero, CompatibilityWritesEnd: &zero,
		OldTableQueries: &zero, OldCallers: &zero,
		OldQueryFingerprintEnd: strings.Repeat("e", 64),
		StatisticsResetStart:   now.Add(-72 * time.Hour), StatisticsResetEnd: now.Add(-72 * time.Hour),
		StatisticsDeallocStart: &zero, StatisticsDeallocEnd: &zero,
		Backup: StudentContractBackupEvidence{
			Reference: "pre-contract", SHA256: strings.Repeat("b", 64),
			CompletedAt: now.Add(-30 * time.Minute), RestoreTestedAt: now.Add(-10 * time.Minute),
			RestoreEvidence: "restore.json", PriorImage: "phoenix@sha256:" + strings.Repeat("c", 64),
		},
	}, StudentContractPolicy{
		MinimumRollbackWindow: 48 * time.Hour, RequireSchoolDay: true, MaximumEvidenceAge: time.Hour, MaximumBackupAge: time.Hour,
	}, now
}
