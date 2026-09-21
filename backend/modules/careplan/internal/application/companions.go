package application

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
)

// Companions is the native "läuft mit" application (#3427): the edges Care
// Plan owns and the lock protocol every writer of them shares. The child rows
// and their departure plans stay with People Directory behind a port.
type Companions struct {
	records  careplan.Capability
	students ports.CompanionStudents
}

var _ careplan.StudentCompanions = (*Companions)(nil)

// NewCompanions builds the companion capability.
func NewCompanions(records careplan.Capability, students ports.CompanionStudents) (*Companions, error) {
	if records == nil || students == nil {
		return nil, errors.New("care plan companions: records and People Directory students are required")
	}
	return &Companions{records: records, students: students}, nil
}

// LockStudentsForUpdate takes every student row lock a request needs up front,
// in ascending id order.
//
// A companion update writes TWO children: the child being edited and, on a
// confirmed extension, the linked child whose departure plan is widened.
// Without a shared order two requests editing children 1 and 2 while
// extending each other acquire 1→2 and 2→1, and PostgreSQL aborts one as a
// deadlock. Missing rows are skipped: this establishes lock order and nothing
// else.
func (c *Companions) LockStudentsForUpdate(ctx context.Context, ids []int64) error {
	// Nothing is held yet, so every id is at or above the bound and may wait.
	return c.LockStudentsForUpdateBelow(ctx, ids, 0)
}

// LockStudentsForUpdateBelow is the ascending acquisition with an escape
// hatch for ids the transaction learns about only after it already holds
// higher locks. Such a late, LOWER id is taken with NOWAIT: the lock is
// either free, or the request is refused with the retriable
// ErrCompanionLockBusy instead of risking a deadlock abort.
func (c *Companions) LockStudentsForUpdateBelow(ctx context.Context, ids []int64, maxHeldID int64) error {
	for _, id := range domain.DedupeSortedIDs(ids) {
		if err := c.students.LockCompanionStudent(ctx, id, id < maxHeldID); err != nil {
			return err
		}
	}
	return nil
}

// LockCompanionGraph locks the subjects, the caller's additional ids, and the
// far end of every stored edge of every subject in one ascending pass.
//
// It takes a SET of subjects because a transaction writing several children
// (the enrollment change-request approval) would otherwise acquire their
// locks in its own order while the per-child companion writes walk the far
// ends ascending: two orders over the same rows is the deadlock this protocol
// prevents.
//
// The stored-companion snapshot is read BEFORE the locks, so an edge
// committed in between could have a far end this pass never locked. One
// re-read under the subjects' locks closes that: every writer that creates or
// removes an edge touching a subject locks the subject first, so a single
// top-up pass suffices. That top-up may point BELOW ids already held, so it
// is taken with NOWAIT.
func (c *Companions) LockCompanionGraph(ctx context.Context, subjectIDs []int64, additional []int64) error {
	if len(subjectIDs) == 0 {
		return nil
	}
	stored, err := c.companionIDsOfMany(ctx, subjectIDs)
	if err != nil {
		return err
	}
	locked := make(map[int64]bool, len(subjectIDs)+len(additional)+len(stored))
	ids := make([]int64, 0, len(subjectIDs)+len(additional)+len(stored))
	var maxLocked int64
	for _, id := range slices.Concat(subjectIDs, additional, stored) {
		if id <= 0 || locked[id] {
			continue
		}
		locked[id] = true
		ids = append(ids, id)
		maxLocked = max(maxLocked, id)
	}
	if err := c.LockStudentsForUpdate(ctx, ids); err != nil {
		return err
	}
	fresh, err := c.companionIDsOfMany(ctx, subjectIDs)
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
	return c.LockStudentsForUpdateBelow(ctx, late, maxLocked)
}

// companionIDsOfMany collects the far ends of every stored edge of the given
// subjects, duplicates included.
func (c *Companions) companionIDsOfMany(ctx context.Context, subjectIDs []int64) ([]int64, error) {
	links, err := c.ListCompanionsForStudents(ctx, subjectIDs)
	if err != nil {
		return nil, err
	}
	out := make([]int64, 0, len(subjectIDs))
	for _, subjectID := range subjectIDs {
		for _, link := range links[subjectID] {
			out = append(out, link.CompanionStudentID)
		}
	}
	return out, nil
}

func (c *Companions) ListCompanions(ctx context.Context, studentID int64) ([]careplan.CompanionLink, error) {
	if studentID <= 0 {
		return nil, careplan.ErrCompanionStudentNotFound
	}
	return c.storedLinks(ctx, studentID)
}

func (c *Companions) storedLinks(ctx context.Context, studentID int64) ([]careplan.CompanionLink, error) {
	links, err := c.ListCompanionsForStudents(ctx, []int64{studentID})
	if err != nil {
		return nil, err
	}
	return links[studentID], nil
}

func (c *Companions) ListCompanionsForStudents(ctx context.Context, studentIDs []int64) (map[int64][]careplan.CompanionLink, error) {
	if len(studentIDs) == 0 {
		return map[int64][]careplan.CompanionLink{}, nil
	}
	links, err := c.records.ListCompanionLinks(ctx, studentIDs)
	if err != nil {
		return nil, fmt.Errorf("care plan companions: list student companions: %w", err)
	}
	return links, nil
}

func (c *Companions) ListCompanionIDs(ctx context.Context, studentID int64) ([]int64, error) {
	if studentID <= 0 {
		return nil, careplan.ErrCompanionStudentNotFound
	}
	edges, err := c.records.ListCompanionEdges(ctx, studentID)
	if err != nil {
		return nil, fmt.Errorf("care plan companions: list student companions: %w", err)
	}
	seen := make(map[int64]bool, len(edges))
	ids := make([]int64, 0, len(edges))
	for _, edge := range edges {
		far, ok := domain.OtherCompanion(edge, studentID)
		if !ok || seen[far] {
			continue
		}
		seen[far] = true
		ids = append(ids, far)
	}
	return ids, nil
}

func (c *Companions) CompanionIDsForWeekday(ctx context.Context, studentIDs []int64, weekday int) (map[int64][]int64, error) {
	if _, ok := domain.CompanionWeekdayKey(weekday); !ok {
		return nil, careplan.ErrCompanionInvalidWeekday
	}
	ids, err := c.records.CompanionIDsForWeekday(ctx, studentIDs, weekday)
	if err != nil {
		return nil, fmt.Errorf("care plan companions: list companions for weekday: %w", err)
	}
	return ids, nil
}

// TrimCompanionsToDays removes the links the child's departure plan no
// longer covers. It exists for the update that narrows the plan without
// touching the list: dropping "Anderes Kind" from Tuesday drops the Tuesday
// edges too, or the Kindersuche keeps grouping the children on a day the
// Stammdaten forbid. It only ever removes, so no companion's plan is read.
func (c *Companions) TrimCompanionsToDays(ctx context.Context, studentID int64, allowedDays map[string]bool) ([]careplan.CompanionLink, error) {
	if studentID <= 0 {
		return nil, careplan.ErrCompanionStudentNotFound
	}
	links, err := c.storedLinks(ctx, studentID)
	if err != nil {
		return nil, err
	}
	trimmed, changed := domain.TrimCompanionLinks(links, allowedDays)
	if !changed {
		return links, nil
	}
	// Every dropped weekday may have been the far child's only "mit wem"
	// detail for that day. Refuse before writing, like the submitted-list path.
	if err := c.checkCompanionRemovals(ctx, studentID, links, trimmed); err != nil {
		return nil, err
	}
	edges, _, err := domain.BuildCompanionEdges(studentID, trimmed)
	if err != nil {
		return nil, err
	}
	if err := c.records.ReplaceCompanionEdges(ctx, studentID, edges); err != nil {
		return nil, fmt.Errorf("care plan companions: replace student companions: %w", err)
	}
	return trimmed, nil
}

// CheckCompanionTrim answers the question of the trim inside
// TrimCompanionsToDays WITHOUT writing: the caller runs in the request's
// tenant transaction, which commits on every non-5xx response, so a refusal
// raised after the first write would keep that write. This form asks only the
// stranding question, for a caller whose removal intent needs no reference to
// the stored links, such as deleting the child itself.
func (c *Companions) CheckCompanionTrim(ctx context.Context, studentID int64, allowedDays map[string]bool) error {
	links, trimmed, err := c.planCompanionTrim(ctx, studentID, allowedDays)
	if err != nil || links == nil {
		return err
	}
	return c.checkCompanionRemovals(ctx, studentID, links, trimmed)
}

// CheckCompanionTrimForPlan is CheckCompanionTrim for a trim a submitted
// DEPARTURE PLAN drives, and it holds the caller to a baseline. A plan-driven
// trim carries no list of links, so a caller holding a stale plan would
// delete links it never saw. Comparing the claimed fingerprint against the
// stored links under the subject's row lock turns that silent loss into a
// retriable ErrCompanionsChanged. nil claims nothing and is refused as soon
// as the trim would actually drop something.
func (c *Companions) CheckCompanionTrimForPlan(ctx context.Context, studentID int64, allowedDays map[string]bool, expectedFingerprint *string) error {
	links, trimmed, err := c.planCompanionTrim(ctx, studentID, allowedDays)
	if err != nil || links == nil {
		return err
	}
	// Before the stranding check: a stale caller must be told to reload, not
	// sent off to fill in another child's Heimweg for a removal it never asked
	// for.
	if expectedFingerprint == nil || *expectedFingerprint != careplan.CompanionLinksFingerprint(links) {
		return careplan.ErrCompanionsChanged
	}
	return c.checkCompanionRemovals(ctx, studentID, links, trimmed)
}

// planCompanionTrim returns the stored links and what survives, or nil
// links when the trim changes nothing.
func (c *Companions) planCompanionTrim(ctx context.Context, studentID int64, allowedDays map[string]bool) (stored, trimmed []careplan.CompanionLink, err error) {
	if studentID <= 0 {
		return nil, nil, careplan.ErrCompanionStudentNotFound
	}
	links, err := c.storedLinks(ctx, studentID)
	if err != nil {
		return nil, nil, err
	}
	kept, changed := domain.TrimCompanionLinks(links, allowedDays)
	if !changed {
		return nil, nil, nil
	}
	return links, kept, nil
}

// checkCompanionRemovals refuses a write that would strand a child at the FAR
// end of a removed edge. An edge is stored once and read from both sides, so
// removing it edits the other child's record too: its plan may allow "Anderes
// Kind" precisely because this link answered "mit wem". Narrowing the far
// child's plan instead is deliberately NOT an option.
func (c *Companions) checkCompanionRemovals(ctx context.Context, studentID int64, before, after []careplan.CompanionLink) error {
	removed, removedDays := domain.RemovedCompanionDays(before, after)
	if len(removed) == 0 {
		return nil
	}
	// The weekdays each far child still has covered by OTHER companions once
	// our edges are gone.
	covered, err := c.records.CompanionDaysCoveredExcluding(ctx, removed, studentID)
	if err != nil {
		return fmt.Errorf("care plan companions: list student companion days: %w", err)
	}
	students, err := c.students.FindCompanionStudents(ctx, removed)
	if err != nil {
		return err
	}
	for _, id := range removed {
		companion, found := students[id]
		if !found {
			continue // deleted or another tenant: nothing left to strand
		}
		if domain.StrandsCompanion(companion, removedDays[id], covered[id]) {
			return careplan.ErrCompanionWouldLoseDeparture
		}
	}
	return nil
}

// ReplaceCompanions validates the submitted list and writes it as the child's
// complete companion set. Replacing only touches edges that TOUCH the child:
// a link between two other children is left alone.
func (c *Companions) ReplaceCompanions(ctx context.Context, studentID int64, update careplan.CompanionUpdate) ([]careplan.CompanionConflict, error) {
	edges, conflicts, err := c.validateCompanionUpdate(ctx, studentID, update)
	if err != nil {
		return nil, err
	}
	if len(conflicts) > 0 {
		if !update.ExtendCompanionPlans {
			return conflicts, nil
		}
		// Callers that write concurrently took every row touched here in one
		// ascending pass before the first validation, which keeps the conflict
		// set stable between the authorization pass and this one.
		if err := c.extendConflictingPlans(ctx, conflicts, update); err != nil {
			return nil, err
		}
	}
	if err := c.records.ReplaceCompanionEdges(ctx, studentID, edges); err != nil {
		return nil, fmt.Errorf("care plan companions: replace student companions: %w", err)
	}
	return nil, nil
}

func (c *Companions) extendConflictingPlans(ctx context.Context, conflicts []careplan.CompanionConflict, update careplan.CompanionUpdate) error {
	for _, conflict := range conflicts {
		if update.AuthorizedExtensions != nil {
			authorized := update.AuthorizedExtensions[conflict.StudentID]
			for _, day := range conflict.Weekdays {
				if !authorized[day] {
					return fmt.Errorf("%w: student %d, weekday %s", careplan.ErrCompanionExtensionNotAuthorized, conflict.StudentID, day)
				}
			}
		}
		if err := c.students.ExtendAccompaniedDays(ctx, conflict.StudentID, conflict.Weekdays, update.ActorAccountID); err != nil {
			return err
		}
	}
	return nil
}

// CheckCompanionConflicts answers the same question as ReplaceCompanions
// WITHOUT writing: the caller has to know every possible rejection before it
// starts writing, so it runs the full validation regardless of
// ExtendCompanionPlans.
func (c *Companions) CheckCompanionConflicts(ctx context.Context, studentID int64, update careplan.CompanionUpdate) ([]careplan.CompanionConflict, error) {
	_, conflicts, err := c.validateCompanionUpdate(ctx, studentID, update)
	return conflicts, err
}

// validateCompanionUpdate runs every check ReplaceCompanions performs and
// returns the edges to write plus the companions whose own plan does not
// allow the requested days. It writes nothing, so the read-only check and the
// write cannot drift apart.
func (c *Companions) validateCompanionUpdate(ctx context.Context, studentID int64, update careplan.CompanionUpdate) ([]careplan.CompanionEdge, []careplan.CompanionConflict, error) {
	if studentID <= 0 {
		return nil, nil, careplan.ErrCompanionStudentNotFound
	}
	if len(update.Links) > careplan.MaxStudentCompanions {
		return nil, nil, careplan.ErrTooManyCompanions
	}
	subject, err := c.students.FindCompanionStudent(ctx, studentID)
	if err != nil {
		return nil, nil, err
	}
	if subject == nil {
		return nil, nil, careplan.ErrCompanionStudentNotFound
	}
	edges, companionDays, err := domain.BuildCompanionEdges(studentID, update.Links)
	if err != nil {
		return nil, nil, err
	}
	// The subject's own plan gates which days may carry a link at all. A day
	// the plan does not allow is a client bug, so it is a plain rejection.
	allowedDays := update.AccompaniedDays
	if allowedDays == nil {
		allowedDays = subject.AccompaniedDays
	}
	for _, edge := range edges {
		if day, _ := domain.CompanionWeekdayKey(edge.Weekday); !allowedDays[day] {
			return nil, nil, careplan.ErrCompanionDayNotAllowed
		}
	}
	// Replacing the list DELETES the edges not in it, and each is a row on
	// another child's card. The submitted list describes an intent relative
	// to the list the user SAW; anything else stored now means someone else
	// edited this child in between.
	current, err := c.storedLinks(ctx, studentID)
	if err != nil {
		return nil, nil, err
	}
	if update.ExpectedFingerprint != nil && *update.ExpectedFingerprint != careplan.CompanionLinksFingerprint(current) {
		return nil, nil, careplan.ErrCompanionsChanged
	}
	if err := c.checkCompanionRemovals(ctx, studentID, current, update.Links); err != nil {
		return nil, nil, err
	}
	if len(companionDays) == 0 {
		return edges, nil, nil
	}
	conflicts, err := c.resolveCompanions(ctx, studentID, companionDays)
	if err != nil {
		return nil, nil, err
	}
	return edges, conflicts, nil
}

// resolveCompanions loads the requested companions, enforces the cap at
// their end of the edge, and reports which of them may not leave with
// another child on the requested days, in ascending id order. The read is
// unlocked on purpose: it also runs on the read-only path, and every write
// against a companion re-reads it under its row lock.
func (c *Companions) resolveCompanions(ctx context.Context, studentID int64, companionDays map[int64][]string) ([]careplan.CompanionConflict, error) {
	companionIDs := slices.Sorted(maps.Keys(companionDays))
	found, err := c.students.FindCompanionStudents(ctx, companionIDs)
	if err != nil {
		return nil, err
	}
	// MaxStudentCompanions caps EVERY child's list, and an edge counts on
	// both cards. The subject's own edges are excluded so re-submitting an
	// unchanged list is never rejected.
	degrees, err := c.records.CompanionCountsExcluding(ctx, companionIDs, studentID)
	if err != nil {
		return nil, fmt.Errorf("care plan companions: count student companions: %w", err)
	}
	var conflicts []careplan.CompanionConflict
	for _, id := range companionIDs {
		companion, ok := found[id]
		if !ok {
			return nil, careplan.ErrCompanionNotFound
		}
		if degrees[id]+1 > careplan.MaxStudentCompanions {
			return nil, careplan.ErrCompanionAtLimit
		}
		if missing := domain.MissingAccompaniedDays(companion.AccompaniedDays, companionDays[id]); len(missing) > 0 {
			conflicts = append(conflicts, careplan.CompanionConflict{StudentID: id, Weekdays: missing})
		}
	}
	return conflicts, nil
}
