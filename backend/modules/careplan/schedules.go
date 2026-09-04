package careplan

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	ScheduleSourceStaff        = "staff"
	ScheduleSourceCareOffering = "care_offering"
	ExceptionSourceStaff       = "staff"
	ExceptionSourceGuardian    = "guardian"
)

var (
	ErrInvalidStudentSchedule  = errors.New("invalid student arrival or pickup schedule")
	ErrStudentScheduleNotFound = errors.New("student arrival or pickup schedule not found")
)

// StudentScheduleFilter is the closed query contract shared by the six
// arrival/pickup record types. Dates are inclusive and use DateLayout.
type StudentScheduleFilter struct {
	IDs           []int64
	StudentIDs    []int64
	Weekday       int
	Date          Date
	From          Date
	To            Date
	UpcomingFrom  Date
	LockForUpdate bool
}

type ArrivalSchedule struct {
	ID              int64
	TenantID        int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
	StudentID       int64
	Weekday         int
	ExpectedArrival time.Time
	Notes           *string
	CreatedBy       int64
}

type ArrivalException struct {
	ID                int64
	TenantID          int64
	CreatedAt         time.Time
	UpdatedAt         time.Time
	StudentID         int64
	ExceptionDate     Date
	ExpectedArrival   *time.Time
	Reason            *string
	Source            string
	CreatedBy         int64
	CreatedByGuardian *int64
}

type ArrivalNote struct {
	ID        int64
	TenantID  int64
	CreatedAt time.Time
	UpdatedAt time.Time
	StudentID int64
	NoteDate  Date
	Content   string
	CreatedBy int64
}

type PickupSchedule struct {
	ID             int64
	TenantID       int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
	StudentID      int64
	Weekday        int
	PickupTime     time.Time
	Notes          *string
	CreatedBy      int64
	Source         string
	CareOfferingID *int64
}

type PickupException struct {
	ID                    int64
	TenantID              int64
	CreatedAt             time.Time
	UpdatedAt             time.Time
	StudentID             int64
	ExceptionDate         Date
	PickupTime            *time.Time
	Reason                *string
	ExcusedFrom           *time.Time
	ExcusedReason         *string
	ExcusedCreatedBy      *int64
	ExcusedOwnsPickupTime bool
	ExcusedAuto           bool
	Source                string
	CreatedBy             int64
	CreatedByGuardian     *int64
}

type PickupNote struct {
	ID        int64
	TenantID  int64
	CreatedAt time.Time
	UpdatedAt time.Time
	StudentID int64
	NoteDate  Date
	Content   string
	CreatedBy int64
}

type StudentSchedulesQuery interface {
	FindArrivalSchedule(context.Context, int64) (ArrivalSchedule, error)
	ListArrivalSchedules(context.Context, StudentScheduleFilter) ([]ArrivalSchedule, error)
	FindArrivalException(context.Context, int64, bool) (ArrivalException, error)
	ListArrivalExceptions(context.Context, StudentScheduleFilter) ([]ArrivalException, error)
	FindArrivalNote(context.Context, int64) (ArrivalNote, error)
	ListArrivalNotes(context.Context, StudentScheduleFilter) ([]ArrivalNote, error)
	FindPickupSchedule(context.Context, int64) (PickupSchedule, error)
	ListPickupSchedules(context.Context, StudentScheduleFilter) ([]PickupSchedule, error)
	FindPickupException(context.Context, int64, bool) (PickupException, error)
	ListPickupExceptions(context.Context, StudentScheduleFilter) ([]PickupException, error)
	FindPickupNote(context.Context, int64) (PickupNote, error)
	ListPickupNotes(context.Context, StudentScheduleFilter) ([]PickupNote, error)
	CountStudentScheduleRows(context.Context, int64) (int, error)
}

type StudentSchedulesCommand interface {
	CreateArrivalSchedule(context.Context, ArrivalSchedule) (ArrivalSchedule, error)
	UpdateArrivalSchedule(context.Context, ArrivalSchedule) error
	UpsertArrivalSchedule(context.Context, ArrivalSchedule) (ArrivalSchedule, error)
	DeleteArrivalSchedule(context.Context, int64) error
	DeleteArrivalSchedulesByStudent(context.Context, int64) error
	CreateArrivalException(context.Context, ArrivalException) (ArrivalException, error)
	UpdateArrivalException(context.Context, ArrivalException) error
	DeleteArrivalException(context.Context, int64) error
	DeleteArrivalExceptionsByStudent(context.Context, int64) error
	DeleteArrivalExceptionsBefore(context.Context, Date) (int64, error)
	CreateArrivalNote(context.Context, ArrivalNote) (ArrivalNote, error)
	UpdateArrivalNote(context.Context, ArrivalNote) error
	DeleteArrivalNote(context.Context, int64) error
	DeleteArrivalNotesByStudent(context.Context, int64) error
	DeleteArrivalNotesBefore(context.Context, Date) (int64, error)
	CreatePickupSchedule(context.Context, PickupSchedule) (PickupSchedule, error)
	UpdatePickupSchedule(context.Context, PickupSchedule) error
	UpsertPickupSchedule(context.Context, PickupSchedule) (PickupSchedule, error)
	DeletePickupSchedule(context.Context, int64) error
	DeletePickupSchedulesByStudent(context.Context, int64) error
	CreatePickupException(context.Context, PickupException) (PickupException, error)
	UpdatePickupException(context.Context, PickupException) error
	DeletePickupException(context.Context, int64) error
	DeletePickupExceptionsByStudent(context.Context, int64) error
	DeletePickupExceptionsBefore(context.Context, Date) (int64, error)
	CreatePickupNote(context.Context, PickupNote) (PickupNote, error)
	UpdatePickupNote(context.Context, PickupNote) error
	DeletePickupNote(context.Context, int64) error
	DeletePickupNotesByStudent(context.Context, int64) error
	DeletePickupNotesBefore(context.Context, Date) (int64, error)
	EndStudentSchedulesForCareExit(context.Context, []int64, Date) (int64, error)
	RestoreStudentSchedulesForCareExit(context.Context, []int64) (int64, error)
}

func normalizeStudentScheduleFilter(filter StudentScheduleFilter) StudentScheduleFilter {
	filter.IDs = uniquePositive(filter.IDs)
	filter.StudentIDs = uniquePositive(filter.StudentIDs)
	return filter
}

func (m *Module) FindArrivalSchedule(ctx context.Context, id int64) (ArrivalSchedule, error) {
	if id <= 0 {
		return ArrivalSchedule{}, invalid(ErrInvalidStudentSchedule, "arrival schedule ID is required")
	}
	return m.engine.FindArrivalSchedule(ctx, id)
}
func (m *Module) ListArrivalSchedules(ctx context.Context, filter StudentScheduleFilter) ([]ArrivalSchedule, error) {
	return m.engine.ListArrivalSchedules(ctx, normalizeStudentScheduleFilter(filter))
}
func (m *Module) FindArrivalException(ctx context.Context, id int64, lock bool) (ArrivalException, error) {
	if id <= 0 {
		return ArrivalException{}, invalid(ErrInvalidStudentSchedule, "arrival exception ID is required")
	}
	return m.engine.FindArrivalException(ctx, id, lock)
}
func (m *Module) ListArrivalExceptions(ctx context.Context, filter StudentScheduleFilter) ([]ArrivalException, error) {
	return m.engine.ListArrivalExceptions(ctx, normalizeStudentScheduleFilter(filter))
}
func (m *Module) FindArrivalNote(ctx context.Context, id int64) (ArrivalNote, error) {
	if id <= 0 {
		return ArrivalNote{}, invalid(ErrInvalidStudentSchedule, "arrival note ID is required")
	}
	return m.engine.FindArrivalNote(ctx, id)
}
func (m *Module) ListArrivalNotes(ctx context.Context, filter StudentScheduleFilter) ([]ArrivalNote, error) {
	return m.engine.ListArrivalNotes(ctx, normalizeStudentScheduleFilter(filter))
}
func (m *Module) FindPickupSchedule(ctx context.Context, id int64) (PickupSchedule, error) {
	if id <= 0 {
		return PickupSchedule{}, invalid(ErrInvalidStudentSchedule, "pickup schedule ID is required")
	}
	return m.engine.FindPickupSchedule(ctx, id)
}
func (m *Module) ListPickupSchedules(ctx context.Context, filter StudentScheduleFilter) ([]PickupSchedule, error) {
	return m.engine.ListPickupSchedules(ctx, normalizeStudentScheduleFilter(filter))
}
func (m *Module) FindPickupException(ctx context.Context, id int64, lock bool) (PickupException, error) {
	if id <= 0 {
		return PickupException{}, invalid(ErrInvalidStudentSchedule, "pickup exception ID is required")
	}
	return m.engine.FindPickupException(ctx, id, lock)
}
func (m *Module) ListPickupExceptions(ctx context.Context, filter StudentScheduleFilter) ([]PickupException, error) {
	return m.engine.ListPickupExceptions(ctx, normalizeStudentScheduleFilter(filter))
}
func (m *Module) FindPickupNote(ctx context.Context, id int64) (PickupNote, error) {
	if id <= 0 {
		return PickupNote{}, invalid(ErrInvalidStudentSchedule, "pickup note ID is required")
	}
	return m.engine.FindPickupNote(ctx, id)
}
func (m *Module) ListPickupNotes(ctx context.Context, filter StudentScheduleFilter) ([]PickupNote, error) {
	return m.engine.ListPickupNotes(ctx, normalizeStudentScheduleFilter(filter))
}
func (m *Module) CountStudentScheduleRows(ctx context.Context, studentID int64) (int, error) {
	if studentID <= 0 {
		return 0, invalid(ErrInvalidStudentSchedule, "student ID is required")
	}
	return m.engine.CountStudentScheduleRows(ctx, studentID)
}

func (m *Module) EndStudentSchedulesForCareExit(ctx context.Context, studentIDs []int64, validUntil Date) (int64, error) {
	studentIDs = uniquePositive(studentIDs)
	if len(studentIDs) == 0 {
		return 0, nil
	}
	if validUntil.IsZero() {
		return 0, invalid(ErrInvalidStudentSchedule, "care-exit date is required")
	}
	return m.engine.EndStudentSchedulesForCareExit(ctx, studentIDs, validUntil)
}

func (m *Module) RestoreStudentSchedulesForCareExit(ctx context.Context, studentIDs []int64) (int64, error) {
	studentIDs = uniquePositive(studentIDs)
	if len(studentIDs) == 0 {
		return 0, nil
	}
	return m.engine.RestoreStudentSchedulesForCareExit(ctx, studentIDs)
}

func validWeekday(day int) bool { return day >= 1 && day <= 5 }
func validAuthor(source string, staff int64, guardian *int64) bool {
	return (source == "" || source == ExceptionSourceStaff) && staff > 0 || source == ExceptionSourceGuardian && guardian != nil && *guardian > 0
}
func validText(value *string, max int) bool {
	return value == nil || utf8.RuneCountInString(*value) <= max
}
func validateArrivalSchedule(v ArrivalSchedule) bool {
	return v.StudentID > 0 && validWeekday(v.Weekday) && v.CreatedBy > 0 && validText(v.Notes, 500)
}
func validateArrivalException(v ArrivalException) bool {
	return v.StudentID > 0 && !v.ExceptionDate.IsZero() && validText(v.Reason, 255) && validAuthor(v.Source, v.CreatedBy, v.CreatedByGuardian)
}
func validateNote(studentID int64, date Date, content string, createdBy int64) bool {
	return studentID > 0 && !date.IsZero() && strings.TrimSpace(content) != "" && len(content) <= 500 && createdBy > 0
}
func validatePickupSchedule(v PickupSchedule) bool {
	if v.StudentID <= 0 || !validWeekday(v.Weekday) || v.PickupTime.IsZero() || !validText(v.Notes, 500) {
		return false
	}
	if v.Source == "" || v.Source == ScheduleSourceStaff {
		return v.CreatedBy > 0
	}
	return v.Source == ScheduleSourceCareOffering && v.CareOfferingID != nil && *v.CareOfferingID > 0
}
func validatePickupException(v PickupException) bool {
	if v.StudentID <= 0 || v.ExceptionDate.IsZero() || !validText(v.Reason, 255) || !validText(v.ExcusedReason, 255) || !validAuthor(v.Source, v.CreatedBy, v.CreatedByGuardian) {
		return false
	}
	if v.ExcusedFrom == nil {
		return v.ExcusedReason == nil && v.ExcusedCreatedBy == nil && !v.ExcusedOwnsPickupTime && !v.ExcusedAuto
	}
	if v.ExcusedAuto {
		return v.ExcusedCreatedBy == nil && !v.ExcusedOwnsPickupTime && v.PickupTime != nil
	}
	return v.ExcusedCreatedBy != nil && *v.ExcusedCreatedBy > 0 && (!v.ExcusedOwnsPickupTime || v.PickupTime != nil)
}

func scheduleInvalid() error {
	return invalid(ErrInvalidStudentSchedule, "student schedule record is invalid")
}

func (m *Module) CreateArrivalSchedule(ctx context.Context, v ArrivalSchedule) (ArrivalSchedule, error) {
	if !validateArrivalSchedule(v) {
		return ArrivalSchedule{}, scheduleInvalid()
	}
	return m.engine.CreateArrivalSchedule(ctx, v)
}
func (m *Module) UpdateArrivalSchedule(ctx context.Context, v ArrivalSchedule) error {
	if v.ID <= 0 || !validateArrivalSchedule(v) {
		return scheduleInvalid()
	}
	return m.engine.UpdateArrivalSchedule(ctx, v)
}
func (m *Module) UpsertArrivalSchedule(ctx context.Context, v ArrivalSchedule) (ArrivalSchedule, error) {
	if !validateArrivalSchedule(v) {
		return ArrivalSchedule{}, scheduleInvalid()
	}
	return m.engine.UpsertArrivalSchedule(ctx, v)
}
func (m *Module) DeleteArrivalSchedule(ctx context.Context, id int64) error {
	if id <= 0 {
		return scheduleInvalid()
	}
	return m.engine.DeleteArrivalSchedule(ctx, id)
}
func (m *Module) DeleteArrivalSchedulesByStudent(ctx context.Context, id int64) error {
	if id <= 0 {
		return scheduleInvalid()
	}
	return m.engine.DeleteArrivalSchedulesByStudent(ctx, id)
}
func (m *Module) CreateArrivalException(ctx context.Context, v ArrivalException) (ArrivalException, error) {
	if !validateArrivalException(v) {
		return ArrivalException{}, scheduleInvalid()
	}
	return m.engine.CreateArrivalException(ctx, v)
}
func (m *Module) UpdateArrivalException(ctx context.Context, v ArrivalException) error {
	if v.ID <= 0 || !validateArrivalException(v) {
		return scheduleInvalid()
	}
	return m.engine.UpdateArrivalException(ctx, v)
}
func (m *Module) DeleteArrivalException(ctx context.Context, id int64) error {
	if id <= 0 {
		return scheduleInvalid()
	}
	return m.engine.DeleteArrivalException(ctx, id)
}
func (m *Module) DeleteArrivalExceptionsByStudent(ctx context.Context, id int64) error {
	if id <= 0 {
		return scheduleInvalid()
	}
	return m.engine.DeleteArrivalExceptionsByStudent(ctx, id)
}
func (m *Module) DeleteArrivalExceptionsBefore(ctx context.Context, d Date) (int64, error) {
	if d.IsZero() {
		return 0, scheduleInvalid()
	}
	return m.engine.DeleteArrivalExceptionsBefore(ctx, d)
}
func (m *Module) CreateArrivalNote(ctx context.Context, v ArrivalNote) (ArrivalNote, error) {
	if !validateNote(v.StudentID, v.NoteDate, v.Content, v.CreatedBy) {
		return ArrivalNote{}, scheduleInvalid()
	}
	return m.engine.CreateArrivalNote(ctx, v)
}
func (m *Module) UpdateArrivalNote(ctx context.Context, v ArrivalNote) error {
	if v.ID <= 0 || !validateNote(v.StudentID, v.NoteDate, v.Content, v.CreatedBy) {
		return scheduleInvalid()
	}
	return m.engine.UpdateArrivalNote(ctx, v)
}
func (m *Module) DeleteArrivalNote(ctx context.Context, id int64) error {
	if id <= 0 {
		return scheduleInvalid()
	}
	return m.engine.DeleteArrivalNote(ctx, id)
}
func (m *Module) DeleteArrivalNotesByStudent(ctx context.Context, id int64) error {
	if id <= 0 {
		return scheduleInvalid()
	}
	return m.engine.DeleteArrivalNotesByStudent(ctx, id)
}
func (m *Module) DeleteArrivalNotesBefore(ctx context.Context, d Date) (int64, error) {
	if d.IsZero() {
		return 0, scheduleInvalid()
	}
	return m.engine.DeleteArrivalNotesBefore(ctx, d)
}
func (m *Module) CreatePickupSchedule(ctx context.Context, v PickupSchedule) (PickupSchedule, error) {
	if !validatePickupSchedule(v) {
		return PickupSchedule{}, scheduleInvalid()
	}
	return m.engine.CreatePickupSchedule(ctx, v)
}
func (m *Module) UpdatePickupSchedule(ctx context.Context, v PickupSchedule) error {
	if v.ID <= 0 || !validatePickupSchedule(v) {
		return scheduleInvalid()
	}
	return m.engine.UpdatePickupSchedule(ctx, v)
}
func (m *Module) UpsertPickupSchedule(ctx context.Context, v PickupSchedule) (PickupSchedule, error) {
	if !validatePickupSchedule(v) {
		return PickupSchedule{}, scheduleInvalid()
	}
	return m.engine.UpsertPickupSchedule(ctx, v)
}
func (m *Module) DeletePickupSchedule(ctx context.Context, id int64) error {
	if id <= 0 {
		return scheduleInvalid()
	}
	return m.engine.DeletePickupSchedule(ctx, id)
}
func (m *Module) DeletePickupSchedulesByStudent(ctx context.Context, id int64) error {
	if id <= 0 {
		return scheduleInvalid()
	}
	return m.engine.DeletePickupSchedulesByStudent(ctx, id)
}
func (m *Module) CreatePickupException(ctx context.Context, v PickupException) (PickupException, error) {
	if !validatePickupException(v) {
		return PickupException{}, scheduleInvalid()
	}
	return m.engine.CreatePickupException(ctx, v)
}
func (m *Module) UpdatePickupException(ctx context.Context, v PickupException) error {
	if v.ID <= 0 || !validatePickupException(v) {
		return scheduleInvalid()
	}
	return m.engine.UpdatePickupException(ctx, v)
}
func (m *Module) DeletePickupException(ctx context.Context, id int64) error {
	if id <= 0 {
		return scheduleInvalid()
	}
	return m.engine.DeletePickupException(ctx, id)
}
func (m *Module) DeletePickupExceptionsByStudent(ctx context.Context, id int64) error {
	if id <= 0 {
		return scheduleInvalid()
	}
	return m.engine.DeletePickupExceptionsByStudent(ctx, id)
}
func (m *Module) DeletePickupExceptionsBefore(ctx context.Context, d Date) (int64, error) {
	if d.IsZero() {
		return 0, scheduleInvalid()
	}
	return m.engine.DeletePickupExceptionsBefore(ctx, d)
}
func (m *Module) CreatePickupNote(ctx context.Context, v PickupNote) (PickupNote, error) {
	if !validateNote(v.StudentID, v.NoteDate, v.Content, v.CreatedBy) {
		return PickupNote{}, scheduleInvalid()
	}
	return m.engine.CreatePickupNote(ctx, v)
}
func (m *Module) UpdatePickupNote(ctx context.Context, v PickupNote) error {
	if v.ID <= 0 || !validateNote(v.StudentID, v.NoteDate, v.Content, v.CreatedBy) {
		return scheduleInvalid()
	}
	return m.engine.UpdatePickupNote(ctx, v)
}
func (m *Module) DeletePickupNote(ctx context.Context, id int64) error {
	if id <= 0 {
		return scheduleInvalid()
	}
	return m.engine.DeletePickupNote(ctx, id)
}
func (m *Module) DeletePickupNotesByStudent(ctx context.Context, id int64) error {
	if id <= 0 {
		return scheduleInvalid()
	}
	return m.engine.DeletePickupNotesByStudent(ctx, id)
}
func (m *Module) DeletePickupNotesBefore(ctx context.Context, d Date) (int64, error) {
	if d.IsZero() {
		return 0, scheduleInvalid()
	}
	return m.engine.DeletePickupNotesBefore(ctx, d)
}
