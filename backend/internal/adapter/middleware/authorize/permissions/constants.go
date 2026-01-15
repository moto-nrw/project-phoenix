package permissions

import coreperm "github.com/moto-nrw/project-phoenix/internal/core/port/permissions"

// Standard permission action types
const (
	ActionCreate = coreperm.ActionCreate
	ActionRead   = coreperm.ActionRead
	ActionUpdate = coreperm.ActionUpdate
	ActionDelete = coreperm.ActionDelete
	ActionList   = coreperm.ActionList
	ActionManage = coreperm.ActionManage
)

// Resource types
const (
	ResourceUsers         = coreperm.ResourceUsers
	ResourceActivities    = coreperm.ResourceActivities
	ResourceRooms         = coreperm.ResourceRooms
	ResourceGroups        = coreperm.ResourceGroups
	ResourceSubstitutions = coreperm.ResourceSubstitutions
	ResourceFeedback      = coreperm.ResourceFeedback
	ResourceConfig        = coreperm.ResourceConfig
	ResourceAuth          = coreperm.ResourceAuth
	ResourceIOT           = coreperm.ResourceIOT
	ResourceSchedules     = coreperm.ResourceSchedules
)

// Admin permissions
const (
	AdminWildcard = coreperm.AdminWildcard
	FullAccess    = coreperm.FullAccess
)

// User permissions
const (
	UsersCreate = coreperm.UsersCreate
	UsersRead   = coreperm.UsersRead
	UsersUpdate = coreperm.UsersUpdate
	UsersDelete = coreperm.UsersDelete
	UsersList   = coreperm.UsersList
	UsersManage = coreperm.UsersManage
)

// Activity permissions
const (
	ActivitiesCreate = coreperm.ActivitiesCreate
	ActivitiesRead   = coreperm.ActivitiesRead
	ActivitiesUpdate = coreperm.ActivitiesUpdate
	ActivitiesDelete = coreperm.ActivitiesDelete
	ActivitiesList   = coreperm.ActivitiesList
	ActivitiesManage = coreperm.ActivitiesManage
	ActivitiesEnroll = coreperm.ActivitiesEnroll
	ActivitiesAssign = coreperm.ActivitiesAssign
)

// Room permissions
const (
	RoomsCreate = coreperm.RoomsCreate
	RoomsRead   = coreperm.RoomsRead
	RoomsUpdate = coreperm.RoomsUpdate
	RoomsDelete = coreperm.RoomsDelete
	RoomsList   = coreperm.RoomsList
	RoomsManage = coreperm.RoomsManage
)

// Group permissions
const (
	GroupsCreate = coreperm.GroupsCreate
	GroupsRead   = coreperm.GroupsRead
	GroupsUpdate = coreperm.GroupsUpdate
	GroupsDelete = coreperm.GroupsDelete
	GroupsList   = coreperm.GroupsList
	GroupsManage = coreperm.GroupsManage
	GroupsAssign = coreperm.GroupsAssign
)

// Feedback permissions
const (
	FeedbackCreate = coreperm.FeedbackCreate
	FeedbackRead   = coreperm.FeedbackRead
	FeedbackDelete = coreperm.FeedbackDelete
	FeedbackList   = coreperm.FeedbackList
	FeedbackManage = coreperm.FeedbackManage
)

// Config permissions
const (
	ConfigRead   = coreperm.ConfigRead
	ConfigUpdate = coreperm.ConfigUpdate
	ConfigManage = coreperm.ConfigManage
)

// IOT permissions
const (
	IOTRead   = coreperm.IOTRead
	IOTUpdate = coreperm.IOTUpdate
	IOTManage = coreperm.IOTManage
)

// Auth permissions
const (
	AuthManage = coreperm.AuthManage
)

// Schedule permissions
const (
	SchedulesCreate = coreperm.SchedulesCreate
	SchedulesRead   = coreperm.SchedulesRead
	SchedulesUpdate = coreperm.SchedulesUpdate
	SchedulesDelete = coreperm.SchedulesDelete
	SchedulesList   = coreperm.SchedulesList
	SchedulesManage = coreperm.SchedulesManage
)

// Substitution permissions
const (
	SubstitutionsCreate = coreperm.SubstitutionsCreate
	SubstitutionsRead   = coreperm.SubstitutionsRead
	SubstitutionsUpdate = coreperm.SubstitutionsUpdate
	SubstitutionsDelete = coreperm.SubstitutionsDelete
	SubstitutionsList   = coreperm.SubstitutionsList
	SubstitutionsManage = coreperm.SubstitutionsManage
)

// Visit permissions
const (
	VisitsCreate = coreperm.VisitsCreate
	VisitsRead   = coreperm.VisitsRead
	VisitsUpdate = coreperm.VisitsUpdate
	VisitsDelete = coreperm.VisitsDelete
	VisitsList   = coreperm.VisitsList
	VisitsManage = coreperm.VisitsManage
)
