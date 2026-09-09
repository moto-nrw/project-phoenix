package ports

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/schoolcalendar/portal"
)

// Directory facts are the fields the calendar consumes, not persistence rows.
type Person struct {
	ID                  int64
	FirstName, LastName string
}
type Staff struct {
	ID     int64
	Person *Person
}
type Student struct {
	ID                  int64
	Person              *Person
	Status, SchoolClass string
	GroupID             *int64
}
type GuardianProfile struct {
	ID                  int64
	AccountID           *int64
	Email               *string
	FirstName, LastName string
	PortalLocale        *string
}
type StudentGuardian struct {
	GuardianProfileID, StudentID int64
	PortalAccess                 bool
}
type ChildSummary struct {
	StudentID, GuardianProfileID, TenantID int64
	SchoolName                             string
}
type Group struct {
	ID   int64
	Name string
}
type School struct{ Name string }
type Account struct {
	ID                int64
	Email             string
	Active            bool
	CalendarFeedToken *string
}

func (a *Account) IsActive() bool { return a.Active }

type FeedOwner struct{ AccountID, TenantID int64 }
type Room struct {
	ID   int64
	Name string
}
type InstanceStaff struct {
	InstanceID int64
	RoomID     *int64
	UpdatedAt  time.Time
}
type ActivityInstance struct {
	ID, RoomID                    int64
	Title, Status                 string
	Description                   *string
	Date                          portal.Date
	StartTime, EndTime, UpdatedAt time.Time
}
type StaffShift struct {
	ID                            int64
	Date                          portal.Date
	StartTime, EndTime, UpdatedAt time.Time
	ShiftTypeID                   *int64
	Cancelled                     bool
	Notes                         string
}
type ShiftType struct {
	ID   int64
	Name string
}

type StaffDirectory interface {
	ListAllWithPerson(context.Context) ([]*Staff, error)
	FindReachableCalendarStaffIDs(context.Context, []int64) (map[int64]bool, error)
	FindWithPersonByIDs(context.Context, []int64) (map[int64]*Staff, error)
	FindByPersonID(context.Context, int64) (*Staff, error)
}
type StudentDirectory interface {
	ListActive(context.Context) ([]*Student, error)
	FindByGroupIDs(context.Context, []int64) ([]*Student, error)
	ListSchoolClasses(context.Context) ([]string, error)
	FindAllWithGroups(context.Context) ([]*Student, error)
}
type GuardianDirectory interface {
	SearchByText(context.Context, string, int) ([]*GuardianProfile, error)
	FindActivePortalProfilesByIDs(context.Context, []int64) (map[int64]*GuardianProfile, error)
	FindByIDs(context.Context, []int64) (map[int64]*GuardianProfile, error)
}
type GuardianRelationships interface {
	FindByGuardianProfileIDs(context.Context, []int64) ([]*StudentGuardian, error)
	FindByStudentIDs(context.Context, []int64) ([]*StudentGuardian, error)
	FindByGuardianProfileID(context.Context, int64) ([]*StudentGuardian, error)
}
type Children interface {
	ListByAccount(context.Context, int64) ([]*ChildSummary, error)
}
type Groups interface {
	List(context.Context) ([]*Group, error)
}
type Schools interface {
	FindByID(context.Context, int64) (*School, error)
}
type Persons interface {
	FindByAccountID(context.Context, int64) (*Person, error)
}
type Identity interface {
	GetCurrentStaff(context.Context) (*Staff, error)
	GetCurrentUser(context.Context) (*Account, error)
	MissingUser(error) bool
	MissingStaff(error) bool
}
type Accounts interface {
	FindByID(context.Context, int64) (*Account, error)
	FindByCalendarFeedToken(context.Context, string) (*Account, error)
	SetCalendarFeedToken(context.Context, int64, string) error
	EnsureCalendarFeedToken(context.Context, int64, string) (string, error)
}
type StaffFeeds interface {
	FindOwnerByTokenHash(context.Context, string) (*FeedOwner, error)
	EnsureToken(context.Context, int64, int64, string) (string, error)
	RotateToken(context.Context, int64, int64, string) (bool, error)
}
type Rooms interface {
	FindByIDs(context.Context, []int64) ([]*Room, error)
}
type Assignments interface {
	FindByStaffAndDateRange(context.Context, int64, portal.Date, portal.Date) ([]*InstanceStaff, error)
}
type Instances interface {
	FindByIDs(context.Context, []int64) ([]*ActivityInstance, error)
}
type Shifts interface {
	FindByStaffAndDateRange(context.Context, int64, portal.Date, portal.Date) ([]*StaffShift, error)
}
type ShiftTypes interface {
	ListAll(context.Context) ([]*ShiftType, error)
}

// Runtime keeps transaction ownership in composition. Tenant operations reuse
// the ambient transaction, including rollback and after-commit semantics.
type Runtime interface {
	TenantID(context.Context) int64
	WithinAppointmentWrite(context.Context, func(context.Context) error) error
	WithinTenant(context.Context, int64, func(context.Context) error) error
	WithinAdmin(context.Context, func(context.Context) error) error
	AfterCommit(context.Context, func())
	WithoutTransactionAndHooks(context.Context) context.Context
}
