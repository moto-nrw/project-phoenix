//go:build testhooks

package api

import (
	"context"
	"time"

	devicescanCompose "github.com/moto-nrw/project-phoenix/modules/devicescan/compose"
	deviceScan "github.com/moto-nrw/project-phoenix/modules/devicescan/ports"
	activityContract "github.com/moto-nrw/project-phoenix/modules/timetable/activitycontract"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// KioskSessionMirror binds the kiosk session mirror to the
// retained timetable rows of the test repositories, the way the root binds
// it to the production ones. The instance rows are returned for read-back.
func (TimetableTestModule) KioskSessionMirror(db transactionruntime.Database, activities activityContract.ActivityService) (devicescanCompose.SessionMirror, timetablePorts.NativeInstanceRecords, error) {
	repos, err := NewTimetableTestRepositories(db)
	if err != nil {
		return nil, nil, err
	}
	return NewKioskSessionMirror(repos.Repositories, activities, nil, nil), repos.NativeInstances(), nil
}

// mirrorKioskSessionAt supplies a captured session instant without changing
// the process clock or exposing a consumer port to the root's test role.
func mirrorKioskSessionAt(ctx context.Context, mirror devicescanCompose.SessionMirror, groupID, roomID int64, at time.Time) {
	mirror.MirrorSession(ctx, deviceScan.LifecycleSession{ID: groupID, RoomID: roomID, StartTime: at}, nil)
}
