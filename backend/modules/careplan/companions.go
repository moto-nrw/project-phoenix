package careplan

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"
)

// The "läuft mit" graph (#3350, #3427): which children walk home together, on
// which weekdays, and what a change to one child's list means for the
// others. users.student_companions belongs to Care Plan; the child row and
// its departure plan belong to People Directory and are reached through a
// consumer-owned port.

// MaxStudentCompanions caps how many children one child may be linked to. A
// Laufgemeinschaft is a handful of neighbours walking home together; a
// request with more than this is a client bug or an attempt to use the field
// as a group list, and both should fail loudly.
const MaxStudentCompanions = 10

// CompanionWeekdays are the weekday keys a link may carry, Mon..Fri, in
// display order. The stored weekday is the 1-based position.
var CompanionWeekdays = []string{"mon", "tue", "wed", "thu", "fri"}

// The companion sentinels. The German messages reach the user unchanged.
var (
	// ErrCompanionStudentNotFound indicates the child being edited does not
	// exist in this tenant.
	ErrCompanionStudentNotFound = errors.New("student not found")
	// ErrCompanionNotFound indicates a submitted companion is not a child of
	// this school.
	ErrCompanionNotFound = errors.New("Ein ausgewähltes Kind wurde nicht gefunden.") //nolint:staticcheck // ST1005: user-facing German message
	// ErrDuplicateCompanion indicates the same child was submitted twice.
	ErrDuplicateCompanion = errors.New("Ein Kind kann nur einmal in der Laufgemeinschaft stehen.") //nolint:staticcheck // ST1005: user-facing German message
	// ErrCompanionWeekdayRequired indicates a link without any weekday: a
	// link that applies on no day is not a link.
	ErrCompanionWeekdayRequired = errors.New("Bitte mindestens einen Wochentag auswählen.") //nolint:staticcheck // ST1005: user-facing German message
	// ErrCompanionDayNotAllowed indicates a link on a weekday the child's own
	// departure plan does not permit leaving with another child.
	ErrCompanionDayNotAllowed = errors.New("An diesem Tag ist \"Anderes Kind\" als Heimweg nicht erlaubt.") //nolint:staticcheck // ST1005: user-facing German message
	// ErrTooManyCompanions indicates more companions than MaxStudentCompanions.
	ErrTooManyCompanions = errors.New("Es können höchstens 10 Kinder verknüpft werden.") //nolint:staticcheck // ST1005: user-facing German message
	// ErrCompanionAtLimit indicates the cap is already reached at the FAR end
	// of a submitted link: an edge counts for both children.
	ErrCompanionAtLimit = errors.New("Ein ausgewähltes Kind ist bereits mit 10 Kindern verknüpft.") //nolint:staticcheck // ST1005: user-facing German message
	// ErrCompanionSelfLink indicates a child linked to itself.
	ErrCompanionSelfLink = errors.New("a child cannot be its own departure companion")
	// ErrCompanionInvalidWeekday indicates a weekday outside Mon..Fri.
	ErrCompanionInvalidWeekday = errors.New("companion weekday must be one of mon/tue/wed/thu/fri")
	// ErrCompanionStudentIDRequired indicates a link without a usable child id.
	ErrCompanionStudentIDRequired = errors.New("Bitte ein Kind für die Laufgemeinschaft auswählen.") //nolint:staticcheck // ST1005: user-facing German message
	// ErrCompanionWouldLoseDeparture indicates that removing a link would leave
	// the OTHER child with an accompanied ("Anderes Kind") departure plan and
	// no remaining detail. Refused rather than silently narrowing that child's
	// plan: one child's edit must never change another child's departure
	// permissions.
	ErrCompanionWouldLoseDeparture = errors.New("Ein verknüpftes Kind hätte danach keine Angabe mehr dazu, mit wem es nach Hause geht. Bitte zuerst den Heimweg dieses Kindes anpassen.") //nolint:staticcheck // ST1005: user-facing German message
	// ErrCompanionLockBusy indicates a linked child's row was locked by another
	// edit that this transaction may not wait for without risking a deadlock.
	// Transient: the same request succeeds on retry.
	ErrCompanionLockBusy = errors.New("Ein verknüpftes Kind wird gerade an anderer Stelle bearbeitet. Bitte in einem Moment erneut speichern.") //nolint:staticcheck // ST1005: user-facing German message
	// ErrCompanionsChanged indicates the submitted list was built on a
	// snapshot that is no longer the stored one. The list REPLACES the stored
	// one, so without this check two staff members editing the same child
	// would silently delete each other's links. Retriable after a reload.
	ErrCompanionsChanged = errors.New("Die Laufgemeinschaft dieses Kindes wurde zwischenzeitlich geändert. Bitte neu laden und noch einmal speichern.") //nolint:staticcheck // ST1005: user-facing German message
	// ErrCompanionExtensionNotAuthorized indicates the write path was asked to
	// widen a companion's departure plan the caller was never authorized for.
	// Not user-facing: it can only fire if the authorization pass and the
	// write pass disagree, so it must surface as a 500 and roll back.
	ErrCompanionExtensionNotAuthorized = errors.New("companion departure-plan extension was not authorized")
)

// CompanionUpdate is the input of ReplaceCompanions.
type CompanionUpdate struct {
	// Links is the child's complete companion list. Empty clears every link.
	Links []CompanionLink
	// ExpectedFingerprint is the CompanionLinksFingerprint of the list the
	// client built Links from. The write is refused with ErrCompanionsChanged
	// when the stored links no longer match. nil means "no expectation" and is
	// for system callers that derive the list from the stored state.
	ExpectedFingerprint *string
	// AccompaniedDays are the weekday keys on which the SUBJECT may leave with
	// another child. A caller that changes the departure plan in the same
	// request passes the NEW plan's days; nil means "read the stored plan".
	AccompaniedDays map[string]bool
	// ExtendCompanionPlans permits widening a companion's own allowed
	// departure modes so the requested days become legal for them too. False
	// reports the mismatch instead, so the caller can ask a human first.
	ExtendCompanionPlans bool
	// AuthorizedExtensions are the WEEKDAYS per companion the caller was both
	// authorized and explicitly confirmed to widen. nil means "no restriction"
	// and is for system callers only.
	AuthorizedExtensions map[int64]map[string]bool
	// ActorAccountID is the account that confirmed widening another child's
	// departure plan; the change history records it.
	ActorAccountID int64
}

// CompanionConflict names a companion whose departure plan does not permit
// leaving with another child on the requested weekdays.
type CompanionConflict struct {
	StudentID int64    `json:"student_id"`
	Weekdays  []string `json:"weekdays"`
}

// CompanionLocks is the shared lock protocol every writer of companion edges
// follows, so requests touching an overlapping set of children cannot
// deadlock each other.
type CompanionLocks interface {
	// LockStudentsForUpdate takes the row locks of the given children in one
	// ascending-id pass.
	LockStudentsForUpdate(ctx context.Context, ids []int64) error
	// LockStudentsForUpdateBelow takes the row locks knowing this transaction
	// already holds locks up to maxHeldID. Ids below it would invert the order
	// and are taken without waiting, returning ErrCompanionLockBusy.
	LockStudentsForUpdateBelow(ctx context.Context, ids []int64, maxHeldID int64) error
	// LockCompanionGraph locks the subjects, the additional ids the caller
	// already knows about, and the far end of every stored edge.
	LockCompanionGraph(ctx context.Context, subjectIDs []int64, additional []int64) error
}

// CompanionQueries read the companion graph.
type CompanionQueries interface {
	// ListCompanions returns the children this child walks home with, folded
	// per companion with their weekdays and names.
	ListCompanions(ctx context.Context, studentID int64) ([]CompanionLink, error)
	// ListCompanionsForStudents is the bulk form of ListCompanions.
	ListCompanionsForStudents(ctx context.Context, studentIDs []int64) (map[int64][]CompanionLink, error)
	// ListCompanionIDs returns the distinct ids of the children this child is
	// linked to, across all weekdays.
	ListCompanionIDs(ctx context.Context, studentID int64) ([]int64, error)
	// CompanionIDsForWeekday bulk-resolves, per student, who they walk home
	// with on the given weekday (1..5).
	CompanionIDsForWeekday(ctx context.Context, studentIDs []int64, weekday int) (map[int64][]int64, error)
}

// CompanionCommands change the companion graph.
type CompanionCommands interface {
	// TrimCompanionsToDays drops every link on a weekday the child's
	// departure plan no longer allows and reports the links that remain.
	TrimCompanionsToDays(ctx context.Context, studentID int64, allowedDays map[string]bool) ([]CompanionLink, error)
	// CheckCompanionTrim reports, without writing, whether the trim would
	// strand a former companion. A nil or empty allowedDays drops every link.
	CheckCompanionTrim(ctx context.Context, studentID int64, allowedDays map[string]bool) error
	// CheckCompanionTrimForPlan is CheckCompanionTrim for a trim a submitted
	// departure plan drives; it also verifies the caller knew which links the
	// plan is about to delete.
	CheckCompanionTrimForPlan(ctx context.Context, studentID int64, allowedDays map[string]bool, expectedFingerprint *string) error
	// CheckCompanionConflicts reports the same conflicts as ReplaceCompanions
	// without writing anything.
	CheckCompanionConflicts(ctx context.Context, studentID int64, update CompanionUpdate) ([]CompanionConflict, error)
	// ReplaceCompanions makes the given links the child's complete companion
	// set. It returns the companions whose own departure plan does not allow
	// the requested days; unless the update says to extend them, nothing is
	// written in that case.
	ReplaceCompanions(ctx context.Context, studentID int64, update CompanionUpdate) ([]CompanionConflict, error)
}

// StudentCompanions is the companion capability.
type StudentCompanions interface {
	CompanionLocks
	CompanionQueries
	CompanionCommands
}

// CompanionLinksFingerprint is an order-independent fingerprint of a
// companion list: the state a client read, in one comparable string. The
// format is MIRRORED by companionsFingerprint() in
// frontend/src/lib/student-companion-api.ts: "<id>:<mon,tue,…>" per link,
// weekdays in Mon..Fri order, links sorted as strings and joined with "|".
// Change one and you must change the other.
func CompanionLinksFingerprint(links []CompanionLink) string {
	entries := make([]string, 0, len(links))
	for _, link := range links {
		requested := make(map[string]bool, len(link.Weekdays))
		for _, day := range link.Weekdays {
			requested[day] = true
		}
		days := make([]string, 0, len(CompanionWeekdays))
		for _, day := range CompanionWeekdays {
			if requested[day] {
				days = append(days, day)
			}
		}
		entries = append(entries, strconv.FormatInt(link.CompanionStudentID, 10)+":"+strings.Join(days, ","))
	}
	sort.Strings(entries)
	return strings.Join(entries, "|")
}
