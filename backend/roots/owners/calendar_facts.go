package owners

import (
	"context"
	"encoding/json"
	"fmt"

	appointmentcap "github.com/moto-nrw/project-phoenix/modules/appointments"
	carePlanBindingPorts "github.com/moto-nrw/project-phoenix/modules/careplan/ports"
	facilitiesNative "github.com/moto-nrw/project-phoenix/modules/facilities"
	facilitiesCompose "github.com/moto-nrw/project-phoenix/modules/facilities/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	calendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal/compose"
	educationRepo "github.com/moto-nrw/project-phoenix/modules/schoolstructure/compose"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/modules/timetable"
	timetableCompose "github.com/moto-nrw/project-phoenix/modules/timetable/compose"
	timetablePorts "github.com/moto-nrw/project-phoenix/modules/timetable/ports"
)

// NewCalendarFacts binds the calendar's directory fact ports to the
// retained staff, student, guardian, relationship, child and group rows; a
// nil source leaves its port unset. BindCalendarScheduleFacts,
// BindCalendarFeedFacts and BindCalendarCaller bind the other ports into
// the result.
func NewCalendarFacts(
	staff calendarStaffRows, students calendarStudentRows, guardians calendarCompose.GuardianDirectory,
	relationships calendarRelationshipRows, children ParentChildLinkSource, groups EducationGroupRows,
) calendarCompose.Config {
	cfg := calendarCompose.Config{}
	if staff != nil {
		cfg.StaffRepo = calendarStaffPort{source: staff, calendarSchoolPortalStaff: staff}
	}
	if students != nil {
		cfg.StudentRepo = calendarStudentPort{source: students}
	}
	if guardians != nil {
		cfg.GuardianProfileRepo = guardians
	}
	if relationships != nil {
		cfg.StudentGuardianRepo = calendarRelationshipPort{source: relationships}
	}
	if children != nil {
		cfg.ChildRepo = calendarChildPort{source: children}
	}
	if groups != nil {
		cfg.GroupRepo = calendarGroupPort{source: groups}
	}
	return cfg
}

// BindCalendarScheduleFacts binds the calendar's timetable, room and shift
// fact ports into cfg: the instance staff and activity instance rows, the
// rooms and the staff shifts and shift types. A nil source leaves its port
// as it is.
func BindCalendarScheduleFacts(
	cfg calendarCompose.Config, assignments timetable.InstanceStaffQuery, instances timetablePorts.LegacyInstanceReads,
	rooms facilitiesNative.RoomRows, shifts timetablePorts.CalendarShiftRows, shiftTypes timetablePorts.CalendarShiftTypeRows,
) calendarCompose.Config {
	if assignments != nil {
		cfg.InstanceStaffRepo = calendarAssignmentPort{source: assignments}
	}
	if instances != nil {
		cfg.ActivityInstanceRepo = calendarInstancePort{source: instances}
	}
	if rooms != nil {
		cfg.RoomRepo = calendarRoomPort{source: rooms}
	}
	if shifts != nil {
		cfg.StaffShiftRepo = calendarShiftPort{source: shifts}
	}
	if shiftTypes != nil {
		cfg.ShiftTypeRepo = calendarShiftTypePort{source: shiftTypes}
	}
	return cfg
}

// BindCalendarFeedFacts binds the subscription feeds' fact ports into cfg:
// the schools, the parent and staff feed accounts and the persons behind
// them. A nil source leaves its port as it is.
func BindCalendarFeedFacts(
	cfg calendarCompose.Config, schools organizationtenancy.Query, accounts identityaccess.ParentCalendarFeeds,
	staffFeeds identityaccess.StaffCalendarFeeds, persons calendarPersonRows,
) calendarCompose.Config {
	if schools != nil {
		cfg.SchoolRepo = calendarSchoolPort{source: schools}
	}
	if accounts != nil {
		cfg.AccountRepo = calendarAccountPort{ParentCalendarFeeds: accounts}
	}
	if staffFeeds != nil {
		cfg.StaffFeedRepo = calendarStaffFeedPort{StaffCalendarFeeds: staffFeeds}
	}
	if persons != nil {
		cfg.PersonRepo = calendarPersonPort{source: persons}
	}
	return cfg
}

// calendarStaffPort maps the retained staff rows into the calendar's terms;
// the school-portal staff IDs need no mapping and are promoted as they are.
type calendarStaffPort struct {
	source calendarStaffRows
	calendarSchoolPortalStaff
}

type calendarSchoolPortalStaff interface {
	FindSchoolPortalStaffIDs(ctx context.Context) (map[int64]bool, error)
}

// calendarStaffRows are the retained staff reads of the calendar.
type calendarStaffRows interface {
	ListAllWithPerson(ctx context.Context) ([]*peopledirectory.Staff, error)
	FindWithPersonByIDs(ctx context.Context, ids []int64) (map[int64]*peopledirectory.Staff, error)
	FindReachableCalendarStaffIDs(ctx context.Context, ids []int64) (map[int64]bool, error)
	calendarSchoolPortalStaff
	FindByPersonID(ctx context.Context, personID int64) (*peopledirectory.Staff, error)
}

func (p calendarStaffPort) ListAllWithPerson(ctx context.Context) ([]*calendarCompose.Staff, error) {
	value, err := p.source.ListAllWithPerson(ctx)
	return CalendarMapSlice(value, calendarStaff), err
}

func (p calendarStaffPort) FindWithPersonByIDs(ctx context.Context, ids []int64) (map[int64]*calendarCompose.Staff, error) {
	value, err := p.source.FindWithPersonByIDs(ctx, ids)
	return calendarMapByID(value, calendarStaff), err
}

func (p calendarStaffPort) FindReachableCalendarStaffIDs(ctx context.Context, ids []int64) (map[int64]bool, error) {
	value, err := p.source.FindReachableCalendarStaffIDs(ctx, ids)
	return value, err
}

func (p calendarStaffPort) FindByPersonID(ctx context.Context, id int64) (*calendarCompose.Staff, error) {
	value, err := p.source.FindByPersonID(ctx, id)
	return calendarStaff(value), err
}

type calendarStudentPort struct{ source calendarStudentRows }

// calendarStudentRows are the retained student reads of the calendar.
type calendarStudentRows interface {
	List(ctx context.Context, filters map[string]any) ([]*peopledirectory.StudentState, error)
	FindByGroupIDs(ctx context.Context, groupIDs []int64) ([]*peopledirectory.StudentState, error)
	ListSchoolClasses(ctx context.Context) ([]string, error)
	FindAllWithGroups(ctx context.Context) ([]*peopledirectory.StudentRosterEntry, error)
}

func (p calendarStudentPort) ListActive(ctx context.Context) ([]*calendarCompose.Student, error) {
	value, err := p.source.List(ctx, map[string]any{"status": string(peopledirectory.StudentStatusActive)})
	return CalendarMapSlice(value, calendarStudent), err
}

func (p calendarStudentPort) FindByGroupIDs(ctx context.Context, ids []int64) ([]*calendarCompose.Student, error) {
	value, err := p.source.FindByGroupIDs(ctx, ids)
	return CalendarMapSlice(value, calendarStudent), err
}

func (p calendarStudentPort) ListSchoolClasses(ctx context.Context) ([]string, error) {
	value, err := p.source.ListSchoolClasses(ctx)
	return value, err
}

func (p calendarStudentPort) FindAllWithGroups(ctx context.Context) ([]*calendarCompose.Student, error) {
	value, err := p.source.FindAllWithGroups(ctx)
	return CalendarMapSlice(value, func(row *peopledirectory.StudentRosterEntry) *calendarCompose.Student {
		return calendarStudent(row.Record)
	}), err
}

type calendarGuardianPort struct {
	source   calendarGuardianProfiles
	contacts peopledirectory.GuardianPortalContacts
}

// calendarGuardianProfiles are the guardian profile reads of the calendar's
// recipient search.
type calendarGuardianProfiles interface {
	SearchByText(ctx context.Context, searchText string, limit int) ([]*peopledirectory.GuardianProfileSnapshot, error)
	FindByIDs(ctx context.Context, ids []int64) (map[int64]*peopledirectory.GuardianProfileSnapshot, error)
	FindActivePortalProfilesByIDs(ctx context.Context, ids []int64) (map[int64]*peopledirectory.GuardianProfileSnapshot, error)
}

// NewCalendarGuardianDirectory binds the existing profile reads and the native
// batched portal projection without constructing a second directory module.
func NewCalendarGuardianDirectory(source calendarGuardianProfiles, contacts peopledirectory.GuardianPortalContacts) calendarCompose.GuardianDirectory {
	return calendarGuardianPort{source: source, contacts: contacts}
}

func (p calendarGuardianPort) ListPortalContacts(ctx context.Context, guardianIDs, studentIDs []int64) ([]calendarCompose.GuardianContact, error) {
	values, err := p.contacts.ListGuardianPortalContacts(ctx, guardianIDs, studentIDs)
	if err != nil {
		return nil, err
	}
	result := make([]calendarCompose.GuardianContact, 0, len(values))
	for _, value := range values {
		contact := calendarCompose.GuardianContact{Profile: calendarCompose.GuardianProfile{
			ID: value.GuardianProfileID, AccountID: &value.AccountID, Email: value.Email,
			FirstName: value.FirstName, LastName: value.LastName, PortalLocale: value.PortalLocale,
		}}
		if value.StudentID != nil {
			portalAccess, permissionErr := calendarPortalPermission(value.PortalPermission)
			if permissionErr != nil {
				return nil, permissionErr
			}
			contact.Relationship = &calendarCompose.StudentGuardian{
				GuardianProfileID: value.GuardianProfileID, StudentID: *value.StudentID,
				PortalAccess: portalAccess,
			}
		}
		result = append(result, contact)
	}
	return result, nil
}

// Preserve the shared helper's interpretation of historical JSON values.
func calendarPortalPermission(raw json.RawMessage) (bool, error) {
	var value any
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &value); err != nil {
			return false, fmt.Errorf("calendar guardian portal permission: %w", err)
		}
	}
	return securityruntime.GuardianPermissionGranted(value), nil
}

func (p calendarGuardianPort) SearchByText(ctx context.Context, query string, limit int) ([]*calendarCompose.GuardianProfile, error) {
	value, err := p.source.SearchByText(ctx, query, limit)
	return CalendarMapSlice(value, calendarGuardian), err
}

func (p calendarGuardianPort) FindByIDs(ctx context.Context, ids []int64) (map[int64]*calendarCompose.GuardianProfile, error) {
	value, err := p.source.FindByIDs(ctx, ids)
	return calendarMapByID(value, calendarGuardian), err
}

func (p calendarGuardianPort) FindActivePortalProfilesByIDs(ctx context.Context, ids []int64) (map[int64]*calendarCompose.GuardianProfile, error) {
	value, err := p.source.FindActivePortalProfilesByIDs(ctx, ids)
	return calendarMapByID(value, calendarGuardian), err
}

type calendarRelationshipPort struct {
	source calendarRelationshipRows
}

// calendarRelationshipRows and calendarPersonRows are the guardian links
// and the persons the calendar reads.
type (
	calendarRelationshipRows interface {
		FindByGuardianProfileID(ctx context.Context, guardianProfileID int64) ([]*peopledirectory.GuardianRelationshipSnapshot, error)
	}
	calendarPersonRows interface {
		FindByAccountID(ctx context.Context, accountID int64) (*peopledirectory.DirectoryPersonSnapshot, error)
	}
)

func (p calendarRelationshipPort) FindByGuardianProfileID(ctx context.Context, id int64) ([]*calendarCompose.StudentGuardian, error) {
	value, err := p.source.FindByGuardianProfileID(ctx, id)
	return CalendarMapSlice(value, calendarRelationship), err
}

type calendarPersonPort struct {
	source calendarPersonRows
}

func (p calendarPersonPort) FindByAccountID(ctx context.Context, id int64) (*calendarCompose.Person, error) {
	value, err := p.source.FindByAccountID(ctx, id)
	return calendarPerson(value), err
}

type calendarSchoolPort struct {
	source organizationtenancy.Query
}

func (p calendarSchoolPort) FindByID(ctx context.Context, id int64) (*calendarCompose.School, error) {
	value, err := p.source.FindSchool(ctx, id)
	if err != nil {
		return nil, err
	}
	return &calendarCompose.School{Name: value.Name}, nil
}

type calendarChildPort struct {
	source ParentChildLinkSource
}

func (p calendarChildPort) ListByAccount(ctx context.Context, id int64) ([]*calendarCompose.ChildSummary, error) {
	value, err := p.source.ListByAccount(ctx, id)
	return CalendarMapSlice(value, func(v carePlanBindingPorts.ParentChildLink) *calendarCompose.ChildSummary {
		return &calendarCompose.ChildSummary{StudentID: v.StudentID, GuardianProfileID: v.GuardianProfileID, TenantID: v.TenantID, SchoolName: v.SchoolName}
	}), err
}

type calendarGroupPort struct {
	source interface {
		List(ctx context.Context, filters map[string]any) ([]*educationRepo.Group, error)
	}
}

func (p calendarGroupPort) List(ctx context.Context) ([]*calendarCompose.Group, error) {
	value, err := p.source.List(ctx, nil)
	return CalendarMapSlice(value, func(v *educationRepo.Group) *calendarCompose.Group {
		return &calendarCompose.Group{ID: v.ID, Name: v.Name}
	}), err
}

type calendarRoomPort struct {
	source facilitiesNative.RoomRows
}

func (p calendarRoomPort) FindByIDs(ctx context.Context, ids []int64) ([]*calendarCompose.Room, error) {
	value, err := facilitiesCompose.FindRoomNames(ctx, p.source, ids)
	return CalendarMapSlice(value, func(v *facilitiesNative.RoomName) *calendarCompose.Room {
		if v == nil {
			return nil
		}
		return &calendarCompose.Room{ID: v.ID, Name: v.Name}
	}), err
}

type calendarAssignmentPort struct {
	source timetable.InstanceStaffQuery
}

func (p calendarAssignmentPort) FindByStaffAndDateRange(ctx context.Context, id int64, from, to appointmentcap.Date) ([]*calendarCompose.InstanceStaff, error) {
	value, err := timetableCompose.FindCalendarAssignments(ctx, p.source, id, string(from), string(to))
	return CalendarMapSlice(value, func(v timetablePorts.CalendarAssignment) *calendarCompose.InstanceStaff {
		return &calendarCompose.InstanceStaff{InstanceID: v.InstanceID, RoomID: v.RoomID, UpdatedAt: v.UpdatedAt}
	}), err
}

type calendarInstancePort struct {
	source timetablePorts.LegacyInstanceReads
}

func (p calendarInstancePort) FindByIDs(ctx context.Context, ids []int64) ([]*calendarCompose.ActivityInstance, error) {
	value, err := timetableCompose.FindCalendarInstances(ctx, p.source, ids)
	return CalendarMapSlice(value, func(v timetablePorts.CalendarInstance) *calendarCompose.ActivityInstance {
		return &calendarCompose.ActivityInstance{ID: v.ID, RoomID: v.RoomID, Title: v.Title, Status: v.Status, Description: v.Description, Date: appointmentcap.Date(v.Date), StartTime: v.StartTime, EndTime: v.EndTime, UpdatedAt: v.UpdatedAt, ActivityType: v.TemplateType}
	}), err
}

type calendarShiftPort struct {
	source timetablePorts.CalendarShiftRows
}

func (p calendarShiftPort) FindByStaffAndDateRange(ctx context.Context, id int64, from, to appointmentcap.Date) ([]*calendarCompose.StaffShift, error) {
	value, err := timetableCompose.FindCalendarShifts(ctx, p.source, id, string(from), string(to))
	return CalendarMapSlice(value, func(v timetablePorts.CalendarShift) *calendarCompose.StaffShift {
		return &calendarCompose.StaffShift{ID: v.ID, Date: appointmentcap.Date(v.Date), StartTime: v.StartTime, EndTime: v.EndTime, UpdatedAt: v.UpdatedAt, ShiftTypeID: v.ShiftTypeID, Cancelled: v.Cancelled, Notes: v.Notes}
	}), err
}

type calendarShiftTypePort struct {
	source timetablePorts.CalendarShiftTypeRows
}

func (p calendarShiftTypePort) ListAll(ctx context.Context) ([]*calendarCompose.ShiftType, error) {
	value, err := timetableCompose.ListCalendarShiftTypes(ctx, p.source)
	return CalendarMapSlice(value, func(v timetablePorts.CalendarShiftType) *calendarCompose.ShiftType {
		return &calendarCompose.ShiftType{ID: v.ID, Name: v.Name}
	}), err
}

func calendarPerson(value *peopledirectory.DirectoryPersonSnapshot) *calendarCompose.Person {
	if value == nil {
		return nil
	}
	return &calendarCompose.Person{ID: value.ID, FirstName: value.FirstName, LastName: value.LastName}
}

func calendarStaff(value *peopledirectory.Staff) *calendarCompose.Staff {
	if value == nil {
		return nil
	}
	return &calendarCompose.Staff{ID: value.ID, Person: calendarPerson(value.Person)}
}

func calendarStudent(value *peopledirectory.StudentState) *calendarCompose.Student {
	if value == nil {
		return nil
	}
	return &calendarCompose.Student{ID: value.ID, Person: calendarPerson(value.Person), Status: string(value.Status), SchoolClass: value.SchoolClass, GroupID: value.GroupID}
}

func calendarGuardian(value *peopledirectory.GuardianProfileSnapshot) *calendarCompose.GuardianProfile {
	if value == nil {
		return nil
	}
	return &calendarCompose.GuardianProfile{ID: value.ID, AccountID: value.AccountID, Email: value.Email, FirstName: value.FirstName, LastName: value.LastName, PortalLocale: value.PortalLocale}
}

func calendarRelationship(value *peopledirectory.GuardianRelationshipSnapshot) *calendarCompose.StudentGuardian {
	if value == nil {
		return nil
	}
	return &calendarCompose.StudentGuardian{GuardianProfileID: value.GuardianProfileID, StudentID: value.StudentID, PortalAccess: securityruntime.GuardianPermissionGrantedJSON(value.Permissions, peopledirectory.GuardianPermissionPortalAccess)}
}

func calendarAccount(value identityaccess.ParentCalendarFeedAccount) *calendarCompose.Account {
	return &calendarCompose.Account{ID: value.ID, Email: value.Email, Active: value.Active, CalendarFeedToken: &value.TokenHash}
}

func CalendarMapSlice[A, B any](values []A, convert func(A) B) []B {
	if values == nil {
		return nil
	}
	result := make([]B, 0, len(values))
	for _, value := range values {
		result = append(result, convert(value))
	}
	return result
}

func calendarMapByID[A, B any](values map[int64]A, convert func(A) B) map[int64]B {
	if values == nil {
		return nil
	}
	result := make(map[int64]B, len(values))
	for id, value := range values {
		result[id] = convert(value)
	}
	return result
}

type calendarAccountPort struct {
	identityaccess.ParentCalendarFeeds
}

func (p calendarAccountPort) FindByID(ctx context.Context, id int64) (*calendarCompose.Account, error) {
	value, found, err := p.FindParentCalendarFeedAccount(ctx, id)
	if !found || err != nil {
		return nil, err
	}
	return calendarAccount(value), err
}

func (p calendarAccountPort) FindByCalendarFeedToken(ctx context.Context, hash string) (*calendarCompose.Account, error) {
	value, found, err := p.FindParentCalendarFeedOwner(ctx, hash)
	if !found || err != nil {
		return nil, err
	}
	return calendarAccount(value), err
}

func (p calendarAccountPort) EnsureCalendarFeedToken(ctx context.Context, id int64, hash string) (string, error) {
	return p.EnsureParentCalendarFeedToken(ctx, id, hash)
}

func (p calendarAccountPort) SetCalendarFeedToken(ctx context.Context, id int64, hash string) error {
	return p.RotateParentCalendarFeedToken(ctx, id, hash)
}

type calendarStaffFeedPort struct {
	identityaccess.StaffCalendarFeeds
}

func (p calendarStaffFeedPort) FindOwnerByTokenHash(ctx context.Context, hash string) (*calendarCompose.FeedOwner, error) {
	value, found, err := p.FindStaffCalendarFeedOwner(ctx, hash)
	if !found || err != nil {
		return nil, err
	}
	return &calendarCompose.FeedOwner{AccountID: value.AccountID, TenantID: value.TenantID}, err
}

func (p calendarStaffFeedPort) EnsureToken(ctx context.Context, accountID, tenantID int64, hash string) (string, error) {
	return p.EnsureStaffCalendarFeedToken(ctx, accountID, tenantID, hash)
}

func (p calendarStaffFeedPort) RotateToken(ctx context.Context, accountID, tenantID int64, hash string) (bool, error) {
	return p.RotateStaffCalendarFeedToken(ctx, accountID, tenantID, hash)
}
