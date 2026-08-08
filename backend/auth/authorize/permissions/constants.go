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
	ResourceUsers         = "users"
	ResourceActivities    = "activities"
	ResourceRooms         = "rooms"
	ResourceGroups        = "groups"
	ResourceSubstitutions = "substitutions"
	ResourceFeedback      = "feedback"
	ResourceConfig        = "config"
	ResourceAuth          = "auth"
	ResourceIOT           = "iot"
	ResourceSchedules     = "schedules"
	ResourceCalendar      = "calendar"
	ResourceDisplay       = "display"
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

	// UsersCheckin allows marking a student as checked in or out of school
	// via the web UI (distinct from the IoT kiosk flow, which authenticates
	// as a device). Admins match via AdminWildcard; other roles must be
	// granted explicitly.
	UsersCheckin = ResourceUsers + ":checkin"
)

// Activity permissions
const (
	ActivitiesCreate = ResourceActivities + ":" + ActionCreate
	ActivitiesRead   = ResourceActivities + ":" + ActionRead
	ActivitiesUpdate = ResourceActivities + ":" + ActionUpdate
	ActivitiesDelete = ResourceActivities + ":" + ActionDelete
	ActivitiesList   = ResourceActivities + ":" + ActionList

	// ActivitiesManageCategories gates the category Stammdaten endpoints
	// (#2131). It cannot be one of the constants above: migration 1.9.4
	// granted activities:create/update/delete/manage to the plain `user` role,
	// so every Betreuer holds them. Category Stammdaten are school-wide
	// configuration and stay with the OGS-Leitung (admin role, migration
	// 1.15.259).
	ActivitiesManageCategories = ResourceActivities + ":manage_categories"
)

// Room permissions
const (
	RoomsCreate = ResourceRooms + ":" + ActionCreate
	RoomsRead   = ResourceRooms + ":" + ActionRead
	RoomsUpdate = ResourceRooms + ":" + ActionUpdate
	RoomsDelete = ResourceRooms + ":" + ActionDelete
	RoomsList   = ResourceRooms + ":" + ActionList
)

// Group permissions
const (
	GroupsCreate = ResourceGroups + ":" + ActionCreate
	GroupsRead   = ResourceGroups + ":" + ActionRead
	GroupsUpdate = ResourceGroups + ":" + ActionUpdate
	GroupsDelete = ResourceGroups + ":" + ActionDelete
	GroupsList   = ResourceGroups + ":" + ActionList

	// Special group actions
	GroupsAssign = ResourceGroups + ":assign"
)

// Feedback permissions
const (
	FeedbackCreate = ResourceFeedback + ":" + ActionCreate
	FeedbackRead   = ResourceFeedback + ":" + ActionRead
	FeedbackDelete = ResourceFeedback + ":" + ActionDelete
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

// Display permissions (info-point dashboards, issue #1325)
const (
	DisplayRead   = ResourceDisplay + ":" + ActionRead
	DisplayManage = ResourceDisplay + ":" + ActionManage
)

// Auth permissions
const (
	AuthManage = ResourceAuth + ":" + ActionManage
)

const (
	SchedulesCreate = ResourceSchedules + ":" + ActionCreate
	SchedulesRead   = ResourceSchedules + ":" + ActionRead
	SchedulesUpdate = ResourceSchedules + ":" + ActionUpdate
	SchedulesDelete = ResourceSchedules + ":" + ActionDelete
	SchedulesList   = ResourceSchedules + ":" + ActionList
	SchedulesManage = ResourceSchedules + ":" + ActionManage
)

// Calendar permissions
const (
	CalendarOwn    = ResourceCalendar + ":own"    // Personal calendar access and own invitation responses
	CalendarManage = ResourceCalendar + ":manage" // Create and manage appointments/invitations
)

// Substitution permissions
const (
	SubstitutionsCreate = ResourceSubstitutions + ":" + ActionCreate
	SubstitutionsRead   = ResourceSubstitutions + ":" + ActionRead
	SubstitutionsUpdate = ResourceSubstitutions + ":" + ActionUpdate
	SubstitutionsDelete = ResourceSubstitutions + ":" + ActionDelete
)

// Suggestions permissions
const (
	ResourceSuggestions = "suggestions"

	SuggestionsCreate = ResourceSuggestions + ":" + ActionCreate
	SuggestionsRead   = ResourceSuggestions + ":" + ActionRead
	SuggestionsUpdate = ResourceSuggestions + ":" + ActionUpdate
	SuggestionsDelete = ResourceSuggestions + ":" + ActionDelete
	SuggestionsList   = ResourceSuggestions + ":" + ActionList
)

// Visit permissions
const (
	VisitsCreate = "visits:create"
	VisitsRead   = "visits:read"
	VisitsUpdate = "visits:update"
	VisitsDelete = "visits:delete"
	VisitsManage = "visits:manage"
)

// Time Tracking permissions
const (
	ResourceTimeTracking = "time_tracking"

	TimeTrackingOwn    = ResourceTimeTracking + ":own"    // Read + write own sessions
	TimeTrackingManage = ResourceTimeTracking + ":manage" // Read + write any staff's sessions (admin)
)

// Vacation permissions
const (
	ResourceVacation = "vacation"

	VacationApprove = ResourceVacation + ":approve" // Approve or deny vacation requests (admin)
)

// Communications permissions (parent-facing broadcast announcements, #1669)
const (
	ResourceCommunications = "communications"

	// CommunicationsAnnounce is the semantic permission for parent
	// announcements (Elternmitteilungen): creating, editing, publishing,
	// deleting and viewing read/ack stats. NOTE: in v1 the announcement routes
	// are gated on AdminWildcard, not on this permission, because there is no
	// per-target audience scoping yet — a non-admin holder could otherwise
	// broadcast school-wide and reach other authors' announcements (#1669
	// decision "nur Admins"). This constant is reserved for the future
	// delegated/scoped announcer role that will enforce per-target limits.
	CommunicationsAnnounce = ResourceCommunications + ":announce"
)

// Staff permissions (#1423). staff:financial gates the bank & tax section
// of the Stammdaten tab (IBAN, Steuer-ID, SV-Nummer) — deliberately its own
// permission instead of users:update: the directory maintainers are not the
// Träger payroll office. NOTE: admins still match via the admin:* wildcard;
// restricting school admins would need an exact-match check, which is a
// product decision, not taken here.
const (
	ResourceStaff = "staff"

	StaffFinancial = ResourceStaff + ":financial"
)

// Staff document permissions (#1424). staff_documents:health gates the
// AU-Bescheinigung category — health data is an Art. 9 GDPR special
// category, so seeing a sick note requires more than maintaining the staff
// directory. Catalog-only like staff:financial: admins match via the
// admin:* wildcard, other categories map to existing permissions in the
// document service (Lohnabrechnung → staff:financial, the rest →
// users:update).
const (
	ResourceStaffDocuments = "staff_documents"

	StaffDocumentsHealth = ResourceStaffDocuments + ":health"
)

// Student document permissions (#777). student_documents:health gates
// Attest, Impfnachweis and Medikamentenplan (Art. 9 GDPR health data);
// student_documents:legal gates the Sorgerechtsnachweis, which is not Art. 9
// but is at least as damaging in the wrong hands. Catalog-only like
// staff_documents:health: admins match via the admin:* wildcard, and every
// remaining category rides on users:update in the document service.
const (
	ResourceStudentDocuments = "student_documents"

	StudentDocumentsHealth = ResourceStudentDocuments + ":health"
	StudentDocumentsLegal  = ResourceStudentDocuments + ":legal"
)

// Grade Transition permissions (admin only)
const (
	GradeTransitionsRead   = "grade_transitions:read"
	GradeTransitionsCreate = "grade_transitions:create"
	GradeTransitionsUpdate = "grade_transitions:update"
	GradeTransitionsDelete = "grade_transitions:delete"
	GradeTransitionsApply  = "grade_transitions:apply"
)
