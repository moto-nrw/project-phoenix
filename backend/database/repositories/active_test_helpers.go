package repositories

import (
	"time"

	activeRepo "github.com/moto-nrw/project-phoenix/database/repositories/active"
	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/modules/schoolmembership"
	"github.com/uptrace/bun"
)

type ActiveTestRepositories struct {
	TimetableTestRepositories
	Attendance       activeModels.AttendanceRepository
	SessionStartLock activeModels.SessionStartLocker
	CombinedGroup    activeModels.CombinedGroupRepository
	GroupMapping     activeModels.GroupMappingRepository
	CrossTenant      CrossTenantQuery
}

func NewActiveTestRepositories(db *bun.DB, clocks ...func() time.Time) (ActiveTestRepositories, error) {
	tt, err := NewTimetableTestRepositories(db, clocks...)
	if err != nil {
		return ActiveTestRepositories{}, err
	}
	people, err := NewPeopleDirectory(db)
	if err != nil {
		return ActiveTestRepositories{}, err
	}
	groups, err := NewSchoolStructure(db)
	if err != nil {
		return ActiveTestRepositories{}, err
	}
	membership, err := NewSchoolMembership(db)
	if err != nil {
		return ActiveTestRepositories{}, err
	}
	r := &Factory{db: db, CrossTenant: activeRepo.NewCrossTenantRepository(db),
		CombinedGroup: activeRepo.NewCombinedGroupRepository(db), GroupMapping: activeRepo.NewGroupMappingRepository(db)}
	r.bindStaffProjections(lazyStaffLookup{get: func() schoolmembership.Capability { return membership }})
	r.BindPeopleDirectory(people)
	r.BindSchoolStructure(groups)
	return ActiveTestRepositories{TimetableTestRepositories: tt, Attendance: activeRepo.NewAttendanceRepository(db, clocks...),
		SessionStartLock: activeRepo.NewSessionStartLocker(db), CombinedGroup: r.CombinedGroup, GroupMapping: r.GroupMapping, CrossTenant: r.CrossTenant}, nil
}
