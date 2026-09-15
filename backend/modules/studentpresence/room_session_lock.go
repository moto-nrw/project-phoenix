package studentpresence

import "context"

// LockRoomSessionWrites serializes session changes for one room until the caller's tenant transaction ends.
func (m *Module) LockRoomSessionWrites(ctx context.Context, roomID int64) error {
	return m.engine.LockRoomSessionWrites(ctx, roomID)
}
