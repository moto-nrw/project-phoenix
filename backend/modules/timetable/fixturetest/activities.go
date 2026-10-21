//go:build testhooks

package fixturetest

import (
	"context"

	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
)

// EnrollmentWriter creates the retained protected roster rows exercised by integration suites.
type EnrollmentWriter interface {
	Create(context.Context, *activityContract.StudentEnrollment) error
}
