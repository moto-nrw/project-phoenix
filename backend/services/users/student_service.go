package users

import (
	"context"
	"database/sql"
	"errors"
	"sort"

	"github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

// StudentService exposes student persistence operations to the api layer
// (issue #584: handlers must not hold repositories). CONTRACT: repository
// results and errors are returned VERBATIM — the handlers keep their existing
// transaction wrappers, validation, and error-to-status mapping, so responses
// stay byte-identical. See the rule-8 deviation note in the PR description.
type StudentService interface {
	// ListWithOptions retrieves students matching the query options.
	ListWithOptions(ctx context.Context, options *base.QueryOptions) ([]*userModels.Student, error)

	// CountWithOptions counts students matching the query options.
	CountWithOptions(ctx context.Context, options *base.QueryOptions) (int, error)

	// ListSchoolClasses retrieves all distinct non-empty school classes.
	ListSchoolClasses(ctx context.Context) ([]string, error)

	// GetByIDForUpdate retrieves a student with SELECT … FOR UPDATE row locking.
	GetByIDForUpdate(ctx context.Context, id int64) (*userModels.Student, error)

	// Create persists a new student.
	Create(ctx context.Context, student *userModels.Student) error

	// Update persists changes to a student.
	Update(ctx context.Context, student *userModels.Student) error

	// Delete removes a student.
	Delete(ctx context.Context, id int64) error

	// LockPhotoFeature acquires the per-tenant photo-feature advisory lock.
	LockPhotoFeature(ctx context.Context) error

	// ListPrivacyConsents retrieves a student's privacy consents.
	ListPrivacyConsents(ctx context.Context, studentID int64) ([]*userModels.PrivacyConsent, error)

	// CreatePrivacyConsent persists a new privacy consent.
	CreatePrivacyConsent(ctx context.Context, consent *userModels.PrivacyConsent) error

	// UpdatePrivacyConsent persists changes to a privacy consent.
	UpdatePrivacyConsent(ctx context.Context, consent *userModels.PrivacyConsent) error

	// GetByIDs retrieves several students in one query, keyed by id.
	GetByIDs(ctx context.Context, ids []int64) (map[int64]*userModels.Student, error)

	// ListCompanions returns the children this child walks home with
	// ("läuft mit"), folded per companion with their weekdays and names.
	ListCompanions(ctx context.Context, studentID int64) ([]userModels.CompanionLink, error)

	// TrimCompanionsToDays drops every companion link on a weekday the child's
	// departure plan no longer allows, and reports the links that remain.
	TrimCompanionsToDays(ctx context.Context, studentID int64, allowedDays map[string]bool) ([]userModels.CompanionLink, error)

	// CheckCompanionConflicts reports the same conflicts as ReplaceCompanions
	// without writing anything, so a caller can refuse before its first write.
	CheckCompanionConflicts(ctx context.Context, studentID int64, update CompanionUpdate) ([]CompanionConflict, error)

	// ReplaceCompanions makes the given links the child's complete companion
	// set. It returns the companions whose own departure plan does not allow
	// the requested days; unless the update says to extend them, nothing is
	// written in that case.
	ReplaceCompanions(ctx context.Context, studentID int64, update CompanionUpdate) ([]CompanionConflict, error)

	// CompanionIDsForWeekday bulk-resolves, per student, who they walk home
	// with on the given weekday (1..5). Drives the Kindersuche grouping.
	CompanionIDsForWeekday(ctx context.Context, studentIDs []int64, weekday int) (map[int64][]int64, error)
}

type studentService struct {
	studentRepo        userModels.StudentRepository
	privacyConsentRepo userModels.PrivacyConsentRepository
	companionRepo      userModels.StudentCompanionRepository
}

// NewStudentService creates a StudentService backed by the student-domain
// repositories.
func NewStudentService(
	studentRepo userModels.StudentRepository,
	privacyConsentRepo userModels.PrivacyConsentRepository,
	companionRepo userModels.StudentCompanionRepository,
) StudentService {
	return &studentService{
		studentRepo:        studentRepo,
		privacyConsentRepo: privacyConsentRepo,
		companionRepo:      companionRepo,
	}
}

func (s *studentService) ListWithOptions(ctx context.Context, options *base.QueryOptions) ([]*userModels.Student, error) {
	return s.studentRepo.ListWithOptions(ctx, options)
}

func (s *studentService) CountWithOptions(ctx context.Context, options *base.QueryOptions) (int, error) {
	return s.studentRepo.CountWithOptions(ctx, options)
}

func (s *studentService) ListSchoolClasses(ctx context.Context) ([]string, error) {
	return s.studentRepo.ListSchoolClasses(ctx)
}

func (s *studentService) GetByIDForUpdate(ctx context.Context, id int64) (*userModels.Student, error) {
	return s.studentRepo.FindByIDForUpdate(ctx, id)
}

func (s *studentService) Create(ctx context.Context, student *userModels.Student) error {
	return s.studentRepo.Create(ctx, student)
}

func (s *studentService) Update(ctx context.Context, student *userModels.Student) error {
	return s.studentRepo.Update(ctx, student)
}

func (s *studentService) Delete(ctx context.Context, id int64) error {
	return s.studentRepo.Delete(ctx, id)
}

func (s *studentService) LockPhotoFeature(ctx context.Context) error {
	return s.studentRepo.LockPhotoFeature(ctx)
}

func (s *studentService) ListPrivacyConsents(ctx context.Context, studentID int64) ([]*userModels.PrivacyConsent, error) {
	return s.privacyConsentRepo.FindByStudentID(ctx, studentID)
}

func (s *studentService) CreatePrivacyConsent(ctx context.Context, consent *userModels.PrivacyConsent) error {
	return s.privacyConsentRepo.Create(ctx, consent)
}

func (s *studentService) UpdatePrivacyConsent(ctx context.Context, consent *userModels.PrivacyConsent) error {
	return s.privacyConsentRepo.Update(ctx, consent)
}

// MaxStudentCompanions caps how many children one child may be linked to. A
// Laufgemeinschaft is a handful of neighbours walking home together; a request
// with more than this is a client bug or an attempt to use the field as a group
// list, and both should fail loudly rather than produce an unreadable grouping.
const MaxStudentCompanions = 10

func (s *studentService) GetByIDs(ctx context.Context, ids []int64) (map[int64]*userModels.Student, error) {
	if len(ids) == 0 {
		return map[int64]*userModels.Student{}, nil
	}
	return s.studentRepo.FindByIDs(ctx, ids)
}

func (s *studentService) ListCompanions(ctx context.Context, studentID int64) ([]userModels.CompanionLink, error) {
	if studentID <= 0 {
		return nil, ErrStudentNotFound
	}
	return s.companionRepo.ListLinksForStudent(ctx, studentID)
}

// TrimCompanionsToDays removes the links that the child's departure plan no
// longer covers.
//
// It exists for the update that narrows the plan without touching the link
// list: dropping "Anderes Kind" from Tuesday has to drop the Tuesday edges too,
// or the Kindersuche keeps grouping the children on a day the Stammdaten
// forbid. Only ever removes, so no companion's plan is consulted — a day that
// was legal before cannot become illegal by disappearing.
func (s *studentService) TrimCompanionsToDays(ctx context.Context, studentID int64, allowedDays map[string]bool) ([]userModels.CompanionLink, error) {
	if studentID <= 0 {
		return nil, ErrStudentNotFound
	}

	links, err := s.companionRepo.ListLinksForStudent(ctx, studentID)
	if err != nil {
		return nil, err
	}

	trimmed := make([]userModels.CompanionLink, 0, len(links))
	changed := false
	for _, link := range links {
		kept := make([]string, 0, len(link.Weekdays))
		for _, day := range link.Weekdays {
			if allowedDays[day] {
				kept = append(kept, day)
			}
		}
		if len(kept) != len(link.Weekdays) {
			changed = true
		}
		if len(kept) == 0 {
			continue
		}
		link.Weekdays = kept
		trimmed = append(trimmed, link)
	}
	if !changed {
		return links, nil
	}

	edges, _, err := buildCompanionEdges(studentID, trimmed)
	if err != nil {
		return nil, err
	}
	if err := s.companionRepo.ReplaceForStudent(ctx, studentID, edges); err != nil {
		return nil, err
	}
	return trimmed, nil
}

// CompanionUpdate is the input for ReplaceCompanions.
type CompanionUpdate struct {
	// Links is the child's complete companion list. Empty clears every link.
	Links []userModels.CompanionLink

	// AccompaniedDays are the weekday keys on which the SUBJECT child may leave
	// with another child. A caller that changes the departure plan in the same
	// request passes the NEW plan's days here; nil means "read the stored plan".
	AccompaniedDays map[string]bool

	// ExtendCompanionPlans permits widening a companion's own allowed departure
	// modes so the requested days become legal for them too. False (the default)
	// reports the mismatch instead, so the caller can ask a human first —
	// widening another child's departure permission is a safety-relevant write
	// and must never happen silently.
	ExtendCompanionPlans bool
}

// CompanionConflict names a companion whose departure plan does not permit
// leaving with another child on the requested weekdays.
//
// Deliberately id + weekdays only: the caller just picked this child and
// already holds the name, so re-fetching it here would buy nothing but an
// extra query.
type CompanionConflict struct {
	StudentID int64    `json:"student_id"`
	Weekdays  []string `json:"weekdays"`
}

// ReplaceCompanions validates the submitted "läuft mit" list and writes it as
// the child's complete companion set.
//
// The write is symmetric by construction: each link becomes one undirected edge,
// so adding Tom to Lina's card is the same row that shows Lina on Tom's card.
// The flip side is that replacing Lina's list only touches edges that TOUCH
// Lina — a link between Tom and Mia is left alone.
func (s *studentService) ReplaceCompanions(ctx context.Context, studentID int64, update CompanionUpdate) ([]CompanionConflict, error) {
	edges, conflicts, err := s.validateCompanionUpdate(ctx, studentID, update)
	if err != nil {
		return nil, err
	}

	if len(conflicts) > 0 {
		if !update.ExtendCompanionPlans {
			return conflicts, nil
		}
		// Ascending id order (validateCompanionUpdate sorts), so two requests
		// widening overlapping companions take the row locks in the same order.
		for _, conflict := range conflicts {
			if err := s.extendAccompaniedDays(ctx, conflict.StudentID, conflict.Weekdays); err != nil {
				return nil, err
			}
		}
	}

	return nil, s.companionRepo.ReplaceForStudent(ctx, studentID, edges)
}

// CheckCompanionConflicts answers the same question as ReplaceCompanions
// WITHOUT writing anything.
//
// It exists because the caller has to know EVERY possible rejection before it
// starts writing: the request runs inside the middleware's tenant transaction,
// which commits on any non-5xx response, so a 4xx raised halfway through would
// leave the earlier writes of that request committed. It therefore runs the
// full validation — shape, count, subject weekdays, companion existence — and
// not just the confirmable conflicts, and it does so regardless of
// ExtendCompanionPlans: a confirmed retry can still be rejected for one of the
// other reasons.
func (s *studentService) CheckCompanionConflicts(ctx context.Context, studentID int64, update CompanionUpdate) ([]CompanionConflict, error) {
	_, conflicts, err := s.validateCompanionUpdate(ctx, studentID, update)
	return conflicts, err
}

// validateCompanionUpdate runs every check ReplaceCompanions performs and
// returns the edges to write plus the companions whose own departure plan does
// not allow the requested days. It writes nothing, so the read-only
// CheckCompanionConflicts and the writing ReplaceCompanions cannot drift apart.
func (s *studentService) validateCompanionUpdate(ctx context.Context, studentID int64, update CompanionUpdate) ([]*userModels.StudentCompanion, []CompanionConflict, error) {
	if studentID <= 0 {
		return nil, nil, ErrStudentNotFound
	}
	if len(update.Links) > MaxStudentCompanions {
		return nil, nil, ErrTooManyCompanions
	}

	subject, err := s.studentRepo.FindByID(ctx, studentID)
	if err != nil {
		return nil, nil, err
	}
	if subject == nil {
		return nil, nil, ErrStudentNotFound
	}

	edges, companionDays, err := buildCompanionEdges(studentID, update.Links)
	if err != nil {
		return nil, nil, err
	}

	// The subject's own plan gates which days may carry a link at all. A day the
	// plan does not allow is a client bug (the UI only offers allowed days), so
	// it is a plain rejection rather than a conflict to confirm.
	allowedDays := update.AccompaniedDays
	if allowedDays == nil {
		allowedDays = userModels.AccompaniedWeekdays(subject.AllowedDepartureModes, subject.DepartureDays)
	}
	for _, edge := range edges {
		day := userModels.CompanionWeekdayKeys[edge.Weekday]
		if !allowedDays[day] {
			return nil, nil, ErrCompanionDayNotAllowed
		}
	}

	if len(companionDays) == 0 {
		return edges, nil, nil
	}

	conflicts, err := s.resolveCompanions(ctx, companionDays)
	if err != nil {
		return nil, nil, err
	}
	return edges, conflicts, nil
}

// resolveCompanions loads the requested companions and reports which of them
// may not leave with another child on the requested days, in ascending id
// order.
func (s *studentService) resolveCompanions(ctx context.Context, companionDays map[int64][]string) ([]CompanionConflict, error) {
	companionIDs := make([]int64, 0, len(companionDays))
	for id := range companionDays {
		companionIDs = append(companionIDs, id)
	}
	sort.Slice(companionIDs, func(i, j int) bool { return companionIDs[i] < companionIDs[j] })

	// Tenant-filtered, so an id from another school simply does not come back —
	// which is exactly the existence check we need before persisting a link.
	// Read without a lock on purpose: this also runs on the read-only check
	// path. Every WRITE against a companion re-reads it under its row lock (see
	// extendAccompaniedDays), so a stale row here can only cost a redundant
	// confirmation, never a lost update.
	found, err := s.studentRepo.FindByIDs(ctx, companionIDs)
	if err != nil {
		return nil, err
	}

	var conflicts []CompanionConflict
	for _, id := range companionIDs {
		companion := found[id]
		if companion == nil {
			return nil, ErrCompanionNotFound
		}
		if missing := missingAccompaniedDays(companion, companionDays[id]); len(missing) > 0 {
			conflicts = append(conflicts, CompanionConflict{StudentID: id, Weekdays: missing})
		}
	}
	return conflicts, nil
}

// buildCompanionEdges validates the submitted list and turns it into edges,
// also returning the requested weekdays per companion.
func buildCompanionEdges(studentID int64, links []userModels.CompanionLink) ([]*userModels.StudentCompanion, map[int64][]string, error) {
	edges := make([]*userModels.StudentCompanion, 0, len(links)*len(userModels.PickupDayOrder))
	byCompanion := make(map[int64][]string, len(links))

	for _, link := range links {
		if link.CompanionStudentID == studentID {
			return nil, nil, userModels.ErrCompanionSelfLink
		}
		if _, seen := byCompanion[link.CompanionStudentID]; seen {
			return nil, nil, ErrDuplicateCompanion
		}
		if len(link.Weekdays) == 0 {
			return nil, nil, ErrCompanionWeekdayRequired
		}

		days := make([]string, 0, len(link.Weekdays))
		seenDays := make(map[int]bool, len(link.Weekdays))
		for _, day := range link.Weekdays {
			weekday, err := userModels.CompanionWeekdayNumber(day)
			if err != nil {
				return nil, nil, err
			}
			if seenDays[weekday] {
				continue
			}
			seenDays[weekday] = true

			edge, err := userModels.NewStudentCompanion(studentID, link.CompanionStudentID, weekday)
			if err != nil {
				return nil, nil, err
			}
			edges = append(edges, edge)
			days = append(days, day)
		}
		byCompanion[link.CompanionStudentID] = days
	}
	return edges, byCompanion, nil
}

// missingAccompaniedDays returns the requested weekdays the companion's own
// departure plan does not (yet) allow, in Mon..Fri order.
func missingAccompaniedDays(companion *userModels.Student, requested []string) []string {
	allowed := userModels.AccompaniedWeekdays(companion.AllowedDepartureModes, companion.DepartureDays)
	wanted := make(map[string]bool, len(requested))
	for _, day := range requested {
		wanted[day] = true
	}

	var missing []string
	for _, day := range userModels.PickupDayOrder {
		if wanted[day] && !allowed[day] {
			missing = append(missing, day)
		}
	}
	return missing
}

// extendAccompaniedDays widens one companion's allowed departure modes so the
// requested days permit leaving with another child. Purely additive — the bus
// and pickup permissions of those days stay untouched.
//
// The row is re-read under SELECT … FOR UPDATE rather than reused from the
// unlocked snapshot resolveCompanions took: widening writes the WHOLE student
// row, so a sick/group/consent/photo change that another request committed in
// between would be silently rolled back by a stale snapshot write.
func (s *studentService) extendAccompaniedDays(ctx context.Context, companionID int64, days []string) error {
	companion, err := s.studentRepo.FindByIDForUpdate(ctx, companionID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Deleted between the check and the lock.
			return ErrCompanionNotFound
		}
		return err
	}
	if companion == nil {
		return ErrCompanionNotFound
	}

	// Re-derive against the locked row: a concurrent edit may already have
	// widened some (or all) of these days, and re-adding what is there would
	// only widen the write for no reason.
	missing := missingAccompaniedDays(companion, days)
	if len(missing) == 0 {
		return nil
	}

	companion.AllowedDepartureModes = userModels.WithAccompaniedDays(
		companion.AllowedDepartureModes, missing,
	)
	// The link we are about to write IS the "mit wem" detail, so the
	// accompanied-needs-a-note invariant is satisfied without inventing text.
	companion.DepartureCompanionLinked = true
	return s.studentRepo.Update(ctx, companion)
}

func (s *studentService) CompanionIDsForWeekday(ctx context.Context, studentIDs []int64, weekday int) (map[int64][]int64, error) {
	return s.companionRepo.CompanionIDsForWeekday(ctx, studentIDs, weekday)
}
