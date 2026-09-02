package config

import (
	"context"
	"errors"
	"time"

	"github.com/uptrace/bun"
)

// Runtime supplies ambient transaction and tenant state without coupling the
// settings Postgres adapter to the transaction implementation.
type Runtime interface {
	DB(context.Context) bun.IDB
	TenantID(context.Context) int64
	LockStaffBalance(context.Context, int64) error
	TodayTime() time.Time
	// AssignedStaffIDs returns the live staff members assigned to a
	// work-time template. The staff rows belong to School Membership, so the
	// runtime resolves them through that capability instead of letting this
	// package join users.staff (#2667).
	AssignedStaffIDs(ctx context.Context, workTimeModelID int64) ([]int64, error)
	// RebaseAssignedStaffAnchor stamps the template's rotation anchor onto
	// every live staff member assigned to it and returns their IDs.
	// anchorDate is a calendar date in YYYY-MM-DD format.
	RebaseAssignedStaffAnchor(ctx context.Context, workTimeModelID int64, anchorDate string) ([]int64, error)
}

type directRuntime struct{ db *bun.DB }

// NewRuntime provides the non-transactional runtime used by composition
// callers that only perform privileged reads. Application services replace it
// with their tenant-aware runtime before wiring settings repositories.
func NewRuntime(db *bun.DB) Runtime { return directRuntime{db: db} }

func (r directRuntime) DB(context.Context) bun.IDB   { return r.db }
func (directRuntime) TenantID(context.Context) int64 { return 0 }
func (directRuntime) TodayTime() time.Time           { return time.Now() }
func (directRuntime) LockStaffBalance(context.Context, int64) error {
	return errMembershipRuntimeRequired
}

// The bootstrap runtime carries no School Membership capability: staff
// assignments are only ever read and rebased through the tenant-aware
// runtime the service graph installs.
func (directRuntime) AssignedStaffIDs(context.Context, int64) ([]int64, error) {
	return nil, errMembershipRuntimeRequired
}

func (directRuntime) RebaseAssignedStaffAnchor(context.Context, int64, string) ([]int64, error) {
	return nil, errMembershipRuntimeRequired
}

var errMembershipRuntimeRequired = errors.New("config repository transaction runtime is required")
