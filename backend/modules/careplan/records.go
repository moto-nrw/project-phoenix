package careplan

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	CareExitReasonMovedAway  = "moved_away"
	CareExitReasonNoCareNeed = "no_care_needed"
	CareExitReasonOther      = "other"
	MaxCareExitNoteLen       = 200

	CareExitRemovalRoster  = "roster"
	CareExitRemovalBooking = "booking"

	CareExitSourceBooking          = "source_booking"
	CareExitSourcePickupSchedule   = "pickup_schedule"
	CareExitSourcePickupException  = "pickup_exception"
	CareExitSourceArrivalSchedule  = "arrival_schedule"
	CareExitSourceArrivalException = "arrival_exception"
)

var (
	ErrCareExitInvalidReason  = errors.New("care plan: invalid care exit reason")
	ErrCareExitNoteRequired   = errors.New("care plan: care exit reason note is required")
	ErrCareExitNoteNotAllowed = errors.New("care plan: care exit reason note is not allowed")
	ErrCareExitNoteTooLong    = errors.New("care plan: care exit reason note is too long")
	ErrInvalidCareExit        = errors.New("invalid care exit")
	ErrInvalidCompanion       = errors.New("invalid student companion")
	ErrInvalidCareDocument    = errors.New("invalid student care document")
	ErrCareDocumentNotFound   = errors.New("student care document not found")
)

// CareExit records why a child's care ended and who recorded it.
type CareExit struct {
	ID                     int64
	TenantID               int64
	CreatedAt              time.Time
	UpdatedAt              time.Time
	StudentID              int64
	PreviousEnrolledUntil  *Date
	Reason                 string
	ReasonNote             *string
	RecordedBy             *int64
	WithdrawalCompletionID *int64
}

// CompanionEdge is one normalized undirected departure-companion edge.
type CompanionEdge struct {
	ID            int64
	TenantID      int64
	CreatedAt     time.Time
	UpdatedAt     time.Time
	StudentLowID  int64
	StudentHighID int64
	Weekday       int
}

// CompanionLink is the per-child view of companion edges.
type CompanionLink struct {
	CompanionStudentID int64
	FirstName          string
	LastName           string
	Weekdays           []string
}

// CareDocument is metadata for one stored student document.
type CareDocument struct {
	ID              int64
	TenantID        int64
	CreatedAt       time.Time
	UpdatedAt       time.Time
	StudentID       int64
	Category        string
	FilenameDisplay string
	FilenameStored  string
	SizeBytes       int64
	ContentType     string
	UploadedBy      int64
	DeletedAt       *time.Time
	DeletedBy       *int64
	FileDeletedAt   *time.Time
}

type CareDocumentCleanup struct {
	ID             int64
	TenantID       int64
	CreatedAt      time.Time
	UpdatedAt      time.Time
	OwnerID        int64
	FilenameStored string
	RetryAfter     time.Time
	CleanedAt      *time.Time
}

// CareExitRemoval is one reversible roster or activity-booking change made by
// a planned care exit. It is deliberately a data record: the owning Care Plan
// module persists it, while the schedule/activity owners replay their rows.
type CareExitRemoval struct {
	ID                       int64      `json:"id"`
	TenantID                 int64      `json:"tenant_id"`
	StudentID                int64      `json:"student_id"`
	Kind                     string     `json:"kind"`
	InstanceID               *int64     `json:"instance_id"`
	RoomID                   *int64     `json:"room_id"`
	Status                   *string    `json:"status"`
	Substatus                *string    `json:"substatus"`
	Note                     *string    `json:"note"`
	IsUnplanned              *bool      `json:"is_unplanned"`
	NotScheduled             *bool      `json:"not_scheduled"`
	ManualStatusAt           *time.Time `json:"manual_status_at"`
	StudentStatusDayID       *int64     `json:"student_status_day_id"`
	PickupExceptionID        *int64     `json:"pickup_exception_id"`
	EnrollmentID             *int64     `json:"enrollment_id"`
	WasDeleted               bool       `json:"was_deleted"`
	PreviousValidUntil       *Date      `json:"previous_valid_until"`
	ActivityGroupID          *int64     `json:"activity_group_id"`
	ValidFrom                *Date      `json:"valid_from"`
	CalendarPeriodID         *int64     `json:"calendar_period_id"`
	EnrollmentRequestChildID *int64     `json:"enrollment_request_child_id"`
	SelectedWeekdays         []int      `json:"selected_weekdays"`
	AttendanceStatus         *string    `json:"attendance_status"`
	Weekday                  *int       `json:"weekday"`
	CreatedAt                time.Time  `json:"created_at"`
}

// CareExitSourceRemoval is a reversible JSON snapshot of a source booking or
// recurring arrival/pickup row removed by a planned care exit.
type CareExitSourceRemoval struct {
	ID          int64           `json:"id"`
	TenantID    int64           `json:"tenant_id"`
	StudentID   int64           `json:"student_id"`
	Kind        string          `json:"kind"`
	SourceRowID int64           `json:"source_row_id"`
	WasDeleted  bool            `json:"was_deleted"`
	Snapshot    json.RawMessage `json:"snapshot"`
	CreatedAt   time.Time       `json:"created_at"`
}

type CareRecordsQuery interface {
	FindCareExits(context.Context, []int64) (map[int64]CareExit, error)
	ListCompanionEdges(context.Context, int64) ([]CompanionEdge, error)
	ListCompanionLinks(context.Context, []int64) (map[int64][]CompanionLink, error)
	CompanionCountsExcluding(context.Context, []int64, int64) (map[int64]int, error)
	CompanionDaysCoveredExcluding(context.Context, []int64, int64) (map[int64]map[string]bool, error)
	CompanionIDsForWeekday(context.Context, []int64, int) (map[int64][]int64, error)
	CompanionWeekdays(context.Context, int64) ([]int, error)
	CountCompanionLinks(context.Context, int64) (int, error)
	FindCareDocument(context.Context, int64, int64, bool) (CareDocument, error)
	ListCareDocuments(context.Context, int64, []string) ([]CareDocument, error)
	ListPendingCareDocumentCleanup(context.Context, int64) ([]CareDocument, error)
	ListDeletedCareDocuments(context.Context, int64, []string) ([]CareDocument, error)
	ListCareDocumentCleanups(context.Context, *int64) ([]CareDocumentCleanup, error)
	ListCareExitRemovals(context.Context, []int64) ([]CareExitRemoval, error)
	ListCareExitSourceRemovals(context.Context, []int64) ([]CareExitSourceRemoval, error)
}

type CareRecordsCommand interface {
	UpsertCareExit(context.Context, CareExit) error
	DeleteCareExits(context.Context, []int64) error
	ReplaceCompanionEdges(context.Context, int64, []CompanionEdge) error
	DeleteCompanionEdges(context.Context, []int64) error
	CreateCareDocument(context.Context, CareDocument) (CareDocument, error)
	SoftDeleteCareDocument(context.Context, int64, int64) (time.Time, error)
	MarkCareDocumentFileDeleted(context.Context, int64) error
	QueueCareDocumentCleanup(context.Context, CareDocumentCleanup) (CareDocumentCleanup, error)
	CompleteCareDocumentCleanup(context.Context, int64) error
	CompleteCareDocumentCleanupByFilename(context.Context, string) error
	ActivateCareDocumentCleanup(context.Context, string) error
	RecordCareExitRemovals(context.Context, []CareExitRemoval) error
	RecordCareExitSourceRemovals(context.Context, []CareExitSourceRemoval) error
	DiscardCareExitRemovals(context.Context, []int64) error
}

func (m *Module) FindCareExits(ctx context.Context, studentIDs []int64) (map[int64]CareExit, error) {
	return m.engine.FindCareExits(ctx, uniquePositive(studentIDs))
}

func (m *Module) UpsertCareExit(ctx context.Context, value CareExit) error {
	if err := normalizeCareExit(&value); err != nil {
		return err
	}
	return m.engine.UpsertCareExit(ctx, value)
}

func (m *Module) DeleteCareExits(ctx context.Context, studentIDs []int64) error {
	return m.engine.DeleteCareExits(ctx, uniquePositive(studentIDs))
}

func (m *Module) ListCompanionEdges(ctx context.Context, studentID int64) ([]CompanionEdge, error) {
	if studentID <= 0 {
		return nil, invalid(ErrInvalidCompanion, "student ID is required")
	}
	return m.engine.ListCompanionEdges(ctx, studentID)
}

func (m *Module) ListCompanionLinks(ctx context.Context, studentIDs []int64) (map[int64][]CompanionLink, error) {
	return m.engine.ListCompanionLinks(ctx, uniquePositive(studentIDs))
}

func (m *Module) CompanionCountsExcluding(ctx context.Context, studentIDs []int64, excludeID int64) (map[int64]int, error) {
	return m.engine.CompanionCountsExcluding(ctx, uniquePositive(studentIDs), excludeID)
}

func (m *Module) CompanionDaysCoveredExcluding(ctx context.Context, studentIDs []int64, excludeID int64) (map[int64]map[string]bool, error) {
	return m.engine.CompanionDaysCoveredExcluding(ctx, uniquePositive(studentIDs), excludeID)
}

func (m *Module) CompanionIDsForWeekday(ctx context.Context, studentIDs []int64, weekday int) (map[int64][]int64, error) {
	if weekday < 1 || weekday > 5 {
		return nil, invalid(ErrInvalidCompanion, "companion weekday must be between 1 and 5")
	}
	return m.engine.CompanionIDsForWeekday(ctx, uniquePositive(studentIDs), weekday)
}

func (m *Module) CompanionWeekdays(ctx context.Context, studentID int64) ([]int, error) {
	if studentID <= 0 {
		return nil, invalid(ErrInvalidCompanion, "student ID is required")
	}
	return m.engine.CompanionWeekdays(ctx, studentID)
}

func (m *Module) CountCompanionLinks(ctx context.Context, studentID int64) (int, error) {
	if studentID <= 0 {
		return 0, invalid(ErrInvalidCompanion, "student ID is required")
	}
	return m.engine.CountCompanionLinks(ctx, studentID)
}

func (m *Module) ReplaceCompanionEdges(ctx context.Context, studentID int64, edges []CompanionEdge) error {
	if studentID <= 0 {
		return invalid(ErrInvalidCompanion, "student ID is required")
	}
	for i := range edges {
		if edges[i].StudentLowID <= 0 || edges[i].StudentHighID <= 0 || edges[i].StudentLowID >= edges[i].StudentHighID || edges[i].Weekday < 1 || edges[i].Weekday > 5 {
			return invalid(ErrInvalidCompanion, "companion edge is invalid")
		}
		if edges[i].StudentLowID != studentID && edges[i].StudentHighID != studentID {
			return invalid(ErrInvalidCompanion, "companion edge does not contain the student")
		}
	}
	return m.engine.ReplaceCompanionEdges(ctx, studentID, edges)
}

func (m *Module) DeleteCompanionEdges(ctx context.Context, edgeIDs []int64) error {
	return m.engine.DeleteCompanionEdges(ctx, uniquePositive(edgeIDs))
}

func (m *Module) FindCareDocument(ctx context.Context, studentID, documentID int64, includeDeleted bool) (CareDocument, error) {
	if studentID <= 0 || documentID <= 0 {
		return CareDocument{}, invalid(ErrInvalidCareDocument, "student and document IDs are required")
	}
	return m.engine.FindCareDocument(ctx, studentID, documentID, includeDeleted)
}

func (m *Module) ListCareDocuments(ctx context.Context, studentID int64, categories []string) ([]CareDocument, error) {
	if studentID <= 0 {
		return nil, invalid(ErrInvalidCareDocument, "student ID is required")
	}
	return m.engine.ListCareDocuments(ctx, studentID, categories)
}

func (m *Module) ListPendingCareDocumentCleanup(ctx context.Context, studentID int64) ([]CareDocument, error) {
	if studentID <= 0 {
		return nil, invalid(ErrInvalidCareDocument, "student ID is required")
	}
	return m.engine.ListPendingCareDocumentCleanup(ctx, studentID)
}

func (m *Module) ListDeletedCareDocuments(ctx context.Context, studentID int64, categories []string) ([]CareDocument, error) {
	if studentID < 0 {
		return nil, invalid(ErrInvalidCareDocument, "student ID is invalid")
	}
	return m.engine.ListDeletedCareDocuments(ctx, studentID, categories)
}

func (m *Module) ListCareDocumentCleanups(ctx context.Context, studentID *int64) ([]CareDocumentCleanup, error) {
	if studentID != nil && *studentID <= 0 {
		return nil, invalid(ErrInvalidCareDocument, "student ID is invalid")
	}
	return m.engine.ListCareDocumentCleanups(ctx, studentID)
}

func (m *Module) CreateCareDocument(ctx context.Context, value CareDocument) (CareDocument, error) {
	if err := validateCareDocument(value); err != nil {
		return CareDocument{}, err
	}
	return m.engine.CreateCareDocument(ctx, value)
}

func (m *Module) SoftDeleteCareDocument(ctx context.Context, documentID, deletedBy int64) (time.Time, error) {
	if documentID <= 0 || deletedBy <= 0 {
		return time.Time{}, invalid(ErrInvalidCareDocument, "document and actor IDs are required")
	}
	return m.engine.SoftDeleteCareDocument(ctx, documentID, deletedBy)
}

func (m *Module) MarkCareDocumentFileDeleted(ctx context.Context, documentID int64) error {
	if documentID <= 0 {
		return invalid(ErrInvalidCareDocument, "document ID is required")
	}
	return m.engine.MarkCareDocumentFileDeleted(ctx, documentID)
}

func (m *Module) QueueCareDocumentCleanup(ctx context.Context, value CareDocumentCleanup) (CareDocumentCleanup, error) {
	if value.OwnerID <= 0 || strings.TrimSpace(value.FilenameStored) == "" || value.RetryAfter.IsZero() {
		return CareDocumentCleanup{}, invalid(ErrInvalidCareDocument, "care document cleanup is invalid")
	}
	return m.engine.QueueCareDocumentCleanup(ctx, value)
}

func (m *Module) CompleteCareDocumentCleanup(ctx context.Context, cleanupID int64) error {
	if cleanupID <= 0 {
		return invalid(ErrInvalidCareDocument, "cleanup ID is required")
	}
	return m.engine.CompleteCareDocumentCleanup(ctx, cleanupID)
}

func (m *Module) CompleteCareDocumentCleanupByFilename(ctx context.Context, filename string) error {
	if strings.TrimSpace(filename) == "" {
		return invalid(ErrInvalidCareDocument, "stored filename is required")
	}
	return m.engine.CompleteCareDocumentCleanupByFilename(ctx, filename)
}

func (m *Module) ActivateCareDocumentCleanup(ctx context.Context, filename string) error {
	if strings.TrimSpace(filename) == "" {
		return invalid(ErrInvalidCareDocument, "stored filename is required")
	}
	return m.engine.ActivateCareDocumentCleanup(ctx, filename)
}

func (m *Module) ListCareExitRemovals(ctx context.Context, studentIDs []int64) ([]CareExitRemoval, error) {
	return m.engine.ListCareExitRemovals(ctx, uniquePositive(studentIDs))
}

func (m *Module) ListCareExitSourceRemovals(ctx context.Context, studentIDs []int64) ([]CareExitSourceRemoval, error) {
	return m.engine.ListCareExitSourceRemovals(ctx, uniquePositive(studentIDs))
}

func (m *Module) RecordCareExitRemovals(ctx context.Context, values []CareExitRemoval) error {
	for _, value := range values {
		if !validCareExitRemoval(value) {
			return invalid(ErrInvalidCareExit, "care exit removal is invalid")
		}
	}
	return m.engine.RecordCareExitRemovals(ctx, values)
}

func validCareExitRemoval(value CareExitRemoval) bool {
	if value.StudentID <= 0 {
		return false
	}
	switch value.Kind {
	case CareExitRemovalRoster:
		return value.InstanceID != nil && value.Status != nil
	case CareExitRemovalBooking:
		return value.EnrollmentID != nil && (!value.WasDeleted || value.ActivityGroupID != nil && value.ValidFrom != nil)
	default:
		return false
	}
}

func (m *Module) RecordCareExitSourceRemovals(ctx context.Context, values []CareExitSourceRemoval) error {
	for _, value := range values {
		if value.StudentID <= 0 || value.SourceRowID <= 0 || !validCareExitSource(value.Kind) || len(value.Snapshot) == 0 || !json.Valid(value.Snapshot) {
			return invalid(ErrInvalidCareExit, "care exit source removal is invalid")
		}
	}
	return m.engine.RecordCareExitSourceRemovals(ctx, values)
}

func validCareExitSource(kind string) bool {
	switch kind {
	case CareExitSourceBooking, CareExitSourcePickupSchedule, CareExitSourcePickupException,
		CareExitSourceArrivalSchedule, CareExitSourceArrivalException:
		return true
	default:
		return false
	}
}

func (m *Module) DiscardCareExitRemovals(ctx context.Context, studentIDs []int64) error {
	return m.engine.DiscardCareExitRemovals(ctx, uniquePositive(studentIDs))
}

func normalizeCareExit(value *CareExit) error {
	if value.StudentID <= 0 {
		return invalid(ErrInvalidCareExit, "student ID is required")
	}
	if value.Reason != CareExitReasonMovedAway && value.Reason != CareExitReasonNoCareNeed && value.Reason != CareExitReasonOther {
		return ErrCareExitInvalidReason
	}
	if value.ReasonNote != nil {
		trimmed := strings.TrimSpace(*value.ReasonNote)
		if trimmed == "" {
			value.ReasonNote = nil
		} else {
			if utf8.RuneCountInString(trimmed) > MaxCareExitNoteLen {
				return ErrCareExitNoteTooLong
			}
			value.ReasonNote = &trimmed
		}
	}
	if value.Reason == CareExitReasonOther && value.ReasonNote == nil {
		return ErrCareExitNoteRequired
	}
	if value.Reason != CareExitReasonOther && value.ReasonNote != nil {
		return ErrCareExitNoteNotAllowed
	}
	return nil
}

func validateCareDocument(value CareDocument) error {
	if value.StudentID <= 0 || strings.TrimSpace(value.Category) == "" || strings.TrimSpace(value.FilenameDisplay) == "" || strings.TrimSpace(value.FilenameStored) == "" || value.SizeBytes < 0 || strings.TrimSpace(value.ContentType) == "" || value.UploadedBy <= 0 {
		return invalid(ErrInvalidCareDocument, "care document metadata is invalid")
	}
	return nil
}
