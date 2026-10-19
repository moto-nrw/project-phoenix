package importpkg

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/auditlog/consents"
	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
	"github.com/moto-nrw/project-phoenix/modules/dataimport/ports"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// This file holds the owner-backed read and write paths of the student
// import (#2708). Every row is committed through People Directory (person,
// student, guardians), Care Plan (weekly schedules), Student Presence
// (retention consent) and the Audit platform (consent history); the import
// itself never touches a table.

// privacyConsentPolicyVersion is the policy version stamped on consents the
// import records.
const privacyConsentPolicyVersion = "1.0"

// Create files a new child with every related record through the owners.
// A PostgreSQL savepoint keeps the row atomic inside the surrounding import
// transaction: a failure after the person was created rolls that person back
// while the other rows of the batch survive.
func (c *StudentImportConfig) Create(ctx context.Context, row importModels.StudentImportRow) (int64, error) {
	if c.Transactions.HasTransaction(ctx) {
		var studentID int64
		err := c.Transactions.Savepoint(ctx, func(savepointCtx context.Context) error {
			var err error
			studentID, err = c.createAllEntities(savepointCtx, row)
			return err
		})
		return studentID, err
	}
	return c.createAllEntities(ctx, row)
}

func (c *StudentImportConfig) createAllEntities(ctx context.Context, row importModels.StudentImportRow) (int64, error) {
	person, err := c.createPersonFromRow(ctx, row)
	if err != nil {
		return 0, err
	}
	studentID, err := c.createStudentFromRow(ctx, person.ID, row)
	if err != nil {
		return 0, err
	}
	if err := c.createGuardianRelationships(ctx, studentID, row.Guardians); err != nil {
		return 0, err
	}
	if err := c.createPrivacyConsentIfNeeded(ctx, studentID, row); err != nil {
		return 0, err
	}
	if err := c.createArrivalSchedules(ctx, studentID, row.ArrivalSchedules); err != nil {
		return 0, err
	}
	if err := c.createPickupSchedules(ctx, studentID, row.PickupSchedules); err != nil {
		return 0, err
	}
	return studentID, nil
}

func (c *StudentImportConfig) createPersonFromRow(ctx context.Context, row importModels.StudentImportRow) (ports.Person, error) {
	birthday, _ := parseOptionalDate(row.Birthday)
	person, err := c.Persons.CreatePerson(ctx, ports.CreatePerson{
		FirstName: strings.TrimSpace(row.FirstName),
		LastName:  strings.TrimSpace(row.LastName),
		Birthday:  calendarDateString(birthday),
		TagID:     importModels.OptionalCell(row.TagID),
	})
	if err != nil {
		return ports.Person{}, fmt.Errorf("create person: %w", err)
	}
	return person, nil
}

// createStudentFromRow creates the student under the owner's class-write
// gate and then patches the profile columns the enrollment command does not
// carry (health, notes, address, group, departure plan, consent dates).
func (c *StudentImportConfig) createStudentFromRow(ctx context.Context, personID int64, row importModels.StudentImportRow) (int64, error) {
	enrolledFrom := parseOptionalImportCalendarDate(row.EnrolledFrom)
	input := ports.EnrollmentStudent{
		PersonID:      personID,
		SchoolClass:   strings.TrimSpace(row.SchoolClass),
		EnrolledFrom:  calendarDateString(enrolledFrom),
		EnrolledUntil: calendarDateString(parseOptionalImportCalendarDate(row.EnrolledUntil)),
	}
	// A future enrollment start means the student isn't active yet. Mark them
	// pending so the activate-students scheduler flips them to active once
	// enrolled_from arrives (mirrors the parent-enrollment flow).
	if enrollmentStartsInFuture(enrolledFrom) {
		input.Status = ports.StudentStatusPending
	}
	departure, err := importDeparturePatch(ports.AllowedDepartureModesFromDeparture(departurePlanFromImportRow(row)), boundedNotePtr(row.DepartureCompanionNote))
	if err != nil {
		return 0, fmt.Errorf("create student: %w", err)
	}
	patch := departure
	patch.ExtraInfoSet, patch.ExtraInfo = true, importModels.OptionalCell(row.ExtraInfo)
	patch.PermanentHint = importModels.OptionalCell(row.SupervisorNotes)
	patch.HealthInfoSet, patch.HealthInfo = true, importModels.OptionalCell(row.HealthInfo)
	patch.AddressSet = true
	patch.AddressStreet = importModels.OptionalCell(row.AddressStreet)
	patch.AddressCity = importModels.OptionalCell(row.AddressCity)
	patch.AddressPostalCode = importModels.OptionalCell(row.AddressPostalCode)
	patch.GroupIDSet, patch.GroupID = true, row.GroupID
	patch.AGBAcceptedAtSet, patch.AGBAcceptedAt = true, parseOptionalImportDate(row.AGBAcceptedAt)
	patch.DataProcessingAcceptedAtSet, patch.DataProcessingAcceptedAt = true, parseOptionalImportDate(row.DataProcessingAcceptedAt)
	patch.EmailContactAcceptedAtSet, patch.EmailContactAcceptedAt = true, parseOptionalImportDate(row.EmailContactAcceptedAt)
	// Photo consent date is set; "given_by" is intentionally left nil on import.
	patch.PhotoConsentGivenAtSet, patch.PhotoConsentGivenAt = true, parseOptionalImportDate(row.PhotoConsentGivenAt)
	input.InitialProfile = &patch
	created, err := c.Students.CreateEnrollmentStudent(ctx, input)
	if err != nil {
		return 0, fmt.Errorf("create student: %w", err)
	}

	if c.ConsentHistory != nil {
		after := consentSnapshot(created.ID, patch.AGBAcceptedAt, patch.DataProcessingAcceptedAt, patch.EmailContactAcceptedAt, patch.PhotoConsentGivenAt)
		if err := ports.ObserveCommand(ctx, "audit-platform", "record_consent_transitions", func() error {
			return c.ConsentHistory.RecordTransitions(ctx, nil, after, consents.ConsentSourceImport, nil, time.Now())
		}); err != nil {
			return 0, fmt.Errorf("create student consent history: %w", err)
		}
	}
	return created.ID, nil
}

// consentSnapshot is the minimal student view the consent-history recorder
// compares: the four consent timestamps of one persisted student.
func consentSnapshot(studentID int64, agb, dataProcessing, emailContact, photo *time.Time) *consents.ConsentSnapshot {
	return &consents.ConsentSnapshot{
		StudentID:                studentID,
		AGBAcceptedAt:            agb,
		DataProcessingAcceptedAt: dataProcessing,
		EmailContactAcceptedAt:   emailContact,
		PhotoConsentGivenAt:      photo,
	}
}

// importDeparturePatch resolves the departure plan of a row into the owner
// patch: the allowed modes are derived from the per-day plan, the legacy
// mirrors (bus_days, pickup_days, pickup_status) are computed from them, and
// the companion note is kept only while a day is accompanied. The model
// rules (bounded note, note required for an accompanied day) are enforced
// before anything is written.
func importDeparturePatch(modes ports.AllowedDepartureModes, note *string) (ports.EnrollmentProfilePatch, error) {
	allowed := modes.Normalize()
	normalizedNote, err := ports.NormalizeCompanionNote(allowed.DepartureDays(), allowed, note, nil)
	if err != nil {
		return ports.EnrollmentProfilePatch{}, err
	}
	if !allowed.HasMode(ports.DepartureAccompanied) {
		normalizedNote = nil
	}
	patch := ports.EnrollmentProfilePatch{
		DepartureSet:           true,
		AllowedDepartureModes:  map[string][]string{},
		DepartureDays:          map[string]string{},
		BusDays:                map[string]bool(allowed.BusDays()),
		PickupDays:             map[string]bool(allowed.PickupDays()),
		PickupStatus:           allowed.LegacyPickupStatus(),
		DepartureCompanionNote: normalizedNote,
	}
	for day, modes := range allowed {
		for _, mode := range modes {
			patch.AllowedDepartureModes[day] = append(patch.AllowedDepartureModes[day], string(mode))
		}
	}
	for day, mode := range allowed.DepartureDays() {
		patch.DepartureDays[day] = string(mode)
	}
	return patch, nil
}

func departureDaysFromRecord(record ports.EnrollmentRecord) ports.DepartureDays {
	days := ports.DepartureDays{}
	for day, mode := range record.DepartureDays {
		days[day] = ports.DepartureMode(mode)
	}
	return days
}

// allowedModesFromRecord reads the stored mode sets of a child. A record
// written before the unified plan existed carries none; its exclusive
// per-day plan is then the source.
func allowedModesFromRecord(record ports.EnrollmentRecord) ports.AllowedDepartureModes {
	modes := ports.AllowedDepartureModes{}
	for day, values := range record.AllowedDepartureModes {
		for _, value := range values {
			modes[day] = append(modes[day], ports.DepartureMode(value))
		}
	}
	if !modes.HasAny() {
		return ports.AllowedDepartureModesFromDeparture(departureDaysFromRecord(record))
	}
	return modes
}

func calendarDateString(d *timezone.Date) string {
	if d == nil {
		return ""
	}
	return d.String()
}

// --- guardians ---

// --- privacy consent ---

// createPrivacyConsentIfNeeded records the retention consent when the row
// accepts privacy or names a valid retention period (>0).
func (c *StudentImportConfig) createPrivacyConsentIfNeeded(ctx context.Context, studentID int64, row importModels.StudentImportRow) error {
	if !row.PrivacyAccepted && row.DataRetentionDays <= 0 {
		return nil
	}
	if c.PrivacyConsents == nil {
		return errors.New("privacy consent capability is not wired")
	}
	if _, err := c.PrivacyConsents.RecordPrivacyConsent(ctx, buildPrivacyConsent(studentID, row)); err != nil {
		return fmt.Errorf("create privacy consent: %w", err)
	}
	return nil
}

func buildPrivacyConsent(studentID int64, row importModels.StudentImportRow) ports.PrivacyConsent {
	consent := ports.PrivacyConsent{
		StudentID:         studentID,
		PolicyVersion:     privacyConsentPolicyVersion,
		Accepted:          row.PrivacyAccepted,
		DataRetentionDays: validateRetentionDays(row.DataRetentionDays),
	}
	if row.PrivacyAccepted {
		now := time.Now()
		consent.AcceptedAt = &now
	}
	return consent
}

// createPrivacyConsentIfMissing adds the consent row in update mode only when
// the child has none yet; an existing consent is never rewritten by a file.
func (c *StudentImportConfig) createPrivacyConsentIfMissing(ctx context.Context, studentID int64, row importModels.StudentImportRow) error {
	if !row.PrivacyAccepted || c.PrivacyConsents == nil {
		return nil
	}
	consents, err := c.PrivacyConsents.ListPrivacyConsents(ctx, studentID)
	if err != nil {
		return fmt.Errorf("Datenschutz-Einwilligung laden: %w", err) //nolint:staticcheck // ST1005: user-facing German message
	}
	if len(consents) > 0 {
		return nil
	}
	return c.createPrivacyConsentIfNeeded(ctx, studentID, row)
}

// --- update ---

// Update patches an existing student (#2600). Empty cells never clear a
// stored value; only what the row carries is written. Guardians are merged
// (matched by e-mail, new ones linked), schedules are replaced per weekday
// given, privacy consent is only created when none exists yet.
func (c *StudentImportConfig) Update(ctx context.Context, studentID int64, row importModels.StudentImportRow) error {
	return c.Transactions.Savepoint(ctx, func(ctx context.Context) error {
		// The owner takes the shared class gate and the row lock before the
		// import reads the profile it is about to patch.
		record, err := c.Students.ReadEnrollmentStudent(ctx, studentID, "update")
		if err != nil {
			if errors.Is(err, ports.ErrStudentNotFound) {
				return errors.New("Kind nicht gefunden") //nolint:staticcheck // ST1005: user-facing German message
			}
			return fmt.Errorf("Kind laden: %w", err) //nolint:staticcheck // ST1005: user-facing German message
		}
		return c.updateAllEntities(ctx, record, row)
	})
}

func (c *StudentImportConfig) updateAllEntities(ctx context.Context, record ports.EnrollmentRecord, row importModels.StudentImportRow) error {

	person, err := c.Persons.FindPerson(ctx, record.PersonID)
	if err != nil {
		if errors.Is(err, ports.ErrPersonNotFound) {
			return errors.New("Person nicht gefunden") //nolint:staticcheck // ST1005: user-facing German message
		}
		return fmt.Errorf("Person laden: %w", err) //nolint:staticcheck // ST1005: user-facing German message
	}

	if err := c.updatePersonFromRow(ctx, person, row); err != nil {
		return err
	}
	if err := c.updateStudentFromRow(ctx, record, row); err != nil {
		return err
	}
	if err := c.mergeGuardianRelationships(ctx, record.ID, row.Guardians); err != nil {
		return err
	}
	if err := c.upsertArrivalSchedules(ctx, record.ID, row.ArrivalSchedules); err != nil {
		return err
	}
	if err := c.upsertPickupSchedules(ctx, record.ID, row.PickupSchedules); err != nil {
		return err
	}
	return c.createPrivacyConsentIfMissing(ctx, record.ID, row)
}

func (c *StudentImportConfig) updatePersonFromRow(ctx context.Context, person ports.Person, row importModels.StudentImportRow) error {
	input := ports.UpdatePerson{ID: person.ID, FirstName: person.FirstName, LastName: person.LastName, Birthday: person.Birthday, TagID: person.TagID, AccountID: person.AccountID}
	changed := false
	if firstName := strings.TrimSpace(row.FirstName); firstName != "" && person.FirstName != firstName {
		input.FirstName = firstName
		changed = true
	}
	if lastName := strings.TrimSpace(row.LastName); lastName != "" && person.LastName != lastName {
		input.LastName = lastName
		changed = true
	}
	if birthday, _ := parseOptionalDate(row.Birthday); birthday != nil && person.Birthday != birthday.String() {
		input.Birthday = birthday.String()
		changed = true
	}
	if row.TagID != "" && !ptrEquals(person.TagID, row.TagID) {
		input.TagID = importModels.OptionalCell(row.TagID)
		changed = true
	}
	if !changed {
		return nil
	}
	if _, err := c.Persons.UpdatePerson(ctx, input); err != nil {
		return fmt.Errorf("Person aktualisieren: %w", err) //nolint:staticcheck // ST1005: user-facing German message
	}
	return nil
}

// renewalFromRow is the class, lifecycle window and status an update row
// leaves on an existing child: only the cells the row carries change.
func renewalFromRow(record ports.EnrollmentRecord, row importModels.StudentImportRow) ports.EnrollmentStudent {
	renewal := ports.EnrollmentStudent{
		PersonID: record.PersonID, SchoolClass: record.SchoolClass, Status: record.Status,
		EnrolledFrom: record.EnrolledFrom, EnrolledUntil: record.EnrolledUntil,
	}
	if class := strings.TrimSpace(row.SchoolClass); class != "" {
		renewal.SchoolClass = class
	}
	if d := parseOptionalImportCalendarDate(row.EnrolledFrom); d != nil {
		renewal.EnrolledFrom = d.String()
		if enrollmentStartsInFuture(d) {
			renewal.Status = ports.StudentStatusPending
		} else if renewal.Status == ports.StudentStatusPending {
			renewal.Status = ports.StudentStatusActive
		}
	}
	if d := parseOptionalImportCalendarDate(row.EnrolledUntil); d != nil {
		renewal.EnrolledUntil = d.String()
	}
	return renewal
}

// newEnrollmentStatus is the status a created child starts with: pending
// while its care starts in the future, active otherwise.
func newEnrollmentStatus(enrolledFrom *timezone.Date) string {
	if enrollmentStartsInFuture(enrolledFrom) {
		return ports.StudentStatusPending
	}
	return ports.StudentStatusActive
}

var errChildQuotaRuleUnbound = errors.New("student import: child quota rule is not bound")

// AddsToChildQuota says whether applying the row raises the Kontingentzahl
// (#3571): a new child that counts, or an update that moves a child who does
// not count now into the count, for example by resuming ended care.
func (c *StudentImportConfig) AddsToChildQuota(ctx context.Context, row importModels.StudentImportRow, existingID *int64) (bool, error) {
	if c.CountsTowardChildQuota == nil {
		return false, errChildQuotaRuleUnbound
	}
	day := timezone.TodayDate().String()
	if existingID == nil {
		enrolledFrom := parseOptionalImportCalendarDate(row.EnrolledFrom)
		return c.CountsTowardChildQuota(newEnrollmentStatus(enrolledFrom),
			calendarDateString(enrolledFrom), calendarDateString(parseOptionalImportCalendarDate(row.EnrolledUntil)), day), nil
	}
	record, err := c.Students.ReadEnrollmentStudent(ctx, *existingID, "")
	if err != nil {
		return false, fmt.Errorf("read student for child quota: %w", err)
	}
	after := renewalFromRow(record, row)
	return !c.CountsTowardChildQuota(record.Status, record.EnrolledFrom, record.EnrolledUntil, day) &&
		c.CountsTowardChildQuota(after.Status, after.EnrolledFrom, after.EnrolledUntil, day), nil
}

func (c *StudentImportConfig) updateStudentFromRow(ctx context.Context, record ports.EnrollmentRecord, row importModels.StudentImportRow) error {
	// Class, lifecycle window and status go through the renewal command.
	renewal := renewalFromRow(record, row)
	if renewal.SchoolClass != record.SchoolClass || renewal.Status != record.Status ||
		renewal.EnrolledFrom != record.EnrolledFrom || renewal.EnrolledUntil != record.EnrolledUntil {
		if err := c.Students.RenewEnrollmentStudent(ctx, record.ID, renewal); err != nil {
			return fmt.Errorf("Kind aktualisieren: %w", err) //nolint:staticcheck // ST1005: user-facing German message
		}
	}

	// Everything else is a profile patch; only the cells the row carries
	// are written.
	patch, changed := studentProfileTextPatch(record, row)
	after, consentChanged := applyStudentConsentDates(record, row, &patch)
	changed = consentChanged || changed

	departureChanged, err := applyStudentDeparture(record, row, &patch)
	if err != nil {
		return err
	}
	changed = departureChanged || changed

	if changed {
		if err := c.Students.ApplyEnrollmentProfile(ctx, record.ID, patch); err != nil {
			return fmt.Errorf("Kind aktualisieren: %w", err) //nolint:staticcheck // ST1005: user-facing German message
		}
	}
	if c.ConsentHistory != nil {
		before := consentSnapshot(record.ID, record.AGBAcceptedAt, record.DataProcessingAcceptedAt, record.EmailContactAcceptedAt, record.PhotoConsentGivenAt)
		if err := ports.ObserveCommand(ctx, "audit-platform", "record_consent_transitions", func() error {
			return c.ConsentHistory.RecordTransitions(ctx, before, after, consents.ConsentSourceImport, nil, time.Now())
		}); err != nil {
			return fmt.Errorf("Einwilligungsverlauf aktualisieren: %w", err) //nolint:staticcheck // ST1005: user-facing German message
		}
	}
	return nil
}

// keepUnlessGiven returns the row value when the cell is filled, otherwise
// the stored value.
func keepUnlessGiven(current *string, cell string) *string {
	if strings.TrimSpace(cell) != "" {
		return importModels.OptionalCell(cell)
	}
	return current
}

// linkedGuardianProfile is one guardian already linked to the child, together
// with the phone numbers stored for it, so a re-import can recognise the
// guardian without an e-mail address.
type linkedGuardianProfile struct {
	profile ports.Guardian
	phones  []string
}

// --- weekly schedules ---

func studentProfileTextPatch(record ports.EnrollmentRecord, row importModels.StudentImportRow) (ports.EnrollmentProfilePatch, bool) {
	patch := ports.EnrollmentProfilePatch{}
	changed := false
	setStr := func(set *bool, dst **string, v string) {
		if strings.TrimSpace(v) != "" {
			*set, *dst = true, importModels.OptionalCell(v)
			changed = true
		}
	}
	setStr(&patch.ExtraInfoSet, &patch.ExtraInfo, row.ExtraInfo)
	if strings.TrimSpace(row.SupervisorNotes) != "" {
		patch.PermanentHint = importModels.OptionalCell(row.SupervisorNotes)
		changed = true
	}
	setStr(&patch.HealthInfoSet, &patch.HealthInfo, row.HealthInfo)
	if strings.TrimSpace(row.AddressStreet) != "" || strings.TrimSpace(row.AddressCity) != "" || strings.TrimSpace(row.AddressPostalCode) != "" {
		patch.AddressSet = true
		patch.AddressStreet = keepUnlessGiven(record.AddressStreet, row.AddressStreet)
		patch.AddressCity = keepUnlessGiven(record.AddressCity, row.AddressCity)
		patch.AddressPostalCode = keepUnlessGiven(record.AddressPostalCode, row.AddressPostalCode)
		changed = true
	}
	if row.GroupID != nil {
		patch.GroupIDSet, patch.GroupID = true, row.GroupID
		changed = true
	}
	return patch, changed
}

func applyStudentConsentDates(record ports.EnrollmentRecord, row importModels.StudentImportRow, patch *ports.EnrollmentProfilePatch) (*consents.ConsentSnapshot, bool) {
	changed := false
	setDate := func(set *bool, dst **time.Time, v string) {
		if t := parseOptionalImportDate(v); t != nil {
			*set, *dst = true, t
			changed = true
		}
	}
	after := consentSnapshot(record.ID, record.AGBAcceptedAt, record.DataProcessingAcceptedAt, record.EmailContactAcceptedAt, record.PhotoConsentGivenAt)
	setDate(&patch.AGBAcceptedAtSet, &patch.AGBAcceptedAt, row.AGBAcceptedAt)
	setDate(&patch.DataProcessingAcceptedAtSet, &patch.DataProcessingAcceptedAt, row.DataProcessingAcceptedAt)
	setDate(&patch.EmailContactAcceptedAtSet, &patch.EmailContactAcceptedAt, row.EmailContactAcceptedAt)
	setDate(&patch.PhotoConsentGivenAtSet, &patch.PhotoConsentGivenAt, row.PhotoConsentGivenAt)
	if patch.AGBAcceptedAtSet {
		after.AGBAcceptedAt = patch.AGBAcceptedAt
	}
	if patch.DataProcessingAcceptedAtSet {
		after.DataProcessingAcceptedAt = patch.DataProcessingAcceptedAt
	}
	if patch.EmailContactAcceptedAtSet {
		after.EmailContactAcceptedAt = patch.EmailContactAcceptedAt
	}
	if patch.PhotoConsentGivenAtSet {
		after.PhotoConsentGivenAt = patch.PhotoConsentGivenAt
	}

	return after, changed
}

func applyStudentDeparture(record ports.EnrollmentRecord, row importModels.StudentImportRow, patch *ports.EnrollmentProfilePatch) (bool, error) {
	changed := false
	// Gehweise only when the file carries the per-day columns; the legacy
	// Bus/Abholstatus columns are not consulted in update mode because their
	// empty state is indistinguishable from "no". A companion note alone is
	// applied to the stored plan (and dropped when no day is accompanied).
	note := boundedNotePtr(row.DepartureCompanionNote)
	if row.DepartureDays != nil || note != nil {
		// A file that names Gehweise days re-derives the whole plan from the
		// exclusive per-day modes it carries, exactly as the enrollment
		// decision does. A note-only row keeps the stored mode sets, so a day
		// that allows several ways home does not collapse to one.
		allowed := allowedModesFromRecord(record)
		if row.DepartureDays != nil {
			days := departureDaysFromRecord(record)
			for weekday, mode := range departurePlanFromImportRow(row) {
				days[weekday] = mode
			}
			allowed = ports.AllowedDepartureModesFromDeparture(days)
		}
		if note == nil {
			note = record.DepartureCompanionNote
		}
		departure, err := importDeparturePatch(allowed, note)
		if err != nil {
			return false, fmt.Errorf("Kind aktualisieren: %w", err) //nolint:staticcheck // ST1005: user-facing German message
		}
		patch.DepartureSet = true
		patch.AllowedDepartureModes = departure.AllowedDepartureModes
		patch.DepartureDays = departure.DepartureDays
		patch.BusDays = departure.BusDays
		patch.PickupDays = departure.PickupDays
		patch.PickupStatus = departure.PickupStatus
		patch.DepartureCompanionNote = departure.DepartureCompanionNote
		changed = true
	}

	return changed, nil
}
