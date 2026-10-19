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

// GroupTargets is the dynamic-cohort fixture capability of a template.
type GroupTargets interface {
	ReplaceTargets(context.Context, int64, []*activityContract.GroupTarget) error
	FindTargetsByGroupIDs(context.Context, []int64) (map[int64][]*activityContract.GroupTarget, error)
	FindTargetStudentIDs(context.Context, int64) ([]int64, error)
	FindTargetStudentIDsByGroupIDs(context.Context, []int64) (map[int64][]int64, error)
}
