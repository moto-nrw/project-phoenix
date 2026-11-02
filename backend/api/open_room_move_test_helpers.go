//go:build testhooks

package api

import (
	"context"
	"log/slog"

	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
	"github.com/moto-nrw/project-phoenix/roots/owners"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/moto-nrw/project-phoenix/workflows/openroommove"
)

// NewOpenRoomMoveTestRunner binds the production workflow over real owners.
// Its caller is an authorized administrator; HTTP permission tests stay at
// the workflow's route boundary. The returned ID is the actual room session.
func NewOpenRoomMoveTestRunner(db transactionruntime.Database, unit tenant.UnitOfWork) (func(context.Context, []int64, int64) (int64, error), error) {
	rooms, err := facilitiesCompose.NewUnobserved(db.Pool())
	if err != nil {
		return nil, err
	}
	active := NewActiveTestPresence(db, unit).Active
	command, err := newOpenRoomMove(rooms, owners.MustNewUnobservedTimetable(db), active, slog.Default())
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context, children []int64, room int64) (int64, error) {
		result, err := command.MoveToOpenRoom(ctx, openroommove.Move{StudentIDs: children, RoomID: room, Actor: openroommove.Actor{BypassResourceChecks: true}})
		return result.RoomSessionID, err
	}, nil
}
