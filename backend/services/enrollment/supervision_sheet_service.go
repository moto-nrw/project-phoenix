package enrollment

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/strutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

// SupervisionContact is one adult a supervisor may need during an Aufsicht:
// the name, how they relate to the child, and a phone number.
//
// Deliberately no e-mail address: this sheet is opened when somebody has to be
// reached NOW, and an inbox is not a way to reach anybody. Everything the sheet
// does not need is a detail a Lehrkraft has no business seeing.
type SupervisionContact struct {
	Name string `json:"name"`
	// Relationship is the stored relationship type ("parent", "guardian", …),
	// not a display string: the portal renders it through the same table as
	// every other guardian screen.
	Relationship string `json:"relationship,omitempty"`
	Phone        string `json:"phone,omitempty"`
	// Note is the pickup remark stored on the relationship ("nur mit
	// Vollmacht", "holt immer freitags ab"). Empty for most contacts.
	Note string `json:"note,omitempty"`
}

// SupervisionStudentSheet is the per-child information one supervisor needs
// while running the block a child is in (#2527): when and how the child leaves
// today, who may collect them, and whom to call in an emergency.
//
// This is a deliberate widening over the class day view (#1772), which carries
// no guardian names at all. It is bounded three ways instead: the caller must
// be assigned to the block, the child must be on that block's roster, and every
// single call writes a GDPR access-log row naming the child.
type SupervisionStudentSheet struct {
	StudentID   int64         `json:"student_id"`
	FirstName   string        `json:"first_name"`
	LastName    string        `json:"last_name"`
	SchoolClass string        `json:"school_class,omitempty"`
	Date        timezone.Date `json:"date"`
	// Arrival / Pickup are the effective times of the day ("07:45" / "15:30"),
	// empty when the plan names none.
	Arrival string `json:"arrival,omitempty"`
	Pickup  string `json:"pickup,omitempty"`
	// Departure renders how the child goes home today ("Bus", "Abholung",
	// "Geht alleine"). Missing plan data reads as "Keine Angabe", never as
	// permission to send the child off alone.
	Departure string `json:"departure"`
	// Status is the reported day status ("sick" / "excused" / "class_trip"),
	// empty when none is reported. Cancelled care ("kommt heute nicht") is
	// reported as StatusDayCancelled, matching the class day view.
	Status            string               `json:"status,omitempty"`
	PickupContacts    []SupervisionContact `json:"pickup_contacts"`
	EmergencyContacts []SupervisionContact `json:"emergency_contacts"`
}

// SupervisionSheetInput parameterizes the sheet.
type SupervisionSheetInput struct {
	StudentID int64
	Date      timezone.Date
	// CompanionBoundary is the set of student IDs the caller ALREADY sees —
	// the roster of the block. An accompanied departure ("läuft mit …") names
	// only children from this set, exactly like the class day view names only
	// children of the class being served: a Laufgemeinschaft may pair the
	// child with one from a class the caller never gets to see.
	CompanionBoundary []int64
	ActorAccountID    int64
	ActorRole         string
}

// SupervisionStudentSheet builds the sheet and records the access.
//
// The authorization ("is this caller assigned to a block holding this child")
// happens BEFORE this call, in the timetable operations service — this method
// deliberately owns no access rule of its own, so there is exactly one place
// where the assignment is checked.
func (s *reportService) SupervisionStudentSheet(ctx context.Context, in SupervisionSheetInput) (*SupervisionStudentSheet, error) {
	if in.StudentID <= 0 {
		return nil, fmt.Errorf("supervision sheet: student id required: %w", ErrReportInvalidFilter)
	}
	if s.StudentRepo == nil || s.PersonRepo == nil {
		return nil, fmt.Errorf("supervision sheet: repos not configured")
	}
	date := in.Date
	if date.IsZero() {
		date = timezone.TodayDate()
	}

	student, err := s.StudentRepo.FindByID(ctx, in.StudentID)
	if err != nil {
		return nil, fmt.Errorf("supervision sheet: load student: %w", err)
	}
	if student == nil {
		return nil, ErrReportInvalidFilter
	}

	sheet := &SupervisionStudentSheet{
		StudentID:         student.ID,
		SchoolClass:       student.SchoolClass,
		Date:              date,
		PickupContacts:    []SupervisionContact{},
		EmergencyContacts: []SupervisionContact{},
	}

	persons, err := s.PersonRepo.FindByIDs(ctx, []int64{student.PersonID})
	if err != nil {
		return nil, fmt.Errorf("supervision sheet: load person: %w", err)
	}
	for _, person := range persons {
		if person != nil && person.ID == student.PersonID {
			sheet.FirstName = person.FirstName
			sheet.LastName = person.LastName
		}
	}

	studentIDs := []int64{student.ID}

	arrivals, pickups, err := s.classDayEffectiveTimes(ctx, studentIDs, date)
	if err != nil {
		return nil, err
	}
	sheet.Arrival = arrivals[student.ID]
	sheet.Pickup = pickups[student.ID]

	sheet.Departure = s.supervisionDeparture(ctx, student, date, in.CompanionBoundary)

	statuses, err := s.classDayStatuses(ctx, studentIDs, date)
	if err != nil {
		return nil, err
	}
	sheet.Status = statuses[student.ID]
	if sheet.Status == "" {
		cancelled, _, cancelErr := s.classDayCancellations(ctx, studentIDs, date)
		if cancelErr != nil {
			return nil, cancelErr
		}
		if cancelled[student.ID] {
			sheet.Status = StudentStatusDayCancelled
		}
	}

	pickupContacts, emergencyContacts, err := s.supervisionContacts(ctx, student.ID)
	if err != nil {
		return nil, err
	}
	sheet.PickupContacts = pickupContacts
	sheet.EmergencyContacts = emergencyContacts

	if err := s.recordSupervisionSheetAudit(ctx, sheet, in.ActorAccountID, in.ActorRole); err != nil {
		return nil, err
	}
	return sheet, nil
}

// supervisionDeparture renders today's departure with the block roster as the
// disclosure boundary for companion names. A companion repo that is not wired
// costs the names in brackets, never the departure itself.
func (s *reportService) supervisionDeparture(ctx context.Context, student *userModels.Student, date timezone.Date, boundary []int64) string {
	weekday := classDayWeekdayKey(date)
	if weekday == "" {
		return classDayDepartureUnknown
	}
	onSheet := make(map[int64]bool, len(boundary))
	for _, id := range boundary {
		onSheet[id] = true
	}
	var companions []userModels.CompanionLink
	if links, err := s.classRosterCompanions(ctx, []int64{student.ID}); err == nil {
		companions = links[student.ID]
	}
	departure := classDayDeparture(student, weekday, companions, onSheet)
	if departure == "" {
		return classDayDepartureUnknown
	}
	return departure
}

// supervisionContacts splits the child's guardians into the two questions a
// supervisor actually asks: who may collect this child, and whom do I call.
//
// A guardian can answer both, and then appears in both lists — the alternative
// is one merged list where the supervisor has to decode flags under pressure.
// Contacts that are neither are left out entirely.
func (s *reportService) supervisionContacts(ctx context.Context, studentID int64) (pickup, emergency []SupervisionContact, err error) {
	pickup = []SupervisionContact{}
	emergency = []SupervisionContact{}
	if s.StudentGuardianRepo == nil {
		return pickup, emergency, nil
	}
	rows, err := s.StudentGuardianRepo.ListEmergencyContactRows(ctx, []int64{studentID})
	if err != nil {
		return nil, nil, fmt.Errorf("supervision sheet: load guardians: %w", err)
	}

	// One row per (guardian, phone number): fold the phone numbers back into
	// one contact per guardian, keeping the repository's priority order.
	type accumulated struct {
		contact    SupervisionContact
		canPickup  bool
		isEmergenc bool
		order      int
	}
	byGuardian := map[int64]*accumulated{}
	order := 0
	for _, row := range rows {
		if row.StudentID != studentID || row.GuardianProfileID <= 0 {
			continue
		}
		entry, ok := byGuardian[row.GuardianProfileID]
		if !ok {
			entry = &accumulated{order: order}
			order++
			entry.contact.Name = strings.TrimSpace(row.FirstName.String + " " + row.LastName.String)
			// Raw wire value ("parent", "guardian", …). The German label is the
			// frontend's RELATIONSHIP_TYPES table, which every other guardian
			// screen already renders from — a second translation here would
			// drift from it the first time somebody adds a type.
			entry.contact.Relationship = strings.ToLower(strings.TrimSpace(row.RelationshipType.String))
			entry.contact.Note = strings.TrimSpace(row.PickupNotes.String)
			byGuardian[row.GuardianProfileID] = entry
		}
		entry.canPickup = entry.canPickup || row.CanPickup
		entry.isEmergenc = entry.isEmergenc || row.IsEmergencyContact
		entry.contact.Phone = strutil.JoinUnique(entry.contact.Phone, strings.TrimSpace(row.PhoneNumber.String))
	}

	entries := make([]*accumulated, 0, len(byGuardian))
	for _, entry := range byGuardian {
		entries = append(entries, entry)
	}
	sort.SliceStable(entries, func(i, j int) bool { return entries[i].order < entries[j].order })

	for _, entry := range entries {
		if entry.contact.Name == "" && entry.contact.Phone == "" {
			continue
		}
		if entry.canPickup {
			pickup = append(pickup, entry.contact)
		}
		if entry.isEmergenc {
			// The pickup note is an instruction for the handover, not for the
			// emergency call — repeating it there only adds noise.
			emergencyContact := entry.contact
			emergencyContact.Note = ""
			emergency = append(emergency, emergencyContact)
		}
	}
	return pickup, emergency, nil
}

// recordSupervisionSheetAudit writes the GDPR access log for one child's
// supervision sheet.
//
// Deliberately NOT deduplicated the way the class day view is: that view
// revalidates itself every few minutes on its own, while this sheet only ever
// opens because a person tapped a child's name. Every tap is a decision, and
// every decision belongs in the log.
func (s *reportService) recordSupervisionSheetAudit(ctx context.Context, sheet *SupervisionStudentSheet, actorAccountID int64, actorRole string) error {
	if s.DataAccessLogRepo == nil {
		return nil
	}
	entry, err := exportAuditEntry("supervision sheet audit", actorAccountID, actorRole,
		auditModels.ResourceTypeSupervisionStudentSheet,
		sheet.Date.BerlinMidnight(), sheet.Date.EndOfDay(), time.Now())
	if err != nil {
		return err
	}
	entry.SetMetadata("report", "supervision_student_sheet")
	entry.SetMetadata("student_id", sheet.StudentID)
	entry.SetMetadata("date", sheet.Date.String())
	entry.SetMetadata("pickup_contact_count", len(sheet.PickupContacts))
	entry.SetMetadata("emergency_contact_count", len(sheet.EmergencyContacts))
	return writeExportAudit(ctx, s.DataAccessLogRepo, entry, "supervision sheet audit")
}
