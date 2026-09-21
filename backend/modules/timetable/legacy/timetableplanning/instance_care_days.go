package timetableplanning

import (
	"context"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
)

// InstanceCareDays combines the plan query and lock protocol required by roster
// rewrites. The supplied capabilities remain owned by Care Plan.
type InstanceCareDays interface {
	careplan.CareDayQuery
	LockStudentAndExceptionDay(context.Context, int64, string) error
}

type InstanceCareDayLocker interface {
	LockStudentAndExceptionDay(context.Context, int64, string) error
}

type instanceCareDays struct {
	careplan.CareDayQuery
	InstanceCareDayLocker
}

// NewInstanceCareDays binds existing capabilities without constructing a graph.
func NewInstanceCareDays(queries careplan.CareDayQuery, locks InstanceCareDayLocker) InstanceCareDays {
	if queries == nil || locks == nil {
		panic("instance care days: queries and locks are required")
	}
	return instanceCareDays{queries, locks}
}
