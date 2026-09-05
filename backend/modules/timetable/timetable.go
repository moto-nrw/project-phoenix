// Package timetable is the public Timetable & Activities capability. It owns
// activity categories, templates, target groups, schedules, rosters, and
// materialized activity instances.
package timetable

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
)

const (
	DefaultCategoryColor         = "#CCCCCC"
	SchulhofCategoryName         = "Schulhof"
	WCCategoryName               = "WC"
	categoryNameMaxLength        = 60
	categoryDescriptionMaxLength = 255
)

var (
	ErrCategoryNotFound          = errors.New("activity category not found")
	ErrInvalidCategory           = errors.New("invalid activity category")
	ErrInvalidCareExitEnrollment = errors.New("invalid care-exit activity enrollment")
	ErrUnknownCategoryIDs        = errors.New("one or more category IDs do not exist in this tenant")
	ErrSystemCategoryProtected   = errors.New("Systemkategorie kann nicht bearbeitet oder archiviert werden") //nolint:staticcheck // ST1005: stable user-facing contract
	ErrSystemCategoryName        = errors.New("Dieser Name ist für eine Systemkategorie reserviert")          //nolint:staticcheck // ST1005: stable user-facing contract
	ErrCategoryNameExists        = errors.New("Eine Kategorie mit diesem Namen existiert bereits")            //nolint:staticcheck // ST1005: stable user-facing contract
	ErrCategoryArchived          = errors.New("Archivierte Kategorie muss zuerst wiederhergestellt werden")   //nolint:staticcheck // ST1005: stable user-facing contract
)

var categoryColorPattern = regexp.MustCompile(`^#([A-Fa-f0-9]{6}|[A-Fa-f0-9]{3})$`)

// Category is the owner view of one activities.categories row.
type Category struct {
	ID          int64      `json:"id"`
	TenantID    int64      `json:"tenant_id"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	Color       string     `json:"color,omitempty"`
	IsSystem    bool       `json:"is_system"`
	ShiftTypeID *int64     `json:"shift_type_id,omitempty"`
	ArchivedAt  *time.Time `json:"archived_at,omitempty"`
}

func (c Category) IsArchived() bool { return c.ArchivedAt != nil }

func (c Category) ColorOrDefault() string {
	if c.Color == "" {
		return DefaultCategoryColor
	}
	return c.Color
}

type CreateCategory struct {
	Name        string
	Description string
	Color       string
	IsSystem    bool
}

type UpdateCategory struct {
	ID          int64
	Name        string
	Description string
	Color       string
}

// CareExitEnrollment is the reversible snapshot used when a student's care
// ends. Calendar dates use YYYY-MM-DD because this public contract represents
// dates, not instants.
type CareExitEnrollment struct {
	ID                       int64
	TenantID                 int64
	StudentID                int64
	ActivityGroupID          int64
	ValidFrom                string
	ValidUntil               *string
	CalendarPeriodID         *int64
	EnrollmentRequestChildID *int64
	SelectedWeekdays         []int
	AttendanceStatus         *string
	Weekday                  *int
}

type CareExitEnrollmentCap struct {
	TenantID           int64
	StudentID          int64
	ID                 int64
	PreviousValidUntil *string
}

type CareExitEnrollmentChanges struct {
	Deleted []CareExitEnrollment
	Capped  []CareExitEnrollmentCap
}

type CareExitEnrollmentRemoval struct {
	CareExitEnrollment
	WasDeleted         bool
	PreviousValidUntil *string
}

type Query interface {
	FindCategory(context.Context, int64) (Category, error)
	FindCategoryForAssignment(context.Context, int64) (Category, error)
	FindCategoryByName(context.Context, string) (Category, error)
	ListCategories(context.Context) ([]Category, error)
	CountCategoryUsage(context.Context) (map[int64]int, error)
}

type Command interface {
	CreateCategory(context.Context, CreateCategory) (Category, error)
	UpdateCategory(context.Context, UpdateCategory) (Category, error)
	ArchiveCategory(context.Context, int64) (Category, error)
	RestoreCategory(context.Context, int64) (Category, error)
	SetCategoryShiftTypeLinks(context.Context, int64, []int64) error
	LockStudentEnrollmentsForCareExit(context.Context, []int64, string) error
	EndStudentEnrollmentsForCareExit(context.Context, []int64, string) (CareExitEnrollmentChanges, error)
	RestoreStudentEnrollmentsForCareExit(context.Context, []int64, []int64, []CareExitEnrollmentRemoval) (int, error)
}

type Capability interface {
	Query
	Command
}

type engine interface {
	Capability
	ObserveRejection(string, time.Duration, error)
}

type Module struct{ engine engine }

func NewModule(engine engine) *Module {
	if engine == nil {
		panic("timetable: engine is required")
	}
	return &Module{engine: engine}
}

func (m *Module) FindCategory(ctx context.Context, id int64) (Category, error) {
	if id <= 0 {
		return Category{}, m.reject("find_category", ErrInvalidCategory)
	}
	return m.engine.FindCategory(ctx, id)
}

func (m *Module) FindCategoryForAssignment(ctx context.Context, id int64) (Category, error) {
	if id <= 0 {
		return Category{}, m.reject("find_category_for_assignment", ErrInvalidCategory)
	}
	return m.engine.FindCategoryForAssignment(ctx, id)
}

func (m *Module) FindCategoryByName(ctx context.Context, name string) (Category, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Category{}, m.reject("find_category_by_name", ErrInvalidCategory)
	}
	return m.engine.FindCategoryByName(ctx, name)
}

func (m *Module) ListCategories(ctx context.Context) ([]Category, error) {
	return m.engine.ListCategories(ctx)
}

func (m *Module) CountCategoryUsage(ctx context.Context) (map[int64]int, error) {
	return m.engine.CountCategoryUsage(ctx)
}

func (m *Module) CreateCategory(ctx context.Context, input CreateCategory) (Category, error) {
	started := time.Now()
	if err := normalizeCategory(&input.Name, &input.Description, &input.Color); err != nil {
		m.engine.ObserveRejection("create_category", time.Since(started), err)
		return Category{}, err
	}
	if !input.IsSystem && reservedCategoryName(input.Name) {
		m.engine.ObserveRejection("create_category", time.Since(started), ErrSystemCategoryName)
		return Category{}, ErrSystemCategoryName
	}
	return m.engine.CreateCategory(ctx, input)
}

func (m *Module) UpdateCategory(ctx context.Context, input UpdateCategory) (Category, error) {
	started := time.Now()
	if input.ID <= 0 {
		m.engine.ObserveRejection("update_category", time.Since(started), ErrInvalidCategory)
		return Category{}, ErrInvalidCategory
	}
	if err := normalizeCategory(&input.Name, &input.Description, &input.Color); err != nil {
		m.engine.ObserveRejection("update_category", time.Since(started), err)
		return Category{}, err
	}
	if reservedCategoryName(input.Name) {
		m.engine.ObserveRejection("update_category", time.Since(started), ErrSystemCategoryName)
		return Category{}, ErrSystemCategoryName
	}
	return m.engine.UpdateCategory(ctx, input)
}

func (m *Module) ArchiveCategory(ctx context.Context, id int64) (Category, error) {
	if id <= 0 {
		return Category{}, m.reject("archive_category", ErrInvalidCategory)
	}
	return m.engine.ArchiveCategory(ctx, id)
}

func (m *Module) RestoreCategory(ctx context.Context, id int64) (Category, error) {
	if id <= 0 {
		return Category{}, m.reject("restore_category", ErrInvalidCategory)
	}
	return m.engine.RestoreCategory(ctx, id)
}

func (m *Module) SetCategoryShiftTypeLinks(ctx context.Context, shiftTypeID int64, categoryIDs []int64) error {
	if shiftTypeID <= 0 || hasInvalidID(categoryIDs) {
		return m.reject("set_category_shift_type_links", ErrInvalidCategory)
	}
	return m.engine.SetCategoryShiftTypeLinks(ctx, shiftTypeID, categoryIDs)
}

func (m *Module) LockStudentEnrollmentsForCareExit(ctx context.Context, studentIDs []int64, validUntil string) error {
	if len(studentIDs) == 0 || hasInvalidID(studentIDs) || !validDate(validUntil) {
		return m.reject("lock_student_enrollments_for_care_exit", ErrInvalidCareExitEnrollment)
	}
	return m.engine.LockStudentEnrollmentsForCareExit(ctx, studentIDs, validUntil)
}

func (m *Module) EndStudentEnrollmentsForCareExit(ctx context.Context, studentIDs []int64, validUntil string) (CareExitEnrollmentChanges, error) {
	if len(studentIDs) == 0 || hasInvalidID(studentIDs) || !validDate(validUntil) {
		return CareExitEnrollmentChanges{}, m.reject("end_student_enrollments_for_care_exit", ErrInvalidCareExitEnrollment)
	}
	return m.engine.EndStudentEnrollmentsForCareExit(ctx, studentIDs, validUntil)
}

func (m *Module) RestoreStudentEnrollmentsForCareExit(ctx context.Context, studentIDs, periodIDs []int64, removals []CareExitEnrollmentRemoval) (int, error) {
	if len(studentIDs) == 0 || hasInvalidID(studentIDs) || hasInvalidID(periodIDs) || invalidCareExitRemovals(removals) {
		return 0, m.reject("restore_student_enrollments_for_care_exit", ErrInvalidCareExitEnrollment)
	}
	return m.engine.RestoreStudentEnrollmentsForCareExit(ctx, studentIDs, periodIDs, removals)
}

func (m *Module) reject(operation string, err error) error {
	m.engine.ObserveRejection(operation, 0, err)
	return err
}

func normalizeCategory(name, description, color *string) error {
	*name = strings.TrimSpace(*name)
	*description = strings.TrimSpace(*description)
	*color = strings.TrimSpace(*color)
	if *name == "" || len([]rune(*name)) > categoryNameMaxLength || len([]rune(*description)) > categoryDescriptionMaxLength {
		return ErrInvalidCategory
	}
	if *color != "" && !strings.HasPrefix(*color, "#") {
		*color = "#" + *color
	}
	if *color != "" && !categoryColorPattern.MatchString(*color) {
		return ErrInvalidCategory
	}
	return nil
}

func reservedCategoryName(name string) bool {
	return strings.EqualFold(name, WCCategoryName) || strings.EqualFold(name, SchulhofCategoryName)
}

func hasInvalidID(ids []int64) bool {
	for _, id := range ids {
		if id <= 0 {
			return true
		}
	}
	return false
}

func validDate(value string) bool {
	_, err := time.Parse(time.DateOnly, value)
	return err == nil
}

func invalidCareExitRemovals(removals []CareExitEnrollmentRemoval) bool {
	for _, removal := range removals {
		if removal.ID <= 0 || removal.TenantID <= 0 || removal.StudentID <= 0 {
			return true
		}
		if removal.WasDeleted && (removal.ActivityGroupID <= 0 || !validDate(removal.ValidFrom)) {
			return true
		}
		if (removal.ValidUntil != nil && !validDate(*removal.ValidUntil)) ||
			(removal.PreviousValidUntil != nil && !validDate(*removal.PreviousValidUntil)) {
			return true
		}
	}
	return false
}

func ErrorCode(err error) string {
	switch {
	case err == nil:
		return "none"
	case errors.Is(err, ErrCategoryNotFound):
		return "not_found"
	case errors.Is(err, ErrInvalidCategory):
		return "invalid"
	case errors.Is(err, ErrInvalidCareExitEnrollment):
		return "invalid_care_exit_enrollment"
	case errors.Is(err, ErrUnknownCategoryIDs):
		return "unknown_category_ids"
	case errors.Is(err, ErrSystemCategoryProtected):
		return "system_category_protected"
	case errors.Is(err, ErrSystemCategoryName):
		return "system_category_name_reserved"
	case errors.Is(err, ErrCategoryNameExists):
		return "category_name_exists"
	case errors.Is(err, ErrCategoryArchived):
		return "category_archived"
	default:
		return "internal_error"
	}
}
