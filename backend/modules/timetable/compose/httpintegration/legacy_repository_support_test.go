package httpintegration_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	"github.com/stretchr/testify/require"
)

// The suites in the legacy_*_test.go files pin the retained schedule
// repositories the composition root's factory binds to the Timetable owner
// and to Care Plan. They moved here with the retirement of the legacy SQL
// test providers (#3554).

// listOptions are the retained repositories' list options. The owner's
// compose package names them for its adapters, so this suite reaches them
// without importing the base model package.
type listOptions = timetableCompose.RecurrenceRuleQueryOptions

// newListOptions mirrors the base package's options constructor: an empty
// filter, no pagination, no sorting.
func newListOptions() *listOptions {
	options := &listOptions{}
	options.Filter = fresh(options.Filter)
	return options
}

// sortedBy sets the options' sort order to the single field.
func sortedBy(options *listOptions, field string, descending bool) {
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
	List(context.Context, *listOptions) ([]T, error)
}

type arrivalScheduleQueryRepository interface {
	scheduleModels.StudentArrivalScheduleRepository
	queryListRepository[*scheduleModels.StudentArrivalSchedule]
}

type arrivalExceptionQueryRepository interface {
	scheduleModels.StudentArrivalExceptionRepository
	queryListRepository[*scheduleModels.StudentArrivalException]
}

type arrivalNoteQueryRepository interface {
	scheduleModels.StudentArrivalNoteRepository
	queryListRepository[*scheduleModels.StudentArrivalNote]
}

type pickupScheduleQueryRepository interface {
	scheduleModels.StudentPickupScheduleRepository
	queryListRepository[*scheduleModels.StudentPickupSchedule]
}

type pickupExceptionQueryRepository interface {
	scheduleModels.StudentPickupExceptionRepository
	queryListRepository[*scheduleModels.StudentPickupException]
}

type pickupNoteQueryRepository interface {
	scheduleModels.StudentPickupNoteRepository
	queryListRepository[*scheduleModels.StudentPickupNote]
}

type instanceStaffQueryRepository interface {
	scheduleModels.InstanceStaffRepository
	queryListRepository[*scheduleModels.InstanceStaff]
}

type activityExceptionQueryRepository interface {
	scheduleModels.ActivityExceptionRepository
	queryListRepository[*scheduleModels.ActivityException]
}

type dateframeQueryRepository interface {
	scheduleModels.DateframeRepository
	queryListRepository[*scheduleModels.Dateframe]
}

type recurrenceRuleQueryRepository interface {
	scheduleModels.RecurrenceRuleRepository
	queryListRepository[*scheduleModels.RecurrenceRule]
}

type timeframeQueryRepository interface {
	scheduleModels.TimeframeRepository
	queryListRepository[*scheduleModels.Timeframe]
}
