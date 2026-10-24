// Package mealplan is the public Meal Plan capability.
package mealplan

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type Date string

func ParseDate(value string) (Date, error) {
	date, err := calendar.ParseDate(value)
	if err != nil {
		return "", err
	}
	return Date(date.String()), nil
}

func (d Date) German() (string, error) {
	date, err := calendar.ParseDate(string(d))
	if err != nil {
		return "", err
	}
	return date.Format("02.01.2006"), nil
}

type Dish struct {
	Dish      string
	Note      *string
	Allergens []Allergen
}

type Entry struct {
	Date      Date
	Position  int
	Dish      string
	Note      *string
	Allergens []Allergen
}

type Weekday int

const (
	Monday Weekday = iota + 1
	Tuesday
	Wednesday
	Thursday
	Friday
)

type ParticipationSource string

const (
	ParticipationNone     ParticipationSource = "none"
	ParticipationRegular  ParticipationSource = "regular"
	ParticipationOverride ParticipationSource = "override"
	ParticipationSick     ParticipationSource = "sick"
	// ParticipationAbsent: excused or on a class trip. The reason stays
	// with the school.
	ParticipationAbsent ParticipationSource = "absent"
	// ParticipationClosed: a closing day or public holiday; nobody eats.
	ParticipationClosed ParticipationSource = "closed"
)

// RegistrationModel is how a school's parents register for lunch (#3873).
type RegistrationModel string

const (
	RegistrationModelSingle RegistrationModel = "ongoing_single"
	RegistrationModelSplit  RegistrationModel = "ongoing_split"
	RegistrationModelAnnual RegistrationModel = "annual"
)

// ParticipationDay is one meal day of a child. Changeable is true while the
// parent may still register or cancel it; RegisterUntil and CancelUntil are
// the deadlines of the day.
type ParticipationDay struct {
	Date          Date
	Participating bool
	// Regular is what the weekly pattern alone gives on the day: false in
	// school holidays and on closed days (#3873).
	Regular       bool
	Source        ParticipationSource
	Changeable    bool
	CanRegister   bool
	CanCancel     bool
	RegisterUntil time.Time
	CancelUntil   time.Time
}

type ParticipationPlan struct {
	Weekdays           []Weekday
	EffectiveFrom      Date
	CutoffTime         string
	Model              RegistrationModel
	ScheduleChangeable bool
	// SickCancels is true when an absence reported after the cancellation
	// deadline still cancels the meal.
	SickCancels bool
	Days        []ParticipationDay
}

type ReplaceParticipationSchedule struct {
	StudentID         int64
	GuardianAccountID int64
	Weekdays          []Weekday
}

// SetParticipationWeek registers or cancels the remaining weekdays of the
// week of WeekOf.
type SetParticipationWeek struct {
	StudentID         int64
	GuardianAccountID int64
	WeekOf            Date
	Participating     bool
}

// WeekChange reports which days a week change changed and which it left
// because their deadline had passed.
type WeekChange struct {
	Changed []Date
	Locked  []Date
}

type SetParticipationDay struct {
	StudentID         int64
	GuardianAccountID int64
	Date              Date
	Participating     bool
}

// SchoolChange is a change of a child's registration by the school (#3873).
// The school changes at any time, deadlines included, but never a past or
// closed day.
type SchoolChange struct {
	StudentID      int64
	StaffAccountID int64
	Date           Date
	Participating  bool
}

// SchoolSchedule replaces a child's regular days from EffectiveFrom on, at
// the earliest today. The school uses it after the annual registration.
type SchoolSchedule struct {
	StudentID      int64
	StaffAccountID int64
	Weekdays       []Weekday
	EffectiveFrom  Date
}

type DailyParticipant struct {
	StudentID   int64
	FirstName   string
	LastName    string
	SchoolClass string
}

// DailyList is the kitchen's list of a day. Deadline is when the day's
// order is fixed (#3873); CutoffTime is its Berlin clock time.
type DailyList struct {
	Date       Date
	CutoffTime string
	Deadline   time.Time
	// Closed is a closing day or public holiday: nobody eats.
	Closed       bool
	Participants []DailyParticipant
	// Others are the enrolled children who do not eat that day.
	Others []DailyParticipant
}

type ReplaceDay struct {
	Date   Date
	Dishes []Dish
}

// Fault is a refused Meal Plan request. Code names it; Detail, when set, is
// the client text the response carries instead of the code.
type Fault struct {
	Code   string
	Detail string
}

func (f *Fault) Error() string { return f.Code }

var (
	ErrDisabled             = &Fault{Code: "meal_plan_disabled", Detail: "feature_disabled"}
	ErrInvalidMealDate      = &Fault{Code: "invalid_meal_date", Detail: "meal plan covers weekdays only (Monday-Friday)"}
	ErrInvalidDishes        = &Fault{Code: "invalid_dishes"}
	ErrInvalidAllergens     = &Fault{Code: "invalid_allergens", Detail: "allergens must be codes of the 14 main allergens"}
	ErrRegistrationDisabled = &Fault{Code: "meal_registration_disabled", Detail: "meal_registration_disabled"}
	ErrInvalidParticipation = &Fault{Code: "invalid_meal_participation"}
	ErrParticipationCutoff  = &Fault{Code: "meal_participation_cutoff_passed", Detail: "meal_participation_cutoff_passed"}
	// ErrPastDay refuses the school a change of a day already over (#3873).
	ErrPastDay = &Fault{Code: "meal_day_past", Detail: "the day is already over"}
	// ErrDayClosed refuses a change of a day the school is closed.
	ErrDayClosed = &Fault{Code: "meal_day_closed", Detail: "the school is closed that day"}
	// ErrStudentNotFound: the school does not know the child.
	ErrStudentNotFound = &Fault{Code: "meal_student_not_found", Detail: "student not found"}
	// ErrChildNotEnrolled: the child is not in care on the day.
	ErrChildNotEnrolled = &Fault{Code: "meal_child_not_enrolled", Detail: "the child is not in care that day"}
)

// Capability is the whole Meal Plan: the menu, the children's registration
// and the school's orders.
type Capability interface {
	Menu
	Registration
	Ordering
}

// Menu is the school's weekly meal plan.
type Menu interface {
	Available(context.Context) (bool, error)
	Week(context.Context, Date) ([]Entry, error)
	ReplaceDay(context.Context, ReplaceDay) error
	ClearDay(context.Context, Date) error
}

// Registration is the children's meal registration, by their parents and by
// the school (#3873).
type Registration interface {
	RegistrationAvailable(context.Context) (bool, error)
	Participation(context.Context, int64, Date, Date) (ParticipationPlan, error)
	ParticipationBySchool(context.Context, int64, Date, Date) (ParticipationPlan, error)
	ReplaceParticipationSchedule(context.Context, ReplaceParticipationSchedule) (Date, error)
	ReplaceScheduleBySchool(context.Context, SchoolSchedule) (Date, error)
	SetParticipationForDay(context.Context, SetParticipationDay) error
	SetParticipationForWeek(context.Context, SetParticipationWeek) (WeekChange, error)
	ClearParticipationForDay(context.Context, SetParticipationDay) error
	SetParticipationBySchool(context.Context, SchoolChange) error
	ClearParticipationBySchool(context.Context, SchoolChange) error
	DailyList(context.Context, Date) (DailyList, error)
	WeeklyList(context.Context, Date) (WeeklyList, error)
}

// Ordering is the school's meal orders and what its caterer reads of them
// (ADR 0077, #3873). Who supplies the school is no Meal Plan data: the
// caterer portal hands in the start of the running supply.
type Ordering interface {
	// SettleOrders fixes the due orders of the school in ctx and records
	// every change of an ordered day since. The worker runs it every few
	// minutes; without meal registration it does nothing.
	SettleOrders(context.Context) error
	// Orders returns the school's orders of [from, to]. Reading writes
	// nothing and takes no lock: a due order the worker has not stored yet
	// shows as ordered without an ID, a change not recorded yet without a
	// sequence, so all readers see the same numbers.
	Orders(ctx context.Context, from, to Date) (OrderWeek, error)
	// CatererOrders returns the school's orders of [from, to] as the caterer
	// whose supply started at since reads them: only days from since on,
	// numbers by class only when the school orders by class, and the
	// children's names only while the school shows them to the caterer.
	CatererOrders(ctx context.Context, since time.Time, from, to Date) (OrderWeek, error)
	// ConfirmOrder records that the caterer whose supply started at since
	// received a stored order of that supply; confirming twice keeps the
	// first. An order not stored yet has no ID and cannot be confirmed.
	ConfirmOrder(ctx context.Context, orderID int64, since time.Time, accountID int64) error
}

// OrderStatus is where a day's order stands (#3873).
type OrderStatus string

const (
	// OrderOpen: the deadline lies ahead; the numbers are a preview.
	OrderOpen OrderStatus = "open"
	// OrderOrdered: the order is fixed.
	OrderOrdered OrderStatus = "ordered"
	// OrderConfirmed: the caterer confirmed the fixed order.
	OrderConfirmed OrderStatus = "confirmed"
	// OrderUnordered: the day is over and was never ordered, as before the
	// school started ordering. Nothing is billed for it.
	OrderUnordered OrderStatus = "unordered"
)

// Billed reports whether a day in this state is ordered and so billed.
func (s OrderStatus) Billed() bool {
	return s == OrderOrdered || s == OrderConfirmed
}

// OrderFormatVersion is the version of the order as other systems read it.
const OrderFormatVersion = 1

// OrderChild is a child in an order.
type OrderChild struct {
	StudentID   int64
	FirstName   string
	LastName    string
	SchoolClass string
}

// OrderChange is a change after the deadline, numbered per school: one child
// more (+1) or less (-1) on a day. It never says why. Sequence is 0 while
// the change is not recorded yet.
type OrderChange struct {
	Sequence   int
	Date       Date
	Child      OrderChild
	Delta      int
	DetectedAt time.Time
}

// ClassCount is the number of meals of one class.
type ClassCount struct {
	SchoolClass string
	Count       int
}

// OrderDay is one weekday of the orders. Children are the children who eat
// with every change applied; Total and Classes describe the frozen order.
// An open day shows the registration as it stands in all three fields.
// OrderID is 0 for a due order the worker has not stored yet.
type OrderDay struct {
	Date        Date
	Closed      bool
	Deadline    time.Time
	Status      OrderStatus
	OrderID     int64
	ConfirmedAt *time.Time
	Total       int
	Classes     []ClassCount
	Children    []OrderChild
	Changes     []OrderChange
}

// OrderWeek is the orders of a window with the school's rhythm and detail.
type OrderWeek struct {
	Rhythm string
	Detail string
	Days   []OrderDay
}

// WeeklyList is the kitchen list for Monday to Friday.
type WeeklyList struct {
	WeekStart Date
	Days      []DailyList
}
