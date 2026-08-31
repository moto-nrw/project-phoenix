package authorize

const (
	databaseStatsAdminWildcard        = "admin:*"
	databaseStatsFullAccess           = "*:*"
	databaseStatsUsersRead            = "users:read"
	databaseStatsUsersList            = "users:list"
	databaseStatsRoomsRead            = "rooms:read"
	databaseStatsRoomsList            = "rooms:list"
	databaseStatsActivitiesRead       = "activities:read"
	databaseStatsActivitiesList       = "activities:list"
	databaseStatsGroupsRead           = "groups:read"
	databaseStatsGroupsList           = "groups:list"
	databaseStatsAuthManage           = "auth:manage"
	databaseStatsIoTRead              = "iot:read"
	databaseStatsIoTManage            = "iot:manage"
	databaseStatsSchedulesRead        = "schedules:read"
	databaseStatsSchedulesList        = "schedules:list"
	databaseStatsGradeTransitionsRead = "grade_transitions:read"
)

type DatabaseStatsCapabilities struct {
	students, teachers, rooms, activities, groups bool
	roles, devices, permissionCatalog             bool
	timetables, gradeTransitions                  bool
}

func NewDatabaseStatsCapabilities(values []string) DatabaseStatsCapabilities {
	has := func(required ...string) bool {
		for _, actual := range values {
			if actual == databaseStatsAdminWildcard || actual == databaseStatsFullAccess {
				return true
			}
			for _, candidate := range required {
				if actual == candidate {
					return true
				}
			}
		}
		return false
	}
	users := has(databaseStatsUsersRead, databaseStatsUsersList)
	return DatabaseStatsCapabilities{
		students: users, teachers: users,
		rooms:             has(databaseStatsRoomsRead, databaseStatsRoomsList),
		activities:        has(databaseStatsActivitiesRead, databaseStatsActivitiesList),
		groups:            has(databaseStatsGroupsRead, databaseStatsGroupsList),
		roles:             has(databaseStatsAuthManage),
		devices:           has(databaseStatsIoTRead, databaseStatsIoTManage),
		permissionCatalog: has(databaseStatsAuthManage),
		timetables:        has(databaseStatsSchedulesRead, databaseStatsSchedulesList),
		gradeTransitions:  has(databaseStatsGradeTransitionsRead),
	}
}

func (c DatabaseStatsCapabilities) ViewStudents() bool          { return c.students }
func (c DatabaseStatsCapabilities) ViewTeachers() bool          { return c.teachers }
func (c DatabaseStatsCapabilities) ViewRooms() bool             { return c.rooms }
func (c DatabaseStatsCapabilities) ViewActivities() bool        { return c.activities }
func (c DatabaseStatsCapabilities) ViewGroups() bool            { return c.groups }
func (c DatabaseStatsCapabilities) ViewRoles() bool             { return c.roles }
func (c DatabaseStatsCapabilities) ViewDevices() bool           { return c.devices }
func (c DatabaseStatsCapabilities) ViewPermissionCatalog() bool { return c.permissionCatalog }
func (c DatabaseStatsCapabilities) ViewTimetables() bool        { return c.timetables }
func (c DatabaseStatsCapabilities) ViewGradeTransitions() bool  { return c.gradeTransitions }
