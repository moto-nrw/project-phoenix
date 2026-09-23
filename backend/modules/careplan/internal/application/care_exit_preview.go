package application

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// careExitFacts is everything the binding preview names about the children,
// read in one pass so the counting half and the token see the same state.
type careExitFacts struct {
	students        map[int64]domain.CareStudent
	persons         map[int64]domain.CarePerson
	rosterCounts    map[int64]int
	bookingCounts   map[int64]int
	sourceOfferings map[int64][]careplan.CareExitSourceOffering
	weeklyPlans     map[int64][]string
	requestCounts   map[int64]int
	presence        map[int64]bool
}

// buildPreview resolves every child, collects the impacts and derives the
// token. It is a read: it opens no transaction of its own. With lock set it
// runs inside the confirmation's transaction and takes the row locks first,
// which is what makes the confirmation's comparison meaningful.
func (s *CareLifecycle) buildPreview(
	ctx context.Context, input careplan.CareExitInput, lock bool,
	withdrawnOfferings map[int64][]careplan.CareExitSourceOffering,
) (*careplan.CareExitPreview, error) {
	facts, err := s.loadCareExitFacts(ctx, input, lock)
	if err != nil {
		return nil, err
	}
	today := s.today()
	preview := &careplan.CareExitPreview{
		LastCareDay: input.LastCareDay, Reason: input.Reason, ReasonNote: input.ReasonNote,
		Students: make([]careplan.CareExitImpact, 0, len(input.StudentIDs)),
	}
	updatedAt := make(map[int64]time.Time, len(facts.students))
	for _, id := range input.StudentIDs {
		impact := careplan.CareExitImpact{StudentID: id}
		student, found := facts.students[id]
		if found {
			updatedAt[id] = student.UpdatedAt
			impact.Blocker = domain.CareExitBlocker(&student, input.LastCareDay, today)
			impact.SchoolClass, impact.PlannedEndsOn = student.SchoolClass, student.EnrolledUntil
			if person, ok := facts.persons[student.PersonID]; ok {
				impact.FirstName, impact.LastName, impact.HasRFIDTag = person.FirstName, person.LastName, person.HasRFIDTag
			}
			impact.PlannedRosterRows = facts.rosterCounts[id]
			impact.ActivityBookings = facts.bookingCounts[id]
			impact.SourceOfferings = domain.MergeCareExitSourceOfferings(facts.sourceOfferings[id], withdrawnOfferings[id])
			impact.WeeklyPlans = facts.weeklyPlans[id]
			impact.OpenParentRequests = facts.requestCounts[id]
			impact.CurrentlyPresent = facts.presence[id]
		} else {
			impact.Blocker = domain.CareExitBlocker(nil, input.LastCareDay, today)
		}
		if impact.Blocker != "" {
			preview.Blocked = true
		}
		preview.Students = append(preview.Students, impact)
	}
	preview.Token = s.fingerprint(domain.CareExitTokenContent(input, updatedAt, preview.Students))
	return preview, nil
}

func (s *CareLifecycle) loadCareExitFacts(ctx context.Context, input careplan.CareExitInput, lock bool) (*careExitFacts, error) {
	ids := input.StudentIDs
	facts := &careExitFacts{}
	var err error
	if facts.students, err = s.owners.Students.FindCareStudents(ctx, ids, lock); err != nil {
		return nil, err
	}
	if lock {
		if err := s.lockCareExitRows(ctx, ids, input.LastCareDay); err != nil {
			return nil, err
		}
	}
	personIDs := make([]int64, 0, len(facts.students))
	for _, student := range facts.students {
		personIDs = append(personIDs, student.PersonID)
	}
	if facts.persons, err = s.owners.Students.FindCarePersons(ctx, personIDs); err != nil {
		return nil, err
	}
	if err := s.loadCareExitCounts(ctx, facts, ids, input.LastCareDay); err != nil {
		return nil, err
	}
	if facts.sourceOfferings, err = s.sourceOfferingsAfter(ctx, ids, input.LastCareDay.AddDays(1)); err != nil {
		return nil, err
	}
	if facts.weeklyPlans, err = s.weeklyPlanPatterns(ctx, ids); err != nil {
		return nil, err
	}
	if facts.requestCounts, err = s.countOpenRequests(ctx, ids); err != nil {
		return nil, err
	}
	if facts.presence, err = s.findOpenPresence(ctx, ids); err != nil {
		return nil, err
	}
	return facts, nil
}

// lockCareExitRows takes, in this order, the open family requests, every
// live plan row the exit can remove, and the rows whose values the preview
// quotes back. Confirmation then deletes or caps only a state that cannot
// change after its token was derived.
func (s *CareLifecycle) lockCareExitRows(ctx context.Context, ids []int64, lastCareDay calendar.Date) error {
	if err := s.lockOpenRequests(ctx, ids); err != nil {
		return err
	}
	if err := s.lockPlanning(ctx, ids, lastCareDay); err != nil {
		return err
	}
	return s.lockImpactRows(ctx, ids)
}

// loadCareExitCounts counts the roster rows and bookings the exit would
// take away against the BASELINE, not against what is left: a child who
// already has a planned exit had their later rows removed then, and those
// rows come back before the new last care day is applied. Otherwise moving a
// planned exit from June to July would promise "0 Termine entfallen" while
// July's rows are restored and then removed again.
func (s *CareLifecycle) loadCareExitCounts(ctx context.Context, facts *careExitFacts, ids []int64, lastCareDay calendar.Date) error {
	ledger, err := s.records.ListCareExitRemovals(ctx, ids)
	if err != nil {
		return fmt.Errorf("care lifecycle: list care exit removals: %w", err)
	}
	if facts.rosterCounts, err = s.owners.Roster.CountPlannedRosterForCareExit(ctx, ids, lastCareDay, rosterRemovals(ledger)); err != nil {
		return fmt.Errorf("care lifecycle: count planned roster rows after care end: %w", err)
	}
	facts.bookingCounts, err = s.owners.Bookings.CountRunningEnrollmentsForCareExit(ctx, ids, lastCareDay.AddDays(1), bookingRestores(ledger))
	if err != nil {
		return fmt.Errorf("care lifecycle: count running bookings after care end: %w", err)
	}
	return nil
}

func (s *CareLifecycle) refreshLockedWithdrawalPreview(ctx context.Context, state *careExitConfirmation) (*careplan.CareExitPreview, error) {
	completion, err := s.records.FindWithdrawalCompletion(ctx, state.completion.ID, true)
	if err != nil {
		return nil, withdrawalLookupError(err, careplan.ErrCareWithdrawalAlreadyResolved)
	}
	if !withdrawalMatchesCareExit(completion, state.input.StudentIDs) {
		return nil, careplan.ErrCareWithdrawalAlreadyResolved
	}
	if err := s.validateWithdrawalCareEnd(ctx, completion, state.input); err != nil {
		return nil, err
	}
	return s.buildPreview(ctx, state.input, false, withdrawalOfferings(&completion))
}

func withdrawalMatchesCareExit(completion careplan.WithdrawalCompletion, studentIDs []int64) bool {
	return completion.State == careplan.WithdrawalStatePending && completion.StudentID != nil &&
		len(studentIDs) == 1 && studentIDs[0] == *completion.StudentID
}
