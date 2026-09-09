package repositories

import (
	"context"

	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	educationModels "github.com/moto-nrw/project-phoenix/models/education"
	facilitiesModels "github.com/moto-nrw/project-phoenix/models/facilities"
	parentModels "github.com/moto-nrw/project-phoenix/models/parent"
	platformModels "github.com/moto-nrw/project-phoenix/models/platform"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	appointmentcap "github.com/moto-nrw/project-phoenix/modules/appointments"
	calendarCompose "github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal/compose"
)

type CalendarFacts struct {
	StaffRepo            userModels.StaffRepository
	StudentRepo          userModels.StudentRepository
	GuardianProfileRepo  userModels.GuardianProfileRepository
	StudentGuardianRepo  userModels.StudentGuardianRepository
	ChildRepo            parentModels.ChildRepository
	GroupRepo            educationModels.GroupRepository
	InstanceStaffRepo    scheduleModels.InstanceStaffRepository
	ActivityInstanceRepo scheduleModels.ActivityInstanceRepository
	RoomRepo             facilitiesModels.RoomRepository
	StaffShiftRepo       scheduleModels.StaffShiftRepository
	ShiftTypeRepo        scheduleModels.ShiftTypeRepository
	SchoolRepo           platformModels.SchoolRepository
	AccountRepo          CalendarFeedAccounts
	StaffFeedRepo        authModels.StaffCalendarFeedTokenRepository
	PersonRepo           userModels.PersonRepository
}
type CalendarFeedAccounts interface {
	FindByID(context.Context, any) (*authModels.Account, error)
	FindByCalendarFeedToken(context.Context, string) (*authModels.Account, error)
	SetCalendarFeedToken(context.Context, int64, string) error
	EnsureCalendarFeedToken(context.Context, int64, string) (string, error)
}

func NewCalendarFacts(d CalendarFacts) calendarCompose.Config {
	cfg := calendarCompose.Config{}
	if d.StaffRepo != nil {
		cfg.StaffRepo = calendarStaffPort{d.StaffRepo}
	}
	if d.StudentRepo != nil {
		cfg.StudentRepo = calendarStudentPort{d.StudentRepo}
	}
	if d.GuardianProfileRepo != nil {
		cfg.GuardianProfileRepo = calendarGuardianPort{d.GuardianProfileRepo}
	}
	if d.StudentGuardianRepo != nil {
		cfg.StudentGuardianRepo = calendarRelationshipPort{d.StudentGuardianRepo}
	}
	if d.ChildRepo != nil {
		cfg.ChildRepo = calendarChildPort{d.ChildRepo}
	}
	if d.GroupRepo != nil {
		cfg.GroupRepo = calendarGroupPort{d.GroupRepo}
	}
	if d.InstanceStaffRepo != nil {
		cfg.InstanceStaffRepo = calendarAssignmentPort{d.InstanceStaffRepo}
	}
	if d.ActivityInstanceRepo != nil {
		cfg.ActivityInstanceRepo = calendarInstancePort{d.ActivityInstanceRepo}
	}
	if d.RoomRepo != nil {
		cfg.RoomRepo = calendarRoomPort{d.RoomRepo}
	}
	if d.StaffShiftRepo != nil {
		cfg.StaffShiftRepo = calendarShiftPort{d.StaffShiftRepo}
	}
	if d.ShiftTypeRepo != nil {
		cfg.ShiftTypeRepo = calendarShiftTypePort{d.ShiftTypeRepo}
	}
	if d.SchoolRepo != nil {
		cfg.SchoolRepo = calendarSchoolPort{d.SchoolRepo}
	}
	if d.AccountRepo != nil {
		cfg.AccountRepo = calendarAccountPort{d.AccountRepo}
	}
	if d.StaffFeedRepo != nil {
		cfg.StaffFeedRepo = calendarStaffFeedPort{d.StaffFeedRepo}
	}
	if d.PersonRepo != nil {
		cfg.PersonRepo = calendarPersonPort{d.PersonRepo}
	}
	return cfg
}

type calendarStaffPort struct{ source userModels.StaffRepository }

func (p calendarStaffPort) ListAllWithPerson(ctx context.Context) ([]*calendarCompose.Staff, error) {
	value, err := p.source.ListAllWithPerson(ctx)
	return calendarMapSlice(value, calendarStaff), err
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

type calendarStudentPort struct{ source userModels.StudentRepository }

func (p calendarStudentPort) ListActive(ctx context.Context) ([]*calendarCompose.Student, error) {
	value, err := p.source.List(ctx, map[string]any{"status": string(userModels.StudentStatusActive)})
	return calendarMapSlice(value, calendarStudent), err
}
func (p calendarStudentPort) FindByGroupIDs(ctx context.Context, ids []int64) ([]*calendarCompose.Student, error) {
	value, err := p.source.FindByGroupIDs(ctx, ids)
	return calendarMapSlice(value, calendarStudent), err
}
func (p calendarStudentPort) ListSchoolClasses(ctx context.Context) ([]string, error) {
	value, err := p.source.ListSchoolClasses(ctx)
	return value, err
}
func (p calendarStudentPort) FindAllWithGroups(ctx context.Context) ([]*calendarCompose.Student, error) {
	value, err := p.source.FindAllWithGroups(ctx)
	return calendarMapSlice(value, func(row *userModels.StudentWithGroupInfo) *calendarCompose.Student {
		return calendarStudent(row.Student)
	}), err
}

type calendarGuardianPort struct {
	source userModels.GuardianProfileRepository
}

func (p calendarGuardianPort) SearchByText(ctx context.Context, query string, limit int) ([]*calendarCompose.GuardianProfile, error) {
	value, err := p.source.SearchByText(ctx, query, limit)
	return calendarMapSlice(value, calendarGuardian), err
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
	source userModels.StudentGuardianRepository
}

func (p calendarRelationshipPort) FindByGuardianProfileIDs(ctx context.Context, ids []int64) ([]*calendarCompose.StudentGuardian, error) {
	value, err := p.source.FindByGuardianProfileIDs(ctx, ids)
	return calendarMapSlice(value, calendarRelationship), err
}
func (p calendarRelationshipPort) FindByStudentIDs(ctx context.Context, ids []int64) ([]*calendarCompose.StudentGuardian, error) {
	value, err := p.source.FindByStudentIDs(ctx, ids)
	return calendarMapSlice(value, calendarRelationship), err
}
func (p calendarRelationshipPort) FindByGuardianProfileID(ctx context.Context, id int64) ([]*calendarCompose.StudentGuardian, error) {
	value, err := p.source.FindByGuardianProfileID(ctx, id)
	return calendarMapSlice(value, calendarRelationship), err
}

type calendarPersonPort struct{ source userModels.PersonRepository }

func (p calendarPersonPort) FindByAccountID(ctx context.Context, id int64) (*calendarCompose.Person, error) {
	value, err := p.source.FindByAccountID(ctx, id)
	return calendarPerson(value), err
}

type calendarSchoolPort struct {
	source platformModels.SchoolRepository
}

func (p calendarSchoolPort) FindByID(ctx context.Context, id int64) (*calendarCompose.School, error) {
	value, err := p.source.FindByID(ctx, id)
	return func() *calendarCompose.School {
		if value == nil {
			return nil
		}
		return &calendarCompose.School{Name: value.Name}
	}(), err
}

type calendarChildPort struct{ source parentModels.ChildRepository }

func (p calendarChildPort) ListByAccount(ctx context.Context, id int64) ([]*calendarCompose.ChildSummary, error) {
	value, err := p.source.ListByAccount(ctx, id)
	return calendarMapSlice(value, func(v *parentModels.ChildSummary) *calendarCompose.ChildSummary {
		return &calendarCompose.ChildSummary{StudentID: v.StudentID, GuardianProfileID: v.GuardianProfileID, TenantID: v.TenantID, SchoolName: v.SchoolName}
	}), err
}

type calendarGroupPort struct {
	source educationModels.GroupRepository
}

func (p calendarGroupPort) List(ctx context.Context) ([]*calendarCompose.Group, error) {
	value, err := p.source.List(ctx, nil)
	return calendarMapSlice(value, func(v *educationModels.Group) *calendarCompose.Group {
		return &calendarCompose.Group{ID: v.ID, Name: v.Name}
	}), err
}

type calendarRoomPort struct {
	source facilitiesModels.RoomRepository
}

func (p calendarRoomPort) FindByIDs(ctx context.Context, ids []int64) ([]*calendarCompose.Room, error) {
	value, err := p.source.FindByIDs(ctx, ids)
	return calendarMapSlice(value, func(v *facilitiesModels.Room) *calendarCompose.Room {
		if v == nil {
			return nil
		}
		return &calendarCompose.Room{ID: v.ID, Name: v.Name}
	}), err
}

type calendarAssignmentPort struct {
	source scheduleModels.InstanceStaffRepository
}

func (p calendarAssignmentPort) FindByStaffAndDateRange(ctx context.Context, id int64, from, to appointmentcap.Date) ([]*calendarCompose.InstanceStaff, error) {
	value, err := p.source.FindByStaffAndDateRange(ctx, id, scheduleModels.Date(from), scheduleModels.Date(to))
	return calendarMapSlice(value, func(v *scheduleModels.InstanceStaff) *calendarCompose.InstanceStaff {
		return &calendarCompose.InstanceStaff{InstanceID: v.InstanceID, RoomID: v.RoomID, UpdatedAt: v.UpdatedAt}
	}), err
}

type calendarInstancePort struct {
	source scheduleModels.ActivityInstanceRepository
}

func (p calendarInstancePort) FindByIDs(ctx context.Context, ids []int64) ([]*calendarCompose.ActivityInstance, error) {
	value, err := p.source.FindByIDs(ctx, ids)
	return calendarMapSlice(value, func(v *scheduleModels.ActivityInstance) *calendarCompose.ActivityInstance {
		return &calendarCompose.ActivityInstance{ID: v.ID, RoomID: v.RoomID, Title: v.Title, Status: v.Status, Description: v.Description, Date: appointmentcap.Date(v.Date), StartTime: v.StartTime, EndTime: v.EndTime, UpdatedAt: v.UpdatedAt}
	}), err
}

type calendarShiftPort struct {
	source scheduleModels.StaffShiftRepository
}

func (p calendarShiftPort) FindByStaffAndDateRange(ctx context.Context, id int64, from, to appointmentcap.Date) ([]*calendarCompose.StaffShift, error) {
	value, err := p.source.FindByStaffAndDateRange(ctx, id, scheduleModels.Date(from), scheduleModels.Date(to))
	return calendarMapSlice(value, func(v *scheduleModels.StaffShift) *calendarCompose.StaffShift {
		return &calendarCompose.StaffShift{ID: v.ID, Date: appointmentcap.Date(v.Date), StartTime: v.StartTime, EndTime: v.EndTime, UpdatedAt: v.UpdatedAt, ShiftTypeID: v.ShiftTypeID, Cancelled: v.Cancelled, Notes: v.Notes}
	}), err
}

type calendarShiftTypePort struct {
	source scheduleModels.ShiftTypeRepository
}

func (p calendarShiftTypePort) ListAll(ctx context.Context) ([]*calendarCompose.ShiftType, error) {
	value, err := p.source.ListAll(ctx)
	return calendarMapSlice(value, func(v *scheduleModels.ShiftType) *calendarCompose.ShiftType {
		return &calendarCompose.ShiftType{ID: v.ID, Name: v.Name}
	}), err
}
func calendarPerson(value *userModels.Person) *calendarCompose.Person {
	if value == nil {
		return nil
	}
	return &calendarCompose.Person{ID: value.ID, FirstName: value.FirstName, LastName: value.LastName}
}
func calendarStaff(value *userModels.Staff) *calendarCompose.Staff {
	if value == nil {
		return nil
	}
	return &calendarCompose.Staff{ID: value.ID, Person: calendarPerson(value.Person)}
}
func calendarStudent(value *userModels.Student) *calendarCompose.Student {
	if value == nil {
		return nil
	}
	return &calendarCompose.Student{ID: value.ID, Person: calendarPerson(value.Person), Status: string(value.Status), SchoolClass: value.SchoolClass, GroupID: value.GroupID}
}
func calendarGuardian(value *userModels.GuardianProfile) *calendarCompose.GuardianProfile {
	if value == nil {
		return nil
	}
	return &calendarCompose.GuardianProfile{ID: value.ID, AccountID: value.AccountID, Email: value.Email, FirstName: value.FirstName, LastName: value.LastName, PortalLocale: value.PortalLocale}
}
func calendarRelationship(value *userModels.StudentGuardian) *calendarCompose.StudentGuardian {
	if value == nil {
		return nil
	}
	return &calendarCompose.StudentGuardian{GuardianProfileID: value.GuardianProfileID, StudentID: value.StudentID, PortalAccess: usersRepo.GuardianPortalAccess(value)}
}
func calendarAccount(value *authModels.Account) *calendarCompose.Account {
	if value == nil {
		return nil
	}
	return &calendarCompose.Account{ID: value.ID, Email: value.Email, Active: value.IsActive(), CalendarFeedToken: value.CalendarFeedToken}
}
func calendarMapSlice[A, B any](values []A, convert func(A) B) []B {
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

type calendarAccountPort struct{ CalendarFeedAccounts }

func (p calendarAccountPort) FindByID(ctx context.Context, id int64) (*calendarCompose.Account, error) {
	value, err := p.CalendarFeedAccounts.FindByID(ctx, id)
	return calendarAccount(value), err
}
func (p calendarAccountPort) FindByCalendarFeedToken(ctx context.Context, hash string) (*calendarCompose.Account, error) {
	value, err := p.CalendarFeedAccounts.FindByCalendarFeedToken(ctx, hash)
	return calendarAccount(value), err
}

type calendarStaffFeedPort struct {
	authModels.StaffCalendarFeedTokenRepository
}

func (p calendarStaffFeedPort) FindOwnerByTokenHash(ctx context.Context, hash string) (*calendarCompose.FeedOwner, error) {
	value, err := p.StaffCalendarFeedTokenRepository.FindOwnerByTokenHash(ctx, hash)
	if value == nil {
		return nil, err
	}
	return &calendarCompose.FeedOwner{AccountID: value.AccountID, TenantID: value.TenantID}, err
}
