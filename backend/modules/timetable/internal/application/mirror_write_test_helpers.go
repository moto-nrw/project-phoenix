//go:build testhooks

package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

func WithMirrorAfterWrite(rows *ports.MirrorParticipantRecords,

	after func(string, bool, error) error) *ports.MirrorParticipantRecords {
	decorated := *rows
	callbacks := *rows.MirrorParticipants
	decorated.MirrorParticipants = &callbacks
	decorated.UpdateAttendanceFromCheckin = func(ctx context.Context, instanceID, studentID int64, at time.Time) (bool, error) {
		changed, err := rows.UpdateAttendanceFromCheckin(ctx, instanceID, studentID, at)
		return changed, after("check-in", changed, err)
	}
	decorated.CreateUnplannedPresentIfAbsent = func(ctx context.Context, instanceID, studentID int64, at time.Time) (*timetable.ScheduledParticipant, error) {
		row, err := rows.CreateUnplannedPresentIfAbsent(ctx, instanceID, studentID, at)
		return row, after("insert", row != nil, err)
	}
	decorated.UpdateAttendanceCheckout = func(ctx context.Context, instanceID, studentID int64, at time.Time) error {
		return after("checkout", false, rows.UpdateAttendanceCheckout(ctx, instanceID, studentID, at))
	}
	decorated.UpdateAttendanceFromCheckinBatch = func(ctx context.Context, keys []timetable.InstanceStudentKey, at time.Time) error {
		return after("check-in", false, rows.UpdateAttendanceFromCheckinBatch(ctx, keys, at))
	}
	decorated.UpdateAttendanceCheckoutBatch = func(ctx context.Context, keys []timetable.InstanceStudentKey, at time.Time) error {
		return after("checkout", false, rows.UpdateAttendanceCheckoutBatch(ctx, keys, at))
	}
	decorated.ReconcileAttendanceInterval = func(ctx context.Context, instanceID, studentID int64, previousIn time.Time, previousOut *time.Time, updatedIn time.Time, updatedOut *time.Time) (bool, error) {
		changed, err := rows.ReconcileAttendanceInterval(ctx, instanceID, studentID, previousIn, previousOut, updatedIn, updatedOut)
		return changed, after("revision", changed, err)
	}
	return &decorated
}
