package ports

import (
	"context"
)

// SettingsSnapshot is an opaque handle on one school's settings, resolved
// ahead of a tick. Only the source that resolved it can bind it.
type SettingsSnapshot any

// SettingsSnapshotSource is the batch read a SettingsResolver may also
// offer. The root binds the Settings Platform's cross-tenant read, so one
// query per minute serves every school; a school without a snapshot in the
// result fails its tick.
type SettingsSnapshotSource interface {
	ResolveSettingsSnapshots(ctx context.Context, tenantIDs []int64, keys []string) (map[int64]SettingsSnapshot, error)
	// BindSettingsSnapshot returns a context whose settings reads the
	// snapshot serves, including the reads of the owners a job calls. A
	// snapshot it did not resolve is an error, never a silent database read.
	BindSettingsSnapshot(ctx context.Context, snapshot SettingsSnapshot) (context.Context, error)
}
