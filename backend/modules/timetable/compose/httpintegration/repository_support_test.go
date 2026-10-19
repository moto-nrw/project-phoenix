package httpintegration_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"

	"github.com/moto-nrw/project-phoenix/modules/timetable/internal/application"
	scheduleModels "github.com/moto-nrw/project-phoenix/modules/timetable/internal/domain/schedulerecords"
	"github.com/stretchr/testify/require"
)

// The suites in the legacy_*_test.go files pin the retained schedule
// repositories the composition root's factory binds to the Timetable owner
// and to Care Plan. They moved here with the retirement of the legacy SQL
// test providers (#3554).

// newListOptions mirrors the base package's options constructor for the
// retained repositories' list options: an empty filter, no pagination, no
// sorting.
func newListOptions() *application.RecurrenceRuleQueryOptions {
	options := &application.RecurrenceRuleQueryOptions{}
	options.Filter = fresh(options.Filter)
	return options
}

// sortedBy sets the options' sort order to the single field.
func sortedBy(options *application.RecurrenceRuleQueryOptions, field string, descending bool) {
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

// isNotFound mirrors the base package's not-found check: the repository
// not-found sentinel, bare or wrapped.
func isNotFound(err error) bool {
	var marker interface{ RepositoryNotFound() }
	return errors.As(err, &marker)
}

type queryListRepository[T any] interface {
	List(context.Context, *application.RecurrenceRuleQueryOptions) ([]T, error)
}

type arrivalScheduleQueryRepository interface {
	ports.StudentArrivalScheduleRecords
	queryListRepository[*scheduleModels.StudentArrivalSchedule]
}

type arrivalExceptionQueryRepository interface {
	ports.StudentArrivalExceptionRecords
	queryListRepository[*scheduleModels.StudentArrivalException]
}

type arrivalNoteQueryRepository interface {
	ports.StudentArrivalNoteRecords
	queryListRepository[*scheduleModels.StudentArrivalNote]
}

type pickupScheduleQueryRepository interface {
	ports.StudentPickupScheduleRecords
	queryListRepository[*scheduleModels.StudentPickupSchedule]
}

type pickupExceptionQueryRepository interface {
	ports.StudentPickupExceptionRecords
	queryListRepository[*scheduleModels.StudentPickupException]
}

type pickupNoteQueryRepository interface {
	ports.StudentPickupNoteRecords
	queryListRepository[*scheduleModels.StudentPickupNote]
}

type instanceStaffQueryRepository interface {
	ports.InstanceStaffRecords
	queryListRepository[*scheduleModels.InstanceStaff]
}

type activityExceptionQueryRepository interface {
	ports.ActivityExceptionRecords
	queryListRepository[*scheduleModels.ActivityException]
}

type dateframeQueryRepository interface {
	ports.DateframeRecords
	queryListRepository[*scheduleModels.Dateframe]
}

type recurrenceRuleQueryRepository interface {
	ports.RecurrenceRuleRecords
	queryListRepository[*scheduleModels.RecurrenceRule]
}

type timeframeQueryRepository interface {
	ports.TimeframeRecords
	queryListRepository[*scheduleModels.Timeframe]
}
