package services

import (
	"github.com/moto-nrw/project-phoenix/database/repositories"
	devicescanCompose "github.com/moto-nrw/project-phoenix/modules/devicescan/compose"
	activitiesSvc "github.com/moto-nrw/project-phoenix/services/activities"
	"github.com/uptrace/bun"
)

// KioskSessionMirror binds the kiosk session mirror to the
// retained timetable rows of the test repositories, the way the root binds
// it to the production ones. The instance rows are returned for read-back.
func (TimetableTestModule) KioskSessionMirror(db *bun.DB, activities activitiesSvc.ActivityService) (devicescanCompose.SessionMirror, devicescanCompose.MirrorInstances, error) {
	repos, err := repositories.NewTimetableTestRepositories(db)
	if err != nil {
		return nil, nil, err
	}
	return devicescanCompose.NewSessionMirror(repos.ActivityInstance, repos.InstanceStaff, activities, nil, nil), repos.ActivityInstance, nil
}
