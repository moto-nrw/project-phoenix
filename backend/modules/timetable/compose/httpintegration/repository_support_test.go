package httpintegration_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	fixtures "github.com/moto-nrw/project-phoenix/modules/timetable/fixturetest"

	"github.com/stretchr/testify/require"
)

// The suites in the legacy_*_test.go files pin the retained schedule
// repositories the composition root's factory binds to the Timetable owner
// and to Care Plan. They moved here with the retirement of the legacy SQL
// test providers (#3554).

// newListOptions mirrors the base package's options constructor for the
// retained repositories' list options: an empty filter, no pagination, no
// sorting.
func newListOptions() *fixtures.QueryOptions {
	options := &fixtures.QueryOptions{}
	options.Filter = fresh(options.Filter)
	return options
}

// sortedBy sets the options' sort order to the single field.
func sortedBy(options *fixtures.QueryOptions, field string, descending bool) {
	options.Sorting = fresh(options.Sorting)
	if descending {
		options.Sorting.AddField(field, "DESC")
		return
	}
	options.Sorting.AddField(field, "ASC")
}

// fresh returns a new zero value of the pointer's element type.
func fresh[T any](*T) *T { return new(T) }

// requireDatabaseError asserts that err carries the retained repositories'
// database error for operation.
func requireDatabaseError(t *testing.T, err error, operation string) {
	t.Helper()
	require.Error(t, err)
	prefix := "database error during " + operation
	for current := err; current != nil; current = errors.Unwrap(current) {
		if message := current.Error(); message == prefix || strings.HasPrefix(message, prefix+": ") {
			return
		}
	}
	t.Fatalf("want the database error of %q, got %v", operation, err)
}

type queryListRepository[T any] interface {
	List(context.Context, *fixtures.QueryOptions) ([]T, error)
}

type arrivalScheduleQueryRepository interface {
	fixtures.
		StudentArrivalScheduleRecords
	queryListRepository[*fixtures.StudentArrivalSchedule]
}

type arrivalExceptionQueryRepository interface {
	fixtures.
		StudentArrivalExceptionRecords
	queryListRepository[*fixtures.StudentArrivalException]
}

type arrivalNoteQueryRepository interface {
	fixtures.
		StudentArrivalNoteRecords
	queryListRepository[*fixtures.StudentArrivalNote]
}

type pickupScheduleQueryRepository interface {
	fixtures.
		StudentPickupScheduleRecords
	queryListRepository[*fixtures.StudentPickupSchedule]
}

type pickupExceptionQueryRepository interface {
	fixtures.
		StudentPickupExceptionRecords
	queryListRepository[*fixtures.StudentPickupException]
}

type pickupNoteQueryRepository interface {
	fixtures.
		StudentPickupNoteRecords
	queryListRepository[*fixtures.StudentPickupNote]
}

type instanceStaffQueryRepository interface {
	fixtures.
		InstanceStaffRecords
	queryListRepository[*fixtures.InstanceStaff]
}

type activityExceptionQueryRepository interface {
	fixtures.
		ActivityExceptionRecords
	queryListRepository[*fixtures.ActivityException]
}

type dateframeQueryRepository interface {
	fixtures.
		DateframeRecords
	queryListRepository[*fixtures.Dateframe]
}

type timeframeQueryRepository interface {
	fixtures.
		TimeframeRecords
	queryListRepository[*fixtures.Timeframe]
}
