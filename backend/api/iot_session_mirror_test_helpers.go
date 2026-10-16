//go:build testhooks

package api

import (
	devicescanCompose "github.com/moto-nrw/project-phoenix/modules/devicescan/compose"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	transactionruntime "github.com/moto-nrw/project-phoenix/modules/transactionruntime/compose"
)

// KioskSessionMirror binds the kiosk session mirror to the
// retained timetable rows of the test repositories, the way the root binds
// it to the production ones. The instance rows are returned for read-back.
func (TimetableTestModule) KioskSessionMirror(db transactionruntime.Database, activities timetableCompose.ActivityService) (devicescanCompose.SessionMirror, devicescanCompose.MirrorInstances, error) {
	repos, err := NewTimetableTestRepositories(db)
	if err != nil {
		return nil, nil, err
	}
	return devicescanCompose.NewSessionMirror(repos.ActivityInstance, repos.InstanceStaff, activities, nil, nil), repos.ActivityInstance, nil
}
