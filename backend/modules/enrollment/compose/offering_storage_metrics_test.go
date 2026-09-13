package compose

import (
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/require"
)

type storageMetricDatabaseError string

func (e storageMetricDatabaseError) Error() string { return "database failure" }
func (e storageMetricDatabaseError) Field(field byte) string {
	if field == 'C' {
		return string(e)
	}
	return ""
}

func TestOfferingStorageMetricsClassifyImmutableConflictAndWrappedDeadlock(t *testing.T) {
	t.Parallel()
	observe := observeOfferingStorage()
	for _, scenario := range []struct {
		operation, code, outcome string
		err                      error
	}{
		{"metric_success", "none", "success", nil},
		{"metric_conflict", "immutable_conflict", "error", enrollment.ErrSubmittedOfferingChoiceConflict},
		{"metric_deadlock", "deadlock", "error", fmt.Errorf("record: %w", storageMetricDatabaseError("40P01"))},
	} {
		counter := offeringStorageOperations.WithLabelValues(scenario.operation, "command", scenario.outcome, scenario.code)
		before := testutil.ToFloat64(counter)
		observe(enrollment.OfferingStorageObservation{Operation: scenario.operation, Kind: "command", Duration: time.Millisecond, InputRows: 2, OutputRows: -1, Err: scenario.err})
		require.Equal(t, before+1, testutil.ToFloat64(counter))
	}
}
