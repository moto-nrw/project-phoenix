package domain

import (
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// NormalizeCareExitInput validates the whole-action fields and returns the
// canonical form every later step works from: the children deduplicated and
// ascending, the note trimmed. allowPast admits a last care day before today,
// which only the guided close-out of a recorded withdrawal may use.
func NormalizeCareExitInput(input careplan.CareExitInput, allowPast bool, today calendar.Date) (careplan.CareExitInput, error) {
	ids := DedupeSortedIDs(input.StudentIDs)
	if len(ids) == 0 {
		return input, careplan.ErrCareExitNoStudents
	}
	if len(ids) > careplan.MaxCareExitBatchSize {
		return input, careplan.ErrCareExitTooManyStudents
	}
	if !allowPast && input.LastCareDay.Before(today) {
		return input, careplan.ErrCareExitDayInPast
	}
	note, err := careplan.NormalizeCareExitReason(input.Reason, &input.ReasonNote)
	if err != nil {
		return input, err
	}
	normalized := careplan.CareExitInput{StudentIDs: ids, LastCareDay: input.LastCareDay, Reason: input.Reason}
	if note != nil {
		normalized.ReasonNote = *note
	}
	return normalized, nil
}

// CareExitBlocker returns the German sentence that keeps one child from
// being ended, or "" when the child can be ended. A nil student is unknown to
// the school.
func CareExitBlocker(student *CareStudent, lastCareDay, today calendar.Date) string {
	switch {
	case student == nil:
		return careplan.CareBlockerUnknown
	case student.IsAlumnus():
		return careplan.CareBlockerAlumnus
	case student.CareEndedOn(today):
		return careplan.CareBlockerAlreadyEnded
	case student.EnrolledFrom != nil && lastCareDay.Before(*student.EnrolledFrom):
		return fmt.Sprintf(careplan.CareBlockerBeforeStart, student.EnrolledFrom.Format("02.01.2006"))
	default:
		return ""
	}
}

// CareExitTokenContent is exactly what the person confirming has seen:
// the selection, the day, the reason, every child's row version and every
// impact number the preview showed. The application fingerprints it, so
// anything moving underneath changes the token, and the confirmation refuses
// instead of doing something else than promised.
func CareExitTokenContent(input careplan.CareExitInput, updatedAt map[int64]time.Time, impacts []careplan.CareExitImpact) []byte {
	type childState struct {
		ID        int64                             `json:"id"`
		UpdatedAt int64                             `json:"updated_at"`
		Roster    int                               `json:"roster"`
		Bookings  int                               `json:"bookings"`
		Offerings []careplan.CareExitSourceOffering `json:"offerings"`
		Requests  int                               `json:"requests"`
		Present   bool                              `json:"present"`
		Tag       bool                              `json:"tag"`
		Blocker   string                            `json:"blocker"`
	}
	payload := struct {
		LastCareDay string       `json:"last_care_day"`
		Reason      string       `json:"reason"`
		ReasonNote  string       `json:"reason_note"`
		Children    []childState `json:"children"`
	}{
		LastCareDay: input.LastCareDay.String(),
		Reason:      input.Reason,
		ReasonNote:  input.ReasonNote,
		Children:    make([]childState, 0, len(impacts)),
	}
	for _, impact := range impacts {
		state := childState{
			ID: impact.StudentID, Roster: impact.PlannedRosterRows, Bookings: impact.ActivityBookings,
			Offerings: impact.SourceOfferings, Requests: impact.OpenParentRequests,
			Present: impact.CurrentlyPresent, Tag: impact.HasRFIDTag, Blocker: impact.Blocker,
		}
		if at, ok := updatedAt[impact.StudentID]; ok {
			state.UpdatedAt = at.UnixNano()
		}
		payload.Children = append(payload.Children, state)
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		// Marshalling a struct of scalars cannot fail; content no preview can
		// produce is still the safe answer if it ever did.
		return []byte("unmatchable")
	}
	return encoded
}

// EqualCareToken compares a quoted token with the rebuilt one. The token is
// a fingerprint of state the caller was shown, not a secret.
func EqualCareToken(actual, expected string) bool {
	expected = strings.ToLower(strings.TrimSpace(expected))
	return actual != "" && expected == actual
}

// DedupeSortedIDs folds duplicates, drops non-positive ids and sorts
// ascending: the project-wide lock order, and the order the token depends on.
func DedupeSortedIDs(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	slices.Sort(out)
	return out
}

// MergeCareExitSourceOfferings lists the offerings a withdrawal already
// recorded first, then the still-running ones, each (name, days) once.
func MergeCareExitSourceOfferings(live, withdrawn []careplan.CareExitSourceOffering) []careplan.CareExitSourceOffering {
	merged := make([]careplan.CareExitSourceOffering, 0, len(live)+len(withdrawn))
	seen := make(map[string]bool, len(live)+len(withdrawn))
	appendUnique := func(rows []careplan.CareExitSourceOffering) {
		for _, row := range rows {
			key := row.Name + "\x00" + strings.Join(row.Days, "\x00")
			if seen[key] {
				continue
			}
			seen[key] = true
			merged = append(merged, row)
		}
	}
	appendUnique(withdrawn)
	appendUnique(live)
	return merged
}

const fixedDaysOfWeekMode = "fixed"

// offeringDays is the booked weekday pattern of one source booking: the
// offering's own days for a fixed offering, the family's choice otherwise.
func offeringDays(offering careplan.CareOffering, link CareExitOfferingLink) []string {
	if offering.DaysOfWeekMode == fixedDaysOfWeekMode {
		return offering.AvailableDays
	}
	return link.SelectedDays
}

type careExitSourceJoin struct {
	application CareExitApplication
	offering    careplan.CareOffering
	link        CareExitOfferingLink
}

// joinCareExitSources joins each source booking to its application child and
// its offering within one tenant, as the former SQL join over the owner
// recordsets did.
func joinCareExitSources(applications []CareExitApplication, offerings []careplan.CareOffering, links []CareExitOfferingLink) []careExitSourceJoin {
	applicationByID := make(map[[2]int64]CareExitApplication, len(applications))
	for _, application := range applications {
		applicationByID[[2]int64{application.TenantID, application.ID}] = application
	}
	offeringByID := make(map[[2]int64]careplan.CareOffering, len(offerings))
	for _, offering := range offerings {
		offeringByID[[2]int64{offering.TenantID, offering.ID}] = offering
	}
	joined := make([]careExitSourceJoin, 0, len(links))
	for _, link := range links {
		application, ok := applicationByID[[2]int64{link.TenantID, link.RequestChildID}]
		if !ok {
			continue
		}
		offering, ok := offeringByID[[2]int64{link.TenantID, link.CareOfferingID}]
		if !ok {
			continue
		}
		joined = append(joined, careExitSourceJoin{application: application, offering: offering, link: link})
	}
	return joined
}

// SourceOfferingsAfter names, per child, the source bookings the child's
// application created that still run on or after validUntil, in the
// offerings' display order.
func SourceOfferingsAfter(
	studentIDs []int64, validUntil calendar.Date,
	applications []CareExitApplication, offerings []careplan.CareOffering, links []CareExitOfferingLink,
) map[int64][]careplan.CareExitSourceOffering {
	wanted := idSet(studentIDs)
	rows := make([]careExitSourceJoin, 0)
	for _, source := range joinCareExitSources(applications, offerings, links) {
		created := source.application.CreatedStudentID
		if created == nil || !wanted[*created] {
			continue
		}
		if source.link.ValidUntil != nil && !source.link.ValidUntil.After(validUntil) {
			continue
		}
		rows = append(rows, source)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		left, right := rows[i], rows[j]
		if *left.application.CreatedStudentID != *right.application.CreatedStudentID {
			return *left.application.CreatedStudentID < *right.application.CreatedStudentID
		}
		if left.offering.SortOrder != right.offering.SortOrder {
			return left.offering.SortOrder < right.offering.SortOrder
		}
		if left.offering.ID != right.offering.ID {
			return left.offering.ID < right.offering.ID
		}
		return left.link.ID < right.link.ID
	})
	result := make(map[int64][]careplan.CareExitSourceOffering, len(studentIDs))
	for _, row := range rows {
		studentID := *row.application.CreatedStudentID
		result[studentID] = append(result[studentID], careplan.CareExitSourceOffering{
			Name: row.offering.Name, Days: offeringDays(row.offering, row.link),
		})
	}
	return result
}

// CareBookingPeriods reads every care-counting booking window of the given
// children from approved applications, oldest first, so the evaluator can
// merge them. A window without a single booked weekday does not count.
func CareBookingPeriods(
	studentIDs []int64,
	applications []CareExitApplication, offerings []careplan.CareOffering, links []CareExitOfferingLink,
) map[int64][]CareBookingPeriod {
	wanted := idSet(studentIDs)
	type period struct {
		studentID int64
		source    careExitSourceJoin
	}
	rows := make([]period, 0)
	for _, source := range joinCareExitSources(applications, offerings, links) {
		studentID := applicationStudentID(source.application)
		if studentID == 0 || !wanted[studentID] || !source.application.Approved || !source.offering.CountsAsCare {
			continue
		}
		if len(offeringDays(source.offering, source.link)) == 0 {
			continue
		}
		rows = append(rows, period{studentID: studentID, source: source})
	}
	sort.SliceStable(rows, func(i, j int) bool {
		left, right := rows[i], rows[j]
		if left.studentID != right.studentID {
			return left.studentID < right.studentID
		}
		if order := compareNullsFirst(left.source.link.ValidFrom, right.source.link.ValidFrom); order != 0 {
			return order < 0
		}
		if order := compareNullsLast(left.source.link.ValidUntil, right.source.link.ValidUntil); order != 0 {
			return order < 0
		}
		return left.source.link.ID < right.source.link.ID
	})
	result := make(map[int64][]CareBookingPeriod, len(studentIDs))
	for _, row := range rows {
		days := offeringDays(row.source.offering, row.source.link)
		result[row.studentID] = append(result[row.studentID], CareBookingPeriod{
			ValidFrom:            row.source.link.ValidFrom,
			ValidUntil:           row.source.link.ValidUntil,
			Days:                 days,
			SourceRequestChildID: row.source.link.RequestChildID,
			SourceOfferings:      []careplan.CareExitSourceOffering{{Name: row.source.offering.Name, Days: days}},
		})
	}
	return result
}

// SourceOfferingIDs names the distinct offerings the given source
// applications select, in ascending order.
func SourceOfferingIDs(links []CareExitOfferingLink, requestChildIDs []int64) []int64 {
	children := idSet(requestChildIDs)
	seen := make(map[int64]bool)
	ids := make([]int64, 0)
	for _, link := range links {
		if !children[link.RequestChildID] || seen[link.CareOfferingID] {
			continue
		}
		seen[link.CareOfferingID] = true
		ids = append(ids, link.CareOfferingID)
	}
	slices.Sort(ids)
	return ids
}

func applicationStudentID(application CareExitApplication) int64 {
	if application.CreatedStudentID != nil {
		return *application.CreatedStudentID
	}
	if application.MatchedStudentID != nil {
		return *application.MatchedStudentID
	}
	return 0
}

func compareNullsFirst(left, right *calendar.Date) int {
	switch {
	case left == nil && right == nil:
		return 0
	case left == nil:
		return -1
	case right == nil:
		return 1
	default:
		return left.Compare(*right)
	}
}

func compareNullsLast(left, right *calendar.Date) int {
	switch {
	case left == nil && right == nil:
		return 0
	case left == nil:
		return 1
	case right == nil:
		return -1
	default:
		return left.Compare(*right)
	}
}

func idSet(ids []int64) map[int64]bool {
	set := make(map[int64]bool, len(ids))
	for _, id := range ids {
		set[id] = true
	}
	return set
}

// WeeklyPlanPatterns renders the recurring arrival and pickup plans of each
// child as the German lines the binding preview lists, sorted.
func WeeklyPlanPatterns(arrivals []careplan.ArrivalSchedule, pickups []careplan.PickupSchedule) map[int64][]string {
	weekdays := [...]string{"", "Montag", "Dienstag", "Mittwoch", "Donnerstag", "Freitag"}
	patterns := make(map[int64][]string)
	for _, value := range arrivals {
		pattern := "Ankunft am " + weekdays[value.Weekday]
		if !value.ExpectedArrival.IsZero() {
			pattern += ": " + value.ExpectedArrival.Format("15:04")
		}
		patterns[value.StudentID] = append(patterns[value.StudentID], pattern)
	}
	for _, value := range pickups {
		patterns[value.StudentID] = append(patterns[value.StudentID], "Abholung am "+weekdays[value.Weekday]+": "+value.PickupTime.Format("15:04"))
	}
	for studentID := range patterns {
		sort.Strings(patterns[studentID])
	}
	return patterns
}

// BuildBookingAuthorityImpact sorts the evaluated children into the ones
// that would be left without any care day and the completions a switch to
// booking-led care would plan.
func BuildBookingAuthorityImpact(evaluations []CareBookingEvaluation, on calendar.Date) *careplan.BookingAuthorityImpact {
	impact := &careplan.BookingAuthorityImpact{
		ReferenceDate:      on,
		BlockingChildren:   make([]careplan.BookingAuthorityImpactChild, 0),
		PlannedCompletions: make([]careplan.BookingAuthorityImpactChild, 0),
	}
	for _, evaluation := range evaluations {
		child := careplan.BookingAuthorityImpactChild{
			StudentID:           strconv.FormatInt(evaluation.StudentID, 10),
			FirstName:           evaluation.FirstName,
			LastName:            evaluation.LastName,
			SchoolClass:         evaluation.SchoolClass,
			FirstBookinglessDay: evaluation.FirstBookinglessDay,
		}
		if !evaluation.HasCareDays {
			impact.BlockingChildren = append(impact.BlockingChildren, child)
			continue
		}
		if evaluation.FirstBookinglessDay != nil && evaluation.FirstBookinglessDay.After(on) {
			impact.PlannedCompletions = append(impact.PlannedCompletions, child)
		}
	}
	return impact
}

// ParticipationBoundaries returns, per child, the first day the child no
// longer participates: the day after the enrolment end or the pending
// completion boundary, whichever comes first.
func ParticipationBoundaries(students map[int64]CareStudent, pending map[int64]calendar.Date) map[int64]calendar.Date {
	boundaries := make(map[int64]calendar.Date, len(students))
	for studentID, student := range students {
		var boundary calendar.Date
		found := false
		if student.EnrolledUntil != nil && !student.EnrolledUntil.IsZero() {
			boundary, found = student.EnrolledUntil.AddDays(1), true
		}
		if day, ok := pending[studentID]; ok {
			if !found || day.Before(boundary) {
				boundary = day
			}
			found = true
		}
		if found {
			boundaries[studentID] = boundary
		}
	}
	return boundaries
}

// ParticipatingOn applies the boundaries to one day. Actual presence always
// wins, and a graduate only remains while actually present.
func ParticipatingOn(
	studentIDs []int64, students map[int64]CareStudent, boundaries map[int64]calendar.Date,
	on calendar.Date, actuallyPresent map[int64]bool,
) map[int64]bool {
	participating := make(map[int64]bool, len(studentIDs))
	for _, id := range studentIDs {
		participating[id] = true
	}
	for id, boundary := range boundaries {
		if !actuallyPresent[id] && !on.Before(boundary) {
			delete(participating, id)
		}
	}
	for id, student := range students {
		if student.IsAlumnus() && !actuallyPresent[id] {
			delete(participating, id)
		}
	}
	return participating
}
