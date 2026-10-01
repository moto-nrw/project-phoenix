package application

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/classday"
)

// SupervisionStudentSheet builds the per-child sheet a supervisor opens from
// a running block (#2527) and records the access.
//
// The authorization ("is this caller assigned to a block holding this
// child") happens BEFORE this call, in the timetable operations — this method
// deliberately owns no access rule of its own, so there is exactly one place
// where the assignment is checked.
func (s *dayReports) SupervisionStudentSheet(ctx context.Context, in classday.SupervisionSheetInput) (*classday.SupervisionStudentSheet, error) {
	if in.StudentID <= 0 {
		return nil, fmt.Errorf("supervision sheet: student id required: %w", errInvalidFilter)
	}
	if s.students == nil {
		return nil, fmt.Errorf("supervision sheet: repos not configured")
	}
	date := in.Date
	if date.IsZero() {
		date = timezone.TodayDate()
	}
	student, err := s.students.FindSheetStudent(ctx, in.StudentID)
	if err != nil {
		return nil, fmt.Errorf("supervision sheet: load student: %w", err)
	}
	if student == nil {
		return nil, errInvalidFilter
	}
	sheet, err := s.supervisionSheetFor(ctx, student, date, in.CompanionBoundary)
	if err != nil {
		return nil, err
	}
	if err := s.recordSupervisionSheetAudit(ctx, sheet, in.ActorAccountID, in.ActorRole); err != nil {
		return nil, err
	}
	return sheet, nil
}

// supervisionSheetFor reads the child's name, the effective times, the
// departure, the day status and the contacts of the date.
func (s *dayReports) supervisionSheetFor(ctx context.Context, student *SheetStudent, date timezone.Date, boundary []int64) (*classday.SupervisionStudentSheet, error) {
	sheet := &classday.SupervisionStudentSheet{
		StudentID:         student.ID,
		SchoolClass:       student.SchoolClass,
		Date:              date,
		PickupContacts:    []classday.SupervisionContact{},
		EmergencyContacts: []classday.SupervisionContact{},
	}
	persons, err := s.students.PersonsByID(ctx, []int64{student.PersonID})
	if err != nil {
		return nil, fmt.Errorf("supervision sheet: load person: %w", err)
	}
	if person, ok := persons[student.PersonID]; ok {
		sheet.FirstName = person.FirstName
		sheet.LastName = person.LastName
	}
	studentIDs := []int64{student.ID}
	facts := newClassDayFacts()
	if err := s.classDayEffectiveTimes(ctx, studentIDs, date, &facts); err != nil {
		return nil, err
	}
	sheet.Arrival = facts.arrivals[student.ID]
	sheet.Pickup = facts.pickups[student.ID]
	if sheet.Departure, err = s.supervisionDeparture(ctx, student, date, boundary); err != nil {
		return nil, err
	}
	if sheet.Status, err = s.supervisionStatus(ctx, student.ID, date, &facts); err != nil {
		return nil, err
	}
	if sheet.PickupContacts, sheet.EmergencyContacts, err = s.supervisionContacts(ctx, student.ID); err != nil {
		return nil, err
	}
	return sheet, nil
}

// supervisionStatus is the reported day status, or "cancelled" when the
// care day was called off.
func (s *dayReports) supervisionStatus(ctx context.Context, studentID int64, date timezone.Date, facts *classDayFacts) (string, error) {
	studentIDs := []int64{studentID}
	statuses, _, err := s.classDayStatuses(ctx, studentIDs, date)
	if err != nil {
		return "", err
	}
	if status := statuses[studentID]; status != "" {
		return status, nil
	}
	cancelled, err := s.classDayCancellations(ctx, studentIDs, date, facts)
	if err != nil {
		return "", err
	}
	if cancelled[studentID] {
		return studentStatusDayCancelled, nil
	}
	return "", nil
}

// supervisionDeparture renders today's departure with the block roster as the
// disclosure boundary for companion names. A companion binding that is not
// wired costs the names in brackets, but a lookup failure fails the sheet
// closed so a partial departure instruction is never presented as complete.
func (s *dayReports) supervisionDeparture(ctx context.Context, student *SheetStudent, date timezone.Date, boundary []int64) (string, error) {
	weekday := classDayWeekdayKey(date)
	if weekday == "" {
		return classDayDepartureUnknown, nil
	}
	onSheet := make(map[int64]bool, len(boundary))
	for _, id := range boundary {
		onSheet[id] = true
	}
	links, err := s.companionLinks(ctx, []int64{student.ID}, "class roster report")
	if err != nil {
		return "", fmt.Errorf("supervision sheet: load departure companions: %w", err)
	}
	modes := weekdayDepartureModes(student.AllowedDepartureModes, student.DepartureDays, weekday)
	if departure := classDayDeparture(modes, weekday, links[student.ID], onSheet); departure != "" {
		return departure, nil
	}
	return classDayDepartureUnknown, nil
}

// supervisionContact accumulates one guardian's rows (one per phone number).
type supervisionContact struct {
	contact     classday.SupervisionContact
	canPickup   bool
	isEmergency bool
	order       int
}

// supervisionContacts splits the child's guardians into the two questions a
// supervisor actually asks: who may collect this child, and whom do I call.
//
// A guardian can answer both, and then appears in both lists — the
// alternative is one merged list where the supervisor has to decode flags
// under pressure. Contacts that are neither are left out entirely.
func (s *dayReports) supervisionContacts(ctx context.Context, studentID int64) (pickup, emergency []classday.SupervisionContact, err error) {
	pickup = []classday.SupervisionContact{}
	emergency = []classday.SupervisionContact{}
	if s.emergencyContacts == nil {
		return pickup, emergency, nil
	}
	rows, err := s.emergencyContacts.EmergencyContactRows(ctx, []int64{studentID})
	if err != nil {
		return nil, nil, fmt.Errorf("supervision sheet: load guardians: %w", err)
	}
	for _, entry := range foldSupervisionContacts(rows, studentID) {
		if entry.contact.Name == "" && len(entry.contact.Phones) == 0 {
			continue
		}
		if entry.canPickup {
			pickup = append(pickup, entry.contact)
		}
		if entry.isEmergency {
			// The pickup note is an instruction for the handover, not for the
			// emergency call — repeating it there only adds noise.
			emergencyContact := entry.contact
			emergencyContact.Note = ""
			emergency = append(emergency, emergencyContact)
		}
	}
	return pickup, emergency, nil
}

// foldSupervisionContacts folds the phone-number rows back into one contact
// per guardian, keeping the rows' priority order.
func foldSupervisionContacts(rows []EmergencyContactRow, studentID int64) []*supervisionContact {
	byGuardian := map[int64]*supervisionContact{}
	for _, row := range rows {
		if row.StudentID != studentID || row.GuardianProfileID <= 0 {
			continue
		}
		entry, ok := byGuardian[row.GuardianProfileID]
		if !ok {
			entry = &supervisionContact{order: len(byGuardian)}
			entry.contact.Name = strings.TrimSpace(row.FirstName + " " + row.LastName)
			entry.contact.Phones = []string{}
			// Raw wire value ("parent", "guardian", …). The German label is the
			// frontend's RELATIONSHIP_TYPES table, which every other guardian
			// screen already renders from — a second translation here would
			// drift from it the first time somebody adds a type.
			entry.contact.Relationship = strings.ToLower(strings.TrimSpace(row.RelationshipType))
			entry.contact.Note = strings.TrimSpace(row.PickupNotes)
			byGuardian[row.GuardianProfileID] = entry
		}
		entry.canPickup = entry.canPickup || row.CanPickup
		entry.isEmergency = entry.isEmergency || row.IsEmergencyContact
		entry.contact.Phones = splitUnique(append(entry.contact.Phones, row.PhoneNumber)...)
	}
	entries := make([]*supervisionContact, 0, len(byGuardian))
	for _, entry := range byGuardian {
		entries = append(entries, entry)
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].order < entries[j].order })
	return entries
}

// splitUnique splits every value on ";" and keeps each trimmed, non-empty
// part once (case-insensitive, first spelling wins), in order. The phones
// stay one entry per number: the portal turns each into a tel: link.
func splitUnique(values ...string) []string {
	seen := make(map[string]bool, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		for _, part := range strings.Split(value, ";") {
			trimmed := strings.TrimSpace(part)
			key := strings.ToLower(trimmed)
			if trimmed == "" || seen[key] {
				continue
			}
			seen[key] = true
			result = append(result, trimmed)
		}
	}
	return result
}

// recordSupervisionSheetAudit writes the GDPR access log for one child's
// supervision sheet.
//
// Deliberately NOT deduplicated the way the class day view is: that view
// revalidates itself every few minutes on its own, while this sheet only ever
// opens because a person tapped a child's name. Every tap is a decision, and
// every decision belongs in the log.
func (s *dayReports) recordSupervisionSheetAudit(ctx context.Context, sheet *classday.SupervisionStudentSheet, actorAccountID int64, actorRole string) error {
	if s.accessLog == nil {
		return nil
	}
	const errPrefix = "supervision sheet audit"
	entry, err := newAccessRecord(errPrefix, actorAccountID, actorRole, sheet.Date.BerlinMidnight(), sheet.Date.EndOfDay(), time.Now())
	if err != nil {
		return err
	}
	studentID := sheet.StudentID
	entry.StudentID = &studentID
	entry.Metadata = map[string]any{
		"report":                  "supervision_student_sheet",
		"student_id":              sheet.StudentID,
		"date":                    sheet.Date.String(),
		"pickup_contact_count":    len(sheet.PickupContacts),
		"emergency_contact_count": len(sheet.EmergencyContacts),
	}
	if err := s.accessLog.RecordSupervisionSheet(ctx, entry); err != nil {
		return fmt.Errorf("%s write: %w", errPrefix, err)
	}
	return nil
}
