package application

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"
	peopleEnrollment "github.com/moto-nrw/project-phoenix/modules/peopledirectory/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// companionWeekdayKeys maps the ISO weekday of a companion edge onto the
// departure plan's weekday key.
var companionWeekdayKeys = map[int]string{
	1: departure.PickupDayMonday,
	2: departure.PickupDayTuesday,
	3: departure.PickupDayWednesday,
	4: departure.PickupDayThursday,
	5: departure.PickupDayFriday,
}

// readEnrollmentStudent reads a student through the People Directory's
// enrollment contract. lock is "" (read), "update" or "nowait". A missing
// student wraps the runtime's not-found error.
func (d *Decisions) readEnrollmentStudent(ctx context.Context, id int64, lock string) (*Student, error) {
	if d.deps.StudentEnrollment == nil {
		return nil, errors.New("decision: student enrollment capability is required")
	}
	row, err := d.deps.StudentEnrollment.ReadEnrollmentStudent(ctx, id, lock)
	if errors.Is(err, peopleEnrollment.ErrStudentNotFound) {
		return nil, fmt.Errorf("%w: %w", d.deps.Runtime.NoRows, err)
	}
	if err != nil {
		return nil, err
	}
	student := studentFromRecord(row)
	if err := parseEnrollmentWindow(student, row); err != nil {
		return nil, err
	}
	allowed := hydratedAllowedDepartureModes(row)
	student.AllowedDepartureModes = allowed
	student.DepartureDays = allowed.DepartureDays()
	student.BusDays = allowed.BusDays()
	student.PickupDays = allowed.PickupDays()
	student.snapshotDeparturePlan()
	return student, nil
}

func studentFromRecord(row peopleEnrollment.Record) *Student {
	return &Student{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		PersonID:    row.PersonID,
		SchoolClass: row.SchoolClass,

		AddressStreet:            row.AddressStreet,
		AddressCity:              row.AddressCity,
		AddressPostalCode:        row.AddressPostalCode,
		ExtraInfo:                row.ExtraInfo,
		SupervisorNotes:          row.SupervisorNotes,
		HealthInfo:               row.HealthInfo,
		PickupStatus:             row.PickupStatus,
		DepartureCompanionNote:   row.DepartureCompanionNote,
		PhotoPath:                row.PhotoPath,
		GroupID:                  row.GroupID,
		Sick:                     row.Sick,
		Excused:                  row.Excused,
		SickSince:                row.SickSince,
		ExcusedSince:             row.ExcusedSince,
		PhotoConsentGivenAt:      row.PhotoConsentGivenAt,
		AGBAcceptedAt:            row.AGBAcceptedAt,
		DataProcessingAcceptedAt: row.DataProcessingAcceptedAt,
		EmailContactAcceptedAt:   row.EmailContactAcceptedAt,
		PhotoConsentGivenBy:      row.PhotoConsentGivenBy,
		Status:                   row.Status,
	}
}

func parseEnrollmentWindow(student *Student, row peopleEnrollment.Record) error {
	for _, value := range []struct {
		raw    string
		target **calendar.Date
	}{{row.EnrolledFrom, &student.EnrolledFrom}, {row.EnrolledUntil, &student.EnrolledUntil}} {
		if value.raw == "" {
			continue
		}
		date, err := calendar.ParseDate(value.raw)
		if err != nil {
			return fmt.Errorf("decision: invalid owner enrollment date: %w", err)
		}
		*value.target = &date
	}
	return nil
}

// hydratedAllowedDepartureModes preserves the legacy hydration precedence
// for rows written before the backfill.
func hydratedAllowedDepartureModes(row peopleEnrollment.Record) departure.AllowedDepartureModes {
	allowed := departure.AllowedDepartureModes{}
	for day, modes := range row.AllowedDepartureModes {
		for _, mode := range modes {
			allowed[day] = append(allowed[day], departure.DepartureMode(mode))
		}
	}
	days := departure.DepartureDays{}
	for day, mode := range row.DepartureDays {
		days[day] = departure.DepartureMode(mode)
	}
	allowed = allowed.Normalize()
	if allowed.HasAny() {
		return allowed
	}
	if normalized := days.Normalize(); normalized.HasAny() {
		return departure.AllowedDepartureModesFromDeparture(normalized)
	}
	return departure.AllowedDepartureModesFromLegacy(departure.BusDays(row.BusDays).Normalize(), departure.PickupDays(row.PickupDays).Normalize())
}

// enrollmentProfilePatch patches the profile fields an enrollment decision
// may change, marking each field the decision actually changed.
func enrollmentProfilePatch(before, after *Student) peopleEnrollment.ProfilePatch {
	return peopleEnrollment.ProfilePatch{
		HealthInfoSet: enrollmentValueChanged(before.HealthInfo, after.HealthInfo), HealthInfo: after.HealthInfo,
		ExtraInfoSet: enrollmentValueChanged(before.ExtraInfo, after.ExtraInfo), ExtraInfo: after.ExtraInfo,
		PhotoConsentGivenAtSet: enrollmentValueChanged(before.PhotoConsentGivenAt, after.PhotoConsentGivenAt), PhotoConsentGivenAt: after.PhotoConsentGivenAt,
		PhotoConsentGivenBySet: enrollmentValueChanged(before.PhotoConsentGivenBy, after.PhotoConsentGivenBy), PhotoConsentGivenBy: after.PhotoConsentGivenBy,
		AGBAcceptedAtSet: enrollmentValueChanged(before.AGBAcceptedAt, after.AGBAcceptedAt), AGBAcceptedAt: after.AGBAcceptedAt,
		DataProcessingAcceptedAtSet: enrollmentValueChanged(before.DataProcessingAcceptedAt, after.DataProcessingAcceptedAt), DataProcessingAcceptedAt: after.DataProcessingAcceptedAt,
		EmailContactAcceptedAtSet: enrollmentValueChanged(before.EmailContactAcceptedAt, after.EmailContactAcceptedAt), EmailContactAcceptedAt: after.EmailContactAcceptedAt,
	}
}

func enrollmentValueChanged[T comparable](before, after *T) bool {
	if before == nil || after == nil {
		return before != after
	}
	return *before != *after
}

// enrollmentStudentInput is the class, lifecycle status and window of a
// student the enrollment contract creates or renews.
func enrollmentStudentInput(student *Student) peopleEnrollment.Input {
	input := peopleEnrollment.Input{
		PersonID: student.PersonID, SchoolClass: student.SchoolClass, Status: student.Status,
	}
	if student.EnrolledFrom != nil {
		input.EnrolledFrom = student.EnrolledFrom.String()
	}
	if student.EnrolledUntil != nil {
		input.EnrolledUntil = student.EnrolledUntil.String()
	}
	return input
}

// applyEnrollmentDeparture writes a changed departure plan. Enrollment
// coordinates the "läuft mit" graph; only the two owners persist their rows.
// The owner takes the shared class gate before locking the subject row.
func (d *Decisions) applyEnrollmentDeparture(ctx context.Context, before, student *Student) error {
	if d.deps.Companions == nil {
		return errors.New("decision: departure companion capability is required")
	}
	current, err := d.readEnrollmentStudent(ctx, student.ID, "update")
	if err != nil {
		return err
	}
	enrollmentRebaseUntouchedDeparturePlan(student, current)
	allowed := enrollmentResolveAllowedDepartureModes(student, current)
	student.AllowedDepartureModes = allowed
	student.DepartureDays = allowed.DepartureDays()
	student.BusDays = allowed.BusDays()
	student.PickupDays = allowed.PickupDays()
	dropIDs, err := d.markCoveredCompanionDays(ctx, student, allowed)
	if err != nil {
		return err
	}
	if err := d.deps.People.Rules.ValidateStudent(student); err != nil {
		return err
	}
	if !allowed.HasMode(departure.DepartureAccompanied) {
		student.DepartureCompanionNote = nil
	}
	if err := d.deps.StudentEnrollment.ApplyEnrollmentProfile(ctx, student.ID, departurePatch(before, student, allowed)); err != nil {
		return err
	}
	if len(dropIDs) > 0 {
		if err := d.deps.Companions.DeleteCompanionEdges(ctx, dropIDs); err != nil {
			return err
		}
		recordCompanionChange(ctx)
	}
	status := allowed.LegacyPickupStatus()
	student.PickupStatus = &status
	student.snapshotDeparturePlan()
	return nil
}

// markCoveredCompanionDays keeps the edges the new plan still allows —
// marking their days as covered — and returns the ids of those it no longer
// allows, after checking their far ends keep a way home.
func (d *Decisions) markCoveredCompanionDays(ctx context.Context, student *Student, allowed departure.AllowedDepartureModes) ([]int64, error) {
	edges, err := d.deps.Companions.CompanionsOfStudent(ctx, student.ID)
	if err != nil {
		return nil, err
	}
	accompanied := departure.Plan{AllowedDepartureModes: allowed, DepartureDays: student.DepartureDays}.AccompaniedDays()
	removedDays := map[int64][]string{}
	var dropIDs []int64
	for _, edge := range edges {
		far, ok := edge.other(student.ID)
		if !ok {
			continue
		}
		day := companionWeekdayKeys[edge.Weekday]
		if accompanied[day] {
			student.markDepartureCompanionDays(day)
		} else {
			dropIDs = append(dropIDs, edge.ID)
			removedDays[far] = append(removedDays[far], day)
		}
	}
	if err := d.checkEnrollmentCompanionRemoval(ctx, student.ID, removedDays); err != nil {
		return nil, err
	}
	return dropIDs, nil
}

// departurePatch is the profile patch of a departure write: the changed
// profile fields plus the whole normalized plan and its companion note.
func departurePatch(before, student *Student, allowed departure.AllowedDepartureModes) peopleEnrollment.ProfilePatch {
	patch := enrollmentProfilePatch(before, student)
	patch.DepartureSet = true
	patch.DepartureCompanionNote = student.DepartureCompanionNote
	patch.DepartureCompanionDays = student.DepartureCompanionDays
	patch.AllowedDepartureModes = map[string][]string{}
	for day, modes := range allowed {
		for _, mode := range modes {
			patch.AllowedDepartureModes[day] = append(patch.AllowedDepartureModes[day], string(mode))
		}
	}
	return patch
}

// checkEnrollmentCompanionRemoval refuses to trim edges that would leave a
// linked child without an allowed way home. The far ends are locked in
// ascending id order: below the subject without waiting, above it waiting.
// Inside a coordinated multi-child write the verdicts are deferred to the
// open stranding batch.
func (d *Decisions) checkEnrollmentCompanionRemoval(ctx context.Context, subjectID int64, removedDays map[int64][]string) error {
	if len(removedDays) == 0 {
		return nil
	}
	ids := make([]int64, 0, len(removedDays))
	for id := range removedDays {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	farEnds, err := d.lockCompanionFarEnds(ctx, subjectID, ids)
	if err != nil {
		return err
	}
	if batch := departure.StrandingBatchFromContext(ctx); batch != nil {
		for _, id := range ids {
			batch.Defer(id, removedDays[id])
		}
		return nil
	}
	covered, err := d.deps.Companions.CompanionDaysCoveredExcluding(ctx, ids, subjectID)
	if err != nil {
		return err
	}
	for _, id := range ids {
		if companionWouldLoseDeparture(farEnds[id], removedDays[id], covered[id]) {
			return d.deps.People.ErrCompanionWouldLoseDeparture
		}
	}
	return nil
}

// lockCompanionFarEnds reads the far ends of the trimmed edges under a row
// lock. A far end that vanished is skipped; a NOWAIT lock that lost the race
// answers the busy refusal.
func (d *Decisions) lockCompanionFarEnds(ctx context.Context, subjectID int64, ids []int64) (map[int64]*Student, error) {
	farEnds := make(map[int64]*Student, len(ids))
	for _, id := range ids {
		lock := "update"
		if id < subjectID {
			lock = "nowait"
		}
		student, err := d.readEnrollmentStudent(ctx, id, lock)
		switch {
		case err != nil && d.deps.Runtime.NotFound(err):
			continue
		case err != nil && d.deps.People.IsLockNotAvailable != nil && d.deps.People.IsLockNotAvailable(err):
			return nil, d.deps.People.ErrCompanionLockBusy
		case err != nil:
			return nil, err
		}
		farEnds[id] = student
	}
	return farEnds, nil
}

// companionWouldLoseDeparture reports whether a far end relies on a removed
// edge for an accompanied day no note and no other link covers.
func companionWouldLoseDeparture(student *Student, removed []string, covered map[string]bool) bool {
	if student == nil || (student.DepartureCompanionNote != nil && strings.TrimSpace(*student.DepartureCompanionNote) != "") {
		return false
	}
	accompanied := departure.Plan{AllowedDepartureModes: student.AllowedDepartureModes, DepartureDays: student.DepartureDays}.AccompaniedDays()
	for _, day := range removed {
		if accompanied[day] && !covered[day] {
			return true
		}
	}
	return false
}
