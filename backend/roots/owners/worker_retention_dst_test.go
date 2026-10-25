package owners

import (
	"context"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestNightlyRetentionCleanupCutoffsAcrossDST(t *testing.T) {
	t.Parallel()
	suite := newRetentionSuite(t)
	counter := testpkg.CaptureQueriesForContext(t, suite.admin)
	ctx := counter.Context(context.Background())
	// Capture the real owner queries as superuser, so today's database clock
	// cannot reject a cutoff before we replay it at the pinned instants below.
	for _, step := range suite.retention.steps {
		switch step.Kind {
		case "file_cleanup_retention", "staff_cleanup_retention", "student_document_cleanup_retention", "removed_file_retention", "care_withdrawal_retention":
			require.NoError(t, testpkg.WithinTenantContext(t, ctx, suite.admin, testpkg.Tenant(t), func(ctx context.Context) error {
				_, err := step.Run(ctx)
				return err
			}))
		}
	}
	counter.Stop()
	for _, function := range []string{
		"documents.delete_finished_file_cleanups",
		"documents.delete_finished_announcement_attachment_cleanups",
		"users.delete_finished_staff_document_file_cleanups",
		"users.delete_finished_student_document_file_cleanups",
		"documents.delete_removed_files",
	} {
		queries := counter.Selects(function)
		require.Len(t, queries, 1, function)
		_, cutoff, found := strings.Cut(queries[0], ", ")
		require.True(t, found, queries[0])
		cutoff = strings.TrimSuffix(cutoff, ")")
		require.Contains(t, cutoff, "statement_timestamp()")
		cutoff = strings.ReplaceAll(cutoff, "statement_timestamp()", "?::timestamptz")
		for _, month := range []time.Month{time.March, time.October} {
			day := 29
			if month == time.October {
				day = 25
			}
			now := time.Date(2026, month, day, 12, 0, 0, 0, calendar.Berlin)
			var actual time.Time
			require.NoError(t, suite.admin.NewRaw("SELECT "+cutoff, now).Scan(context.Background(), &actual))
			// The SECURITY DEFINER guards use clock_timestamp() - INTERVAL
			// '30 days': keep the same local clock across 23/25-hour days.
			require.True(t, now.AddDate(0, 0, -30).Equal(actual), "%s at %s: got %s", function, now, actual)
		}
	}
	for table, days := range map[string]int{"users.staff_offboarding_cleanup": 30, "users.care_withdrawal_completions": 365} {
		var queries []string
		for _, query := range counter.Queries() {
			if strings.Contains(query, table) {
				queries = append(queries, query)
			}
		}
		require.Len(t, queries, 1, table)
		assertCalendarDayCutoff(t, suite, queries[0], table, days)
	}
}

var retentionCutoff = regexp.MustCompile(`statement_timestamp\(\) - \([^)]*\)`)

func assertCalendarDayCutoff(t *testing.T, suite retentionSuite, query, name string, days int) {
	t.Helper()
	cutoff := retentionCutoff.FindString(query)
	require.NotEmpty(t, cutoff, query)
	require.Contains(t, cutoff, "INTERVAL '1 day'", name)
	cutoff = strings.ReplaceAll(cutoff, "statement_timestamp()", "?::timestamptz")
	for _, month := range []time.Month{time.March, time.October} {
		day := 29
		if month == time.October {
			day = 25
		}
		now := time.Date(2026, month, day, 12, 0, 0, 0, calendar.Berlin)
		var actual time.Time
		require.NoError(t, suite.admin.NewRaw("SELECT "+cutoff, now).Scan(context.Background(), &actual))
		require.True(t, now.AddDate(0, 0, -days).Equal(actual), "%s at %s: got %s", name, now, actual)
	}
}
