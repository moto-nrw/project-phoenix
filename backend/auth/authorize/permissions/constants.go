package permissions

// Standard permission action types
const (
	ActionCreate = "create"
	ActionRead   = "read"
	ActionUpdate = "update"
	ActionDelete = "delete"
	ActionList   = "list"
	ActionManage = "manage" // Full control over resource
)

// Resource types
const (
	ResourceUsers      = "users"
	ResourceActivities = "activities"
	ResourceRooms      = "rooms"
	ResourceGroups     = "groups"
	ResourceFeedback   = "feedback"
	ResourceConfig     = "config"
	ResourceAuth       = "auth"
	ResourceIOT        = "iot"
)

// Admin permissions
const (
	AdminWildcard = "admin:*" // Grants all permissions
	FullAccess    = "*:*"     // Alias for full system access
)

// User permissions
const (
	UsersCreate = ResourceUsers + ":" + ActionCreate
	UsersRead   = ResourceUsers + ":" + ActionRead
	UsersUpdate = ResourceUsers + ":" + ActionUpdate
	UsersDelete = ResourceUsers + ":" + ActionDelete
	UsersList   = ResourceUsers + ":" + ActionList
	UsersManage = ResourceUsers + ":" + ActionManage
)

// Activity permissions
const (
	ActivitiesCreate = ResourceActivities + ":" + ActionCreate
	ActivitiesRead   = ResourceActivities + ":" + ActionRead
	ActivitiesUpdate = ResourceActivities + ":" + ActionUpdate
	ActivitiesDelete = ResourceActivities + ":" + ActionDelete
	ActivitiesList   = ResourceActivities + ":" + ActionList
	ActivitiesManage = ResourceActivities + ":" + ActionManage

	// Special activity actions
	ActivitiesEnroll = ResourceActivities + ":enroll"
	ActivitiesAssign = ResourceActivities + ":assign"
)

// Room permissions
const (
	RoomsCreate = ResourceRooms + ":" + ActionCreate
	RoomsRead   = ResourceRooms + ":" + ActionRead
	RoomsUpdate = ResourceRooms + ":" + ActionUpdate
	RoomsDelete = ResourceRooms + ":" + ActionDelete
	RoomsList   = ResourceRooms + ":" + ActionList
	RoomsManage = ResourceRooms + ":" + ActionManage
)

// Group permissions
const (
	GroupsCreate = ResourceGroups + ":" + ActionCreate
	GroupsRead   = ResourceGroups + ":" + ActionRead
	GroupsUpdate = ResourceGroups + ":" + ActionUpdate
	GroupsDelete = ResourceGroups + ":" + ActionDelete
	GroupsList   = ResourceGroups + ":" + ActionList
	GroupsManage = ResourceGroups + ":" + ActionManage

	// Special group actions
	GroupsAssign = ResourceGroups + ":assign"
)

// Feedback permissions
const (
	FeedbackCreate = ResourceFeedback + ":" + ActionCreate
	FeedbackRead   = ResourceFeedback + ":" + ActionRead
	FeedbackDelete = ResourceFeedback + ":" + ActionDelete
	FeedbackList   = ResourceFeedback + ":" + ActionList
	FeedbackManage = ResourceFeedback + ":" + ActionManage
)

// Config permissions
const (
	ConfigRead   = ResourceConfig + ":" + ActionRead
	ConfigUpdate = ResourceConfig + ":" + ActionUpdate
	ConfigManage = ResourceConfig + ":" + ActionManage
)

// IOT permissions
const (
	IOTRead   = ResourceIOT + ":" + ActionRead
	IOTUpdate = ResourceIOT + ":" + ActionUpdate
	IOTManage = ResourceIOT + ":" + ActionManage
)

// Auth permissions
const (
	AuthManage = ResourceAuth + ":" + ActionManage
)
