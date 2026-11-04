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
	// CareExitSourceDepartureException is a future day Abholart (#3945).
	CareExitSourceDepartureException = "departure_exception"
)

var (
	ErrCareExitInvalidReason  = errors.New("care plan: invalid care exit reason")
	ErrCareExitNoteRequired   = errors.New("care plan: care exit reason note is required")
	ErrCareExitNoteNotAllowed = errors.New("care plan: care exit reason note is not allowed")
	ErrCareExitNoteTooLong    = errors.New("die Begründung ist zu lang. Bitte kürzen Sie sie")
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
	// The accident report of an unfallanzeige document (#4771): what was
	// recorded, the report it corrects, and how and when it was signed. A
	// report without a paper copy has no stored file.
	AccidentReport       *AccidentReport
	AccidentSupersedesID *int64
	AccidentSignatureID  *int64
	AccidentSignedAt     *time.Time
	AccidentSignedBy     *int64
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

// CareRecordsQuery reads the care records of a child: its care exits, its
// companions and its care documents.
type CareRecordsQuery interface {
	CareExitsQuery
	CompanionsQuery
	CareDocumentsQuery
}

// CareExitsQuery reads care exits and the removals recorded for them.
type CareExitsQuery interface {
	FindCareExits(context.Context, []int64) (map[int64]CareExit, error)
	ListCareExitRemovals(context.Context, []int64) ([]CareExitRemoval, error)
	ListCareExitSourceRemovals(context.Context, []int64) ([]CareExitSourceRemoval, error)
}

// CompanionsQuery reads the companion edges between children.
type CompanionsQuery interface {
	ListCompanionEdges(context.Context, int64) ([]CompanionEdge, error)
	ListCompanionLinks(context.Context, []int64) (map[int64][]CompanionLink, error)
	CompanionCountsExcluding(context.Context, []int64, int64) (map[int64]int, error)
	CompanionDaysCoveredExcluding(context.Context, []int64, int64) (map[int64]map[string]bool, error)
	CompanionIDsForWeekday(context.Context, []int64, int) (map[int64][]int64, error)
	CompanionWeekdays(context.Context, int64) ([]int, error)
	CountCompanionLinks(context.Context, int64) (int, error)
}

// CareDocumentsQuery reads care documents and their pending file cleanups.
type CareDocumentsQuery interface {
	FindCareDocument(context.Context, int64, int64, bool) (CareDocument, error)
	ListCareDocuments(context.Context, int64, []string) ([]CareDocument, error)
	ListPendingCareDocumentCleanup(context.Context, int64) ([]CareDocument, error)
	ListDeletedCareDocuments(context.Context, int64, []string) ([]CareDocument, error)
	ListCareDocumentCleanups(context.Context, *int64) ([]CareDocumentCleanup, error)
}

// CareRecordsCommand writes the care records of a child.
type CareRecordsCommand interface {
	CareExitsCommand
	CompanionsCommand
	CareDocumentsCommand
}

// CareExitsCommand writes care exits and the removals recorded for them.
type CareExitsCommand interface {
	UpsertCareExit(context.Context, CareExit) error
	DeleteCareExits(context.Context, []int64) error
	RecordCareExitRemovals(context.Context, []CareExitRemoval) error
	RecordCareExitSourceRemovals(context.Context, []CareExitSourceRemoval) error
	DiscardCareExitRemovals(context.Context, []int64) error
}

// CompanionsCommand writes the companion edges between children.
type CompanionsCommand interface {
	ReplaceCompanionEdges(context.Context, int64, []CompanionEdge) error
	DeleteCompanionEdges(context.Context, []int64) error
}

// CareDocumentsCommand writes care documents and drives their file cleanup.
type CareDocumentsCommand interface {
	CreateCareDocument(context.Context, CareDocument) (CareDocument, error)
	// UpdateOpenCareAccidentReport rewrites the report, signature and paper
	// copy of an accident report that is neither signed nor deleted; a report
	// that is no longer open is ErrAccidentReportSigned.
	UpdateOpenCareAccidentReport(context.Context, CareDocument) (CareDocument, error)
	SoftDeleteCareDocument(context.Context, int64, int64) (time.Time, error)
	MarkCareDocumentFileDeleted(context.Context, int64) error
	QueueCareDocumentCleanup(context.Context, CareDocumentCleanup) (CareDocumentCleanup, error)
	CompleteCareDocumentCleanup(context.Context, int64) error
	CompleteCareDocumentCleanupByFilename(context.Context, string) error
	ActivateCareDocumentCleanup(context.Context, string) error
}

// NormalizeCareExitReason checks one exit reason and its note and returns the
// canonical note: trimmed, nil when blank. Only "other" carries a note, and
// it must.
func NormalizeCareExitReason(reason string, note *string) (*string, error) {
	if reason != CareExitReasonMovedAway && reason != CareExitReasonNoCareNeed && reason != CareExitReasonOther {
		return nil, ErrCareExitInvalidReason
	}
	var normalized *string
	if note != nil {
		if trimmed := strings.TrimSpace(*note); trimmed != "" {
			if utf8.RuneCountInString(trimmed) > MaxCareExitNoteLen {
				return nil, ErrCareExitNoteTooLong
			}
			normalized = &trimmed
		}
	}
	if reason == CareExitReasonOther && normalized == nil {
		return nil, ErrCareExitNoteRequired
	}
	if reason != CareExitReasonOther && normalized != nil {
		return nil, ErrCareExitNoteNotAllowed
	}
	return normalized, nil
}

// ValidateCareDocument checks that one metadata row is storable.
func ValidateCareDocument(value CareDocument) error {
	// Only an accident report may be stored without a file: its PDF is
	// rendered from the report.
	fileMissing := strings.TrimSpace(value.FilenameStored) == "" && value.AccidentReport == nil
	if value.StudentID <= 0 || strings.TrimSpace(value.Category) == "" || strings.TrimSpace(value.FilenameDisplay) == "" || fileMissing || value.SizeBytes < 0 || strings.TrimSpace(value.ContentType) == "" || value.UploadedBy <= 0 {
		return &InvalidError{Kind: ErrInvalidCareDocument, Reason: "care document metadata is invalid"}
	}
	if value.AccidentReport != nil && value.Category != StudentDocumentCategoryUnfallanzeige {
		return &InvalidError{Kind: ErrInvalidCareDocument, Reason: "only an unfallanzeige document carries an accident report"}
	}
	return nil
}

// Other returns the far endpoint when studentID belongs to this edge.
func (c *CompanionEdge) Other(studentID int64) (int64, bool) {
	switch studentID {
	case c.StudentLowID:
		return c.StudentHighID, true
	case c.StudentHighID:
		return c.StudentLowID, true
	default:
		return 0, false
	}
}

func (c *CompanionEdge) GetTenantID() int64 { return c.TenantID }
