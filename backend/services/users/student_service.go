package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"

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

	// LockStudentsForUpdate takes the row locks of the given students in one
	// deterministic order, so requests touching an overlapping set cannot
	// deadlock each other.
	LockStudentsForUpdate(ctx context.Context, ids []int64) error

	// LockStudentsForUpdateBelow takes the row locks of the given students
	// knowing this transaction ALREADY holds locks up to maxHeldID. Ids at or
	// above that bound still follow the ascending order and may wait; ids below
	// it would invert the order and are taken without waiting, returning
	// ErrCompanionLockBusy rather than risking a deadlock.
	LockStudentsForUpdateBelow(ctx context.Context, ids []int64, maxHeldID int64) error

	// LockCompanionGraph is the shared lock protocol for every transaction that
	// can add or remove "läuft mit" edges: it locks the subjects, the extra ids
	// the caller already knows about (a submitted companion list), and the far
	// end of every stored edge, in one ascending-id pass.
	LockCompanionGraph(ctx context.Context, subjectIDs []int64, additional []int64) error

	// VerifyCompanionStrandingBatch decides the deferred stranding verdicts of a
	// coordinated multi-child write (a userModels.CompanionStrandingBatch opened
	// on the context) against the final state of every child in it. Returns
	// ErrCompanionWouldLoseDeparture when a linked child would really be left
	// without a "mit wem" detail; a no-op without an open batch.
	VerifyCompanionStrandingBatch(ctx context.Context) error

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

	// ListCompanionsForStudents is the bulk form of ListCompanions: the links of
	// many children at once, for the offline lists (student export, class
	// roster) that render the "mit wem" detail of a whole school.
	ListCompanionsForStudents(ctx context.Context, studentIDs []int64) (map[int64][]userModels.CompanionLink, error)

	// ListCompanionIDs returns just the distinct ids of the children this child
	// is linked to, across all weekdays. Exists for the shared lock protocol,
	// which needs the far end of every stored edge but none of the names — the
	// name join of ListCompanions would be a wasted query per update.
	ListCompanionIDs(ctx context.Context, studentID int64) ([]int64, error)

	// TrimCompanionsToDays drops every companion link on a weekday the child's
	// departure plan no longer allows, and reports the links that remain.
	TrimCompanionsToDays(ctx context.Context, studentID int64, allowedDays map[string]bool) ([]userModels.CompanionLink, error)

	// CheckCompanionTrim reports, without writing anything, whether the trim
	// TrimCompanionsToDays would perform for allowedDays leaves a former
	// companion without any remaining "mit wem" detail. A nil/empty allowedDays
	// means every link is dropped. For a trim that a submitted DEPARTURE PLAN
	// drives, use CheckCompanionTrimForPlan instead.
	CheckCompanionTrim(ctx context.Context, studentID int64, allowedDays map[string]bool) error

	// CheckCompanionTrimForPlan is CheckCompanionTrim for the caller whose trim
	// is derived from a departure plan it submitted, and additionally verifies
	// that the caller knew which links that plan is about to delete.
	CheckCompanionTrimForPlan(ctx context.Context, studentID int64, allowedDays map[string]bool, expectedFingerprint *string) error

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

// LockStudentsForUpdate takes every student row lock a request needs up front,
// in ascending id order.
//
// A companion update writes TWO children: the child being edited and, on a
// confirmed extension, the linked child whose departure plan is widened. Each
// of those writes locks its row, and without a shared order two requests
// editing children 1 and 2 while extending each other acquire 1→2 and 2→1 and
// PostgreSQL aborts one as a deadlock (a 500 for a legitimate edit). Sorting
// only the companions is not enough: the subject is already locked by then, so
// the order has to be established here, before the first lock of the request.
//
// Missing rows are skipped rather than reported: this establishes lock order
// and nothing else, and the validation that follows already distinguishes "the
// child being edited is gone" from "a submitted companion is gone".
func (s *studentService) LockStudentsForUpdate(ctx context.Context, ids []int64) error {
	// Nothing is held yet, so every id is "at or above the bound" and may wait.
	return s.LockStudentsForUpdateBelow(ctx, ids, 0)
}

// LockStudentsForUpdateBelow implements the ascending-order acquisition with an
// escape hatch for ids the transaction learns about only after it already holds
// higher locks.
//
// The ascending order alone is enough while the whole set is known up front.
// It is NOT enough for the companion graph: the far ends are read from the edge
// table, and a concurrent writer can commit a new edge between that read and
// the lock pass. Locking such a late, LOWER id normally would put this
// transaction (holding the higher id, wanting the lower) head-on against a
// writer following the plain ascending order (holding the lower, wanting the
// higher) — PostgreSQL breaks that with a deadlock abort, i.e. a 500 on a
// legitimate edit. Taking the downward locks with NOWAIT cannot deadlock: the
// lock is either free (the overwhelmingly common case, since the writer that
// created the edge has committed and released it) or the request is refused
// with the retriable ErrCompanionLockBusy.
func (s *studentService) LockStudentsForUpdateBelow(ctx context.Context, ids []int64, maxHeldID int64) error {
	ordered := make([]int64, 0, len(ids))
	seen := make(map[int64]bool, len(ids))
	for _, id := range ids {
		if id <= 0 || seen[id] {
			continue
		}
		seen[id] = true
		ordered = append(ordered, id)
	}
	sort.Slice(ordered, func(i, j int) bool { return ordered[i] < ordered[j] })

	for _, id := range ordered {
		var err error
		if id < maxHeldID {
			_, err = s.studentRepo.FindByIDForUpdateNoWait(ctx, id)
		} else {
			_, err = s.studentRepo.FindByIDForUpdate(ctx, id)
		}
		switch {
		case err == nil:
		case errors.Is(err, sql.ErrNoRows):
			continue
		case base.IsLockNotAvailable(err):
			return ErrCompanionLockBusy
		default:
			return err
		}
	}
	return nil
}

// LockCompanionGraph locks the subjects, the caller's additional ids, and the
// far end of every stored edge of every subject in one ascending-id pass (via
// LockStudentsForUpdate).
//
// It takes a SET of subjects, not a single one, because a transaction that
// writes several children (the enrollment change-request approval applies one
// approved child after the other) would otherwise acquire their student locks
// in its own order — enrollment sort order — while the per-child companion
// writes walk the far ends in ascending order. Two orders over the same rows is
// the deadlock this protocol exists to prevent: an approval holding student 100
// and waiting for 10, against a companion edit holding 10 and waiting for 100,
// ends with PostgreSQL aborting one of them. Locking the complete set here, once,
// in global order removes the inversion; every later per-row lock in the same
// transaction re-acquires a row it already holds.
//
// The stored-companion snapshot is read BEFORE the locks (there is no other
// order), so an edge committed between the snapshot and the lock pass could
// have a far end this pass never locked. One re-read under the subjects' locks
// closes that: every writer that creates or removes an edge touching a subject
// locks the subject's row first, so once we hold them no further edges can
// appear, and a single top-up pass over the late edges suffices.
//
// That top-up is the one place where the ascending order cannot be honored: a
// late edge may point at an id BELOW ids this transaction already holds, and
// waiting for it head-on against a writer coming up the other way is exactly
// the deadlock this whole protocol exists to prevent. Those downward locks are
// therefore taken with NOWAIT (LockStudentsForUpdateBelow), which either gets
// them immediately or refuses the request as retriable — never blocks.
func (s *studentService) LockCompanionGraph(ctx context.Context, subjectIDs []int64, additional []int64) error {
	if len(subjectIDs) == 0 {
		return nil
	}

	stored, err := s.companionIDsOfMany(ctx, subjectIDs)
	if err != nil {
		return err
	}

	locked := make(map[int64]bool, len(subjectIDs)+len(additional)+len(stored))
	ids := make([]int64, 0, len(subjectIDs)+len(additional)+len(stored))
	var maxLocked int64
	add := func(id int64) {
		if id <= 0 || locked[id] {
			return
		}
		locked[id] = true
		ids = append(ids, id)
		if id > maxLocked {
			maxLocked = id
		}
	}
	for _, id := range subjectIDs {
		add(id)
	}
	for _, id := range additional {
		add(id)
	}
	for _, id := range stored {
		add(id)
	}
	if err := s.LockStudentsForUpdate(ctx, ids); err != nil {
		return err
	}

	fresh, err := s.companionIDsOfMany(ctx, subjectIDs)
	if err != nil {
		return err
	}
	var late []int64
	for _, id := range fresh {
		if id <= 0 || locked[id] {
			continue
		}
		locked[id] = true
		late = append(late, id)
	}
	if len(late) == 0 {
		return nil
	}
	return s.LockStudentsForUpdateBelow(ctx, late, maxLocked)
}

// VerifyCompanionStrandingBatch hands the decision to the repository, which is
// the layer that deferred the verdicts in the first place (every departure-plan
// write passes through it) and the only one that can re-read the plans and edges
// the batch left behind.
func (s *studentService) VerifyCompanionStrandingBatch(ctx context.Context) error {
	return s.studentRepo.VerifyCompanionStrandingBatch(ctx)
}

// companionIDsOfMany collects the far ends of every stored edge of the given
// subjects, duplicates included — LockCompanionGraph folds them.
func (s *studentService) companionIDsOfMany(ctx context.Context, subjectIDs []int64) ([]int64, error) {
	out := make([]int64, 0, len(subjectIDs))
	for _, subjectID := range subjectIDs {
		if subjectID <= 0 {
			continue
		}
		ids, err := s.ListCompanionIDs(ctx, subjectID)
		if err != nil {
			return nil, err
		}
		out = append(out, ids...)
	}
	return out, nil
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

func (s *studentService) ListCompanionsForStudents(ctx context.Context, studentIDs []int64) (map[int64][]userModels.CompanionLink, error) {
	if len(studentIDs) == 0 {
		return map[int64][]userModels.CompanionLink{}, nil
	}
	return s.companionRepo.ListLinksForStudents(ctx, studentIDs)
}

func (s *studentService) ListCompanionIDs(ctx context.Context, studentID int64) ([]int64, error) {
	if studentID <= 0 {
		return nil, ErrStudentNotFound
	}
	edges, err := s.companionRepo.ListForStudent(ctx, studentID)
	if err != nil {
		return nil, err
	}
	seen := make(map[int64]bool, len(edges))
	ids := make([]int64, 0, len(edges))
	for _, edge := range edges {
		far, ok := edge.Other(studentID)
		if !ok || seen[far] {
			continue
		}
		seen[far] = true
		ids = append(ids, far)
	}
	return ids, nil
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

	trimmed, changed := trimCompanionLinks(links, allowedDays)
	if !changed {
		return links, nil
	}

	// Every weekday the trim drops may have been the far child's only "mit wem"
	// detail for that day. Refuse before writing, exactly like the
	// submitted-list path does.
	if err := s.checkCompanionRemovals(ctx, studentID, links, trimmed); err != nil {
		return nil, err
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

// CheckCompanionTrim answers the same question as the trim inside
// TrimCompanionsToDays WITHOUT writing anything.
//
// It exists for the same reason CheckCompanionConflicts does: the caller runs
// inside the request's tenant transaction, which commits on every non-5xx
// response, so a rejection raised once the update has started writing would
// keep those writes. A nil or empty allowedDays means every link goes.
//
// This form asks only the stranding question, for a caller whose removal intent
// is unambiguous without any reference to the stored links — deleting the child
// itself, whose edges go by ON DELETE CASCADE. A trim that a submitted departure
// plan drives is the other case and belongs in CheckCompanionTrimForPlan.
func (s *studentService) CheckCompanionTrim(ctx context.Context, studentID int64, allowedDays map[string]bool) error {
	links, trimmed, err := s.planCompanionTrim(ctx, studentID, allowedDays)
	if err != nil || links == nil {
		return err
	}
	return s.checkCompanionRemovals(ctx, studentID, links, trimmed)
}

// CheckCompanionTrimForPlan is CheckCompanionTrim for a trim that a submitted
// DEPARTURE PLAN drives, and it additionally holds the caller to a baseline.
//
// expectedFingerprint is the CompanionLinksFingerprint of the links the caller
// was looking at when it built that plan, and it is what makes the removal safe.
// A plan-driven trim carries no list of links, so a caller holding a stale plan
// DELETES links it never saw: the Stammdaten forms resubmit the whole departure
// plan on every save, so an address edit started before someone else widened the
// plan and linked a child on the new day drops that fresh edge — and the
// stranding check waves it through whenever the far child still has a note or
// another link. Comparing the claim against the stored links under the subject's
// row lock turns that silent loss into a retriable ErrCompanionsChanged.
//
// nil means the caller makes NO claim, which is refused as soon as the trim
// would actually drop something — "I don't know what is stored" is not a licence
// to delete it. A trim that changes nothing needs no claim at all, so the
// overwhelmingly common resubmit of an unchanged plan passes untouched.
func (s *studentService) CheckCompanionTrimForPlan(ctx context.Context, studentID int64, allowedDays map[string]bool, expectedFingerprint *string) error {
	links, trimmed, err := s.planCompanionTrim(ctx, studentID, allowedDays)
	if err != nil || links == nil {
		return err
	}
	// Before the stranding check, not after: a stale caller must be told to
	// reload, not sent off to fill in another child's Heimweg for a removal it
	// never asked for.
	if expectedFingerprint == nil || *expectedFingerprint != userModels.CompanionLinksFingerprint(links) {
		return ErrCompanionsChanged
	}
	return s.checkCompanionRemovals(ctx, studentID, links, trimmed)
}

// planCompanionTrim computes the trim allowedDays would perform. It returns the
// stored links and what survives, or (nil, nil, nil) when the trim changes
// nothing — which is how callers tell "no removal at all" from "these links are
// about to lose weekdays".
func (s *studentService) planCompanionTrim(ctx context.Context, studentID int64, allowedDays map[string]bool) (stored, trimmed []userModels.CompanionLink, err error) {
	if studentID <= 0 {
		return nil, nil, ErrStudentNotFound
	}
	links, err := s.companionRepo.ListLinksForStudent(ctx, studentID)
	if err != nil {
		return nil, nil, err
	}
	kept, changed := trimCompanionLinks(links, allowedDays)
	if !changed {
		return nil, nil, nil
	}
	return links, kept, nil
}

// trimCompanionLinks keeps only the weekdays allowedDays permits, dropping a
// link that has none left, and reports whether anything changed.
func trimCompanionLinks(links []userModels.CompanionLink, allowedDays map[string]bool) ([]userModels.CompanionLink, bool) {
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
	return trimmed, changed
}

// checkCompanionRemovals refuses a write that would strand a child at the FAR
// end of a removed edge.
//
// An edge is stored once and read from both sides, so removing it edits the
// other child's record too: their departure plan may allow "Anderes Kind"
// precisely because this link answered "mit wem" (Student.Validate accepts a
// link in place of the free-text note). Dropping it silently would leave that
// child with an accompanied plan, no note and no link — Stammdaten that
// contradict themselves and, worse, make every later unrelated edit of that
// child fail with ErrDepartureCompanionNoteRequired.
//
// Narrowing the far child's plan instead is deliberately NOT an option: one
// child's edit must never take away another child's departure permissions (see
// WithAccompaniedDays). So this refuses, naming the fix the user has to make
// first.
//
// The check is PER WEEKDAY, exactly like Student.Validate's cover check: a link
// that keeps its Monday but loses its Tuesday still strands the far child on an
// accompanied Tuesday, and another companion covering only Monday does not
// answer for Tuesday either. Every removed (companion, weekday) pair therefore
// needs the far child to keep a note, another link ON THAT DAY, or a plan that
// does not claim "Anderes Kind" there.
func (s *studentService) checkCompanionRemovals(ctx context.Context, studentID int64, before, after []userModels.CompanionLink) error {
	keptDays := make(map[int64]map[string]bool, len(after))
	for _, link := range after {
		days := keptDays[link.CompanionStudentID]
		if days == nil {
			days = make(map[string]bool, len(link.Weekdays))
			keptDays[link.CompanionStudentID] = days
		}
		for _, day := range link.Weekdays {
			days[day] = true
		}
	}

	removedDays := make(map[int64][]string, len(before))
	removed := make([]int64, 0, len(before))
	for _, link := range before {
		for _, day := range link.Weekdays {
			if keptDays[link.CompanionStudentID][day] {
				continue
			}
			if _, seen := removedDays[link.CompanionStudentID]; !seen {
				removed = append(removed, link.CompanionStudentID)
			}
			removedDays[link.CompanionStudentID] = append(removedDays[link.CompanionStudentID], day)
		}
	}
	if len(removed) == 0 {
		return nil
	}
	sort.Slice(removed, func(i, j int) bool { return removed[i] < removed[j] })

	// The weekdays each far child still has covered by OTHER companions once
	// our edges are gone.
	covered, err := s.companionRepo.CompanionDaysCoveredExcluding(ctx, removed, studentID)
	if err != nil {
		return err
	}
	students, err := s.studentRepo.FindByIDs(ctx, removed)
	if err != nil {
		return err
	}

	for _, id := range removed {
		companion := students[id]
		if companion == nil {
			continue // deleted or another tenant — nothing left to strand
		}
		if companion.DepartureCompanionNote != nil && strings.TrimSpace(*companion.DepartureCompanionNote) != "" {
			continue // the free-text note carries the detail for every day
		}
		accompanied := userModels.AccompaniedWeekdays(companion.AllowedDepartureModes, companion.DepartureDays)
		for _, day := range removedDays[id] {
			if !accompanied[day] {
				continue // their plan does not claim "Anderes Kind" on this day
			}
			if covered[id][day] {
				continue // another companion walks with them on this day
			}
			return ErrCompanionWouldLoseDeparture
		}
	}
	return nil
}

// CompanionUpdate is the input for ReplaceCompanions.
type CompanionUpdate struct {
	// Links is the child's complete companion list. Empty clears every link.
	Links []userModels.CompanionLink

	// ExpectedFingerprint is the CompanionLinksFingerprint of the list the
	// client had in front of it when it built Links. The write is refused with
	// ErrCompanionsChanged when the stored links no longer match it.
	//
	// This is the whole answer to the lost update the replacement semantics
	// invite: Links is a COMPLETE list, so two staff members starting from the
	// same snapshot both submit everything they saw, and the row locks merely
	// order the two writes — the later one would drop the other's freshly
	// committed link with nothing in the data to object. Comparing under the
	// subject's row lock (the caller holds it before validation runs) turns
	// that silent overwrite into a retriable conflict.
	//
	// nil means "no expectation" and is for system callers that derive the list
	// from the stored state themselves (trims, enrollment approval); an HTTP
	// caller that submits a user-built list always sets it.
	ExpectedFingerprint *string

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

	// AuthorizedExtensions are the WEEKDAYS per companion that the caller was
	// both authorized and explicitly confirmed to widen, checked against the same
	// locked rows this update writes. nil means "no restriction" and is for
	// system callers only; an HTTP caller always sets it, so a companion — or a
	// weekday of one — that slipped into the write pass without passing
	// authorization and confirmation fails loudly instead of being widened.
	//
	// Per weekday, not per companion: the confirmation the human answered names
	// concrete days ("Tom darf donnerstags noch nicht …"), so a conflict that
	// grew a further day between the question and the answer must not ride along
	// on the earlier yes.
	AuthorizedExtensions map[int64]map[string]bool
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
		// Callers that write concurrently take every row touched here — subject
		// included — in one ascending-id pass via LockStudentsForUpdate before
		// the first validation (the HTTP path does this in lockCompanionRows).
		// That is what keeps the conflict set stable between the authorization
		// pass and this one, and what stops two requests from acquiring the same
		// pair of rows in opposite orders.
		for _, conflict := range conflicts {
			if update.AuthorizedExtensions != nil {
				authorized := update.AuthorizedExtensions[conflict.StudentID]
				for _, day := range conflict.Weekdays {
					if !authorized[day] {
						return nil, fmt.Errorf("%w: student %d, weekday %s", ErrCompanionExtensionNotAuthorized, conflict.StudentID, day)
					}
				}
			}
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

	// Replacing the list DELETES the edges that are not in it, and each of those
	// is also a row on another child's card. Checked before the conflict pass so
	// a removal that would strand a companion is refused on the read-only path
	// too, i.e. before the update writes anything.
	current, err := s.companionRepo.ListLinksForStudent(ctx, studentID)
	if err != nil {
		return nil, nil, err
	}

	// The submitted list only describes an intent relative to the list the user
	// SAW. Anything else stored now means someone else edited this child in the
	// meantime and their links would be deleted by a write that never meant to
	// touch them, so the request is refused instead. Checked here rather than in
	// the writer so the read-only pre-check raises it too — the request runs
	// inside the middleware's tenant transaction, which commits every non-5xx
	// response, so a 409 after the first write would keep that write.
	if update.ExpectedFingerprint != nil && *update.ExpectedFingerprint != userModels.CompanionLinksFingerprint(current) {
		return nil, nil, ErrCompanionsChanged
	}

	if err := s.checkCompanionRemovals(ctx, studentID, current, update.Links); err != nil {
		return nil, nil, err
	}

	if len(companionDays) == 0 {
		return edges, nil, nil
	}

	conflicts, err := s.resolveCompanions(ctx, studentID, companionDays)
	if err != nil {
		return nil, nil, err
	}
	return edges, conflicts, nil
}

// resolveCompanions loads the requested companions, enforces the companion cap
// at their end of the edge, and reports which of them may not leave with
// another child on the requested days, in ascending id order.
func (s *studentService) resolveCompanions(ctx context.Context, studentID int64, companionDays map[int64][]string) ([]CompanionConflict, error) {
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

	// MaxStudentCompanions caps EVERY child's list, and an edge counts on both
	// cards. Checking only the submitted list would let eleven children each
	// name the same child as their single companion and leave that child with
	// eleven links — over the cap, and unable to edit their own list afterwards
	// because the full list is then rejected. The subject's own edges are
	// excluded so re-submitting an unchanged list is never rejected.
	degrees, err := s.companionRepo.CompanionCountsExcluding(ctx, companionIDs, studentID)
	if err != nil {
		return nil, err
	}

	var conflicts []CompanionConflict
	for _, id := range companionIDs {
		companion := found[id]
		if companion == nil {
			return nil, ErrCompanionNotFound
		}
		if degrees[id]+1 > MaxStudentCompanions {
			return nil, ErrCompanionAtLimit
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
	// The links we are about to write ARE the "mit wem" detail for exactly these
	// weekdays, so the accompanied-needs-a-note invariant is satisfied for them
	// without inventing text. Days the companion already had accompanied stay
	// uncovered here on purpose — the repository probes the stored edges for
	// those, which is the only thing that can honestly answer for them.
	companion.MarkDepartureCompanionDays(days...)
	return s.studentRepo.Update(ctx, companion)
}

func (s *studentService) CompanionIDsForWeekday(ctx context.Context, studentIDs []int64, weekday int) (map[int64][]int64, error) {
	return s.companionRepo.CompanionIDsForWeekday(ctx, studentIDs, weekday)
}
