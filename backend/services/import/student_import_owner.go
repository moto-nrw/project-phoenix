package importpkg

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/strutil"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	"github.com/moto-nrw/project-phoenix/models/base"
	importModels "github.com/moto-nrw/project-phoenix/models/import"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/import/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// This file holds the owner-backed read and write paths of the student
// import (#2708). Every row is committed through People Directory (person,
// student, guardians), Care Plan (weekly schedules), Student Presence
// (retention consent) and the Audit platform (consent history); the import
// itself never touches a table.

// privacyConsentPolicyVersion is the policy version stamped on consents the
// import records.
const privacyConsentPolicyVersion = "1.0"

// FindExisting resolves the row to an existing student (duplicate detection
// in create mode, match key in update mode). Keys, in order: the RFID card
// (survives a class change), first + last name + class, and first + last name
// + birthday (the class-change case without a card).
func (c *StudentImportConfig) FindExisting(ctx context.Context, row importModels.StudentImportRow) (*int64, error) {
	if row.TagID != "" {
		id, err := c.findStudentByTag(ctx, row.TagID)
		if err != nil || id != nil {
			return id, err
		}
	}
	return c.findStudentWithoutTag(ctx, row)
}

func (c *StudentImportConfig) findStudentWithoutTag(ctx context.Context, row importModels.StudentImportRow) (*int64, error) {
	students, err := findStudentsByNameAndClass(ctx, c.Persons, c.Students, row.FirstName, row.LastName, row.SchoolClass)
	if err != nil {
		return nil, err
	}
	if len(students) == 1 {
		return &students[0].ID, nil
	}
	if len(students) > 1 {
		return nil, fmt.Errorf("mehrere Kinder gefunden mit Name '%s %s' in Klasse '%s'",
			row.FirstName, row.LastName, row.SchoolClass)
	}
	if importModeFromContext(ctx) == importModels.ImportModeCreate {
		return nil, nil
	}
	return c.findStudentByNameAndBirthday(ctx, row)
}

// findStudentByTag returns the student wearing the card, if any.
func (c *StudentImportConfig) findStudentByTag(ctx context.Context, tagID string) (*int64, error) {
	wearer, err := c.Persons.FindPersonByTag(ctx, tagID)
	if err != nil {
		if errors.Is(err, ports.ErrPersonNotFound) {
			return nil, nil
		}
		return nil, err
	}
	students, err := c.Students.ListStudentsByPersonID(ctx, []int64{wearer.ID})
	if err != nil {
		return nil, err
	}
	if len(students) == 0 {
		return nil, nil
	}
	return &students[0].ID, nil
}

// findStudentByNameAndBirthday matches a child that changed class: same
// name, same birthday, exactly one hit.
func (c *StudentImportConfig) findStudentByNameAndBirthday(ctx context.Context, row importModels.StudentImportRow) (*int64, error) {
	birthday, err := parseOptionalDate(row.Birthday)
	if err != nil || birthday == nil {
		return nil, nil
	}
	persons, err := findPersonsByExactName(ctx, c.Persons, row.FirstName, row.LastName)
	if err != nil {
		return nil, err
	}
	var personIDs []int64
	for _, person := range persons {
		if person.Birthday == birthday.String() {
			personIDs = append(personIDs, person.ID)
		}
	}
	if len(personIDs) == 0 {
		return nil, nil
	}
	students, err := c.Students.ListStudentsByPersonID(ctx, personIDs)
	if err != nil {
		return nil, err
	}
	switch len(students) {
	case 0:
		return nil, nil
	case 1:
		return &students[0].ID, nil
	default:
		return nil, fmt.Errorf("mehrere Kinder gefunden mit Name '%s %s' und Geburtstag %s", row.FirstName, row.LastName, row.Birthday)
	}
}

// findPersonsByExactName returns the tenant's persons whose first and last
// name equal the given ones, ignoring case and surrounding whitespace.
func findPersonsByExactName(ctx context.Context, persons ports.PersonDirectory, firstName, lastName string) ([]ports.Person, error) {
	first := strings.TrimSpace(firstName)
	last := strings.TrimSpace(lastName)
	if first == "" || last == "" {
		return nil, nil
	}
	// Unpaged on purpose: absence or ambiguity is decided over every namesake,
	// not over the first page of a sorted listing.
	return persons.SearchPersons(ctx, ports.PersonFilter{FirstNameEquals: first, LastNameEquals: last})
}

// findStudentsByNameAndClass returns the non-alumni students of the class
// that carry the given name, matching like the legacy repository lookup:
// case-insensitive and trimmed on all three keys.
func findStudentsByNameAndClass(ctx context.Context, persons ports.PersonDirectory, students ports.StudentDirectory, firstName, lastName, schoolClass string) ([]ports.Student, error) {
	matches, err := findPersonsByExactName(ctx, persons, firstName, lastName)
	if err != nil {
		return nil, err
	}
	if len(matches) == 0 {
		return nil, nil
	}
	personIDs := make([]int64, 0, len(matches))
	for _, person := range matches {
		personIDs = append(personIDs, person.ID)
	}
	rows, err := students.ListStudentsByPersonID(ctx, personIDs)
	if err != nil {
		return nil, err
	}
	class := strings.TrimSpace(schoolClass)
	result := make([]ports.Student, 0, len(rows))
	for _, student := range rows {
		if student.IsAlumnus() || !strings.EqualFold(strings.TrimSpace(student.SchoolClass), class) {
			continue
		}
		result = append(result, student)
	}
	return result, nil
}

// Create files a new child with every related record through the owners.
// A PostgreSQL savepoint keeps the row atomic inside the surrounding import
// transaction: a failure after the person was created rolls that person back
// while the other rows of the batch survive.
func (c *StudentImportConfig) Create(ctx context.Context, row importModels.StudentImportRow) (int64, error) {
	if _, hasTx := tenant.TransactionFromContext(ctx); hasTx {
		var studentID int64
		err := tenant.WithSavepoint(ctx, func(savepointCtx context.Context) error {
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
		TagID:     strutil.TrimToNil(row.TagID),
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
		input.Status = string(users.StudentStatusPending)
	}
	created, err := c.Students.CreateEnrollmentStudent(ctx, input)
	if err != nil {
		return 0, fmt.Errorf("create student: %w", err)
	}

	departure, err := importDeparturePatch(users.AllowedDepartureModesFromDeparture(departurePlanFromImportRow(row)), boundedNotePtr(row.DepartureCompanionNote))
	if err != nil {
		return 0, fmt.Errorf("create student: %w", err)
	}
	patch := departure
	patch.ExtraInfoSet, patch.ExtraInfo = true, strutil.TrimToNil(row.ExtraInfo)
	patch.SupervisorNotesSet, patch.SupervisorNotes = true, strutil.TrimToNil(row.SupervisorNotes)
	patch.HealthInfoSet, patch.HealthInfo = true, strutil.TrimToNil(row.HealthInfo)
	patch.AddressSet = true
	patch.AddressStreet = strutil.TrimToNil(row.AddressStreet)
	patch.AddressCity = strutil.TrimToNil(row.AddressCity)
	patch.AddressPostalCode = strutil.TrimToNil(row.AddressPostalCode)
	patch.GroupIDSet, patch.GroupID = true, row.GroupID
	patch.AGBAcceptedAtSet, patch.AGBAcceptedAt = true, parseOptionalImportDate(row.AGBAcceptedAt)
	patch.DataProcessingAcceptedAtSet, patch.DataProcessingAcceptedAt = true, parseOptionalImportDate(row.DataProcessingAcceptedAt)
	patch.EmailContactAcceptedAtSet, patch.EmailContactAcceptedAt = true, parseOptionalImportDate(row.EmailContactAcceptedAt)
	// Photo consent date is set; "given_by" is intentionally left nil on import.
	patch.PhotoConsentGivenAtSet, patch.PhotoConsentGivenAt = true, parseOptionalImportDate(row.PhotoConsentGivenAt)
	if err := c.Students.ApplyEnrollmentProfile(ctx, created.ID, patch); err != nil {
		return 0, fmt.Errorf("create student profile: %w", err)
	}

	if c.ConsentHistory != nil {
		after := consentSnapshot(created.ID, patch.AGBAcceptedAt, patch.DataProcessingAcceptedAt, patch.EmailContactAcceptedAt, patch.PhotoConsentGivenAt)
		if err := ports.ObserveCommand(ctx, "audit-platform", "record_consent_transitions", func() error {
			return c.ConsentHistory.RecordTransitions(ctx, nil, after, auditModels.StudentConsentSourceImport, nil, time.Now())
		}); err != nil {
			return 0, fmt.Errorf("create student consent history: %w", err)
		}
	}
	return created.ID, nil
}

// consentSnapshot is the minimal student view the consent-history recorder
// compares: the four consent timestamps of one persisted student.
func consentSnapshot(studentID int64, agb, dataProcessing, emailContact, photo *time.Time) *users.Student {
	return &users.Student{
		Model:                    base.Model{ID: studentID},
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
func importDeparturePatch(modes users.AllowedDepartureModes, note *string) (ports.EnrollmentProfilePatch, error) {
	allowed := modes.Normalize()
	// The retained model owns the departure rules (bounded note, a note for
	// every accompanied day). Validate them on a probe carrying only the
	// plan; PersonID and SchoolClass are the two unrelated required fields
	// its Validate checks first, and nothing here is persisted.
	probe := users.Student{PersonID: 1, SchoolClass: "probe", DepartureDays: allowed.DepartureDays(), AllowedDepartureModes: allowed, DepartureCompanionNote: note}
	if err := probe.Validate(); err != nil {
		return ports.EnrollmentProfilePatch{}, err
	}
	if !allowed.HasMode(users.DepartureAccompanied) {
		probe.DepartureCompanionNote = nil
	}
	patch := ports.EnrollmentProfilePatch{
		DepartureSet:           true,
		AllowedDepartureModes:  map[string][]string{},
		DepartureDays:          map[string]string{},
		BusDays:                map[string]bool(allowed.BusDays()),
		PickupDays:             map[string]bool(allowed.PickupDays()),
		PickupStatus:           allowed.LegacyPickupStatus(),
		DepartureCompanionNote: probe.DepartureCompanionNote,
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

func departureDaysFromRecord(record ports.EnrollmentRecord) users.DepartureDays {
	days := users.DepartureDays{}
	for day, mode := range record.DepartureDays {
		days[day] = users.DepartureMode(mode)
	}
	return days
}

// allowedModesFromRecord reads the stored mode sets of a child. A record
// written before the unified plan existed carries none; its exclusive
// per-day plan is then the source.
func allowedModesFromRecord(record ports.EnrollmentRecord) users.AllowedDepartureModes {
	modes := users.AllowedDepartureModes{}
	for day, values := range record.AllowedDepartureModes {
		for _, value := range values {
			modes[day] = append(modes[day], users.DepartureMode(value))
		}
	}
	if !modes.HasAny() {
		return users.AllowedDepartureModesFromDeparture(departureDaysFromRecord(record))
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

func (c *StudentImportConfig) createGuardianRelationships(ctx context.Context, studentID int64, guardians []importModels.GuardianImportData) error {
	for i, guardianData := range guardians {
		if err := c.createSingleGuardianRelationship(ctx, studentID, guardianData, i+1); err != nil {
			return err
		}
	}
	return nil
}

func (c *StudentImportConfig) createSingleGuardianRelationship(ctx context.Context, studentID int64, guardianData importModels.GuardianImportData, index int) error {
	guardianID, err := c.createOrFindGuardian(ctx, guardianData)
	if err != nil {
		return fmt.Errorf("guardian %d: %w", index, err)
	}
	return c.createGuardianRelationship(ctx, studentID, guardianID, guardianData, index)
}

// createGuardianRelationship links the guardian to the child. The role preset
// of the row wins; an unknown or empty preset leaves the owner to derive the
// default from the relationship flags.
func (c *StudentImportConfig) createGuardianRelationship(ctx context.Context, studentID, guardianID int64, guardianData importModels.GuardianImportData, index int) error {
	link := ports.LinkGuardian{
		StudentID:          studentID,
		GuardianProfileID:  guardianID,
		RelationshipType:   MapRelationshipType(guardianData.RelationshipType),
		IsPrimary:          guardianData.IsPrimary,
		IsEmergencyContact: guardianData.IsEmergencyContact,
		CanPickup:          guardianData.CanPickup,
		PickupNotes:        strutil.TrimToNil(guardianData.PickupNotes),
		EmergencyPriority:  guardianData.EmergencyPriority,
	}
	if link.EmergencyPriority < 1 {
		link.EmergencyPriority = 1
	}
	if role, ok := MapGuardianRole(guardianData.GuardianRole); ok && role != "" {
		link.GuardianRole = role
	}
	if _, err := c.Guardians.LinkGuardianToStudent(ctx, link); err != nil {
		return fmt.Errorf("create relationship %d: %w", index, err)
	}
	return nil
}

// createOrFindGuardian deduplicates guardians by e-mail: an existing profile
// with the row's e-mail is reused (and patched with the row's non-empty
// fields), otherwise a new profile is created.
func (c *StudentImportConfig) createOrFindGuardian(ctx context.Context, data importModels.GuardianImportData) (int64, error) {
	if email := strings.TrimSpace(data.Email); email != "" {
		existing, err := c.findGuardianByEmail(ctx, email)
		if err != nil {
			return 0, fmt.Errorf("database error checking existing guardian: %w", err)
		}
		if existing != nil {
			if err := c.updateExistingGuardianProfile(ctx, *existing, data); err != nil {
				return 0, fmt.Errorf("existing guardian profile aktualisieren: %w", err) //nolint:staticcheck // ST1005: user-facing German message
			}
			if err := c.createGuardianPhoneNumbers(ctx, existing.ID, data.PhoneNumbers); err != nil {
				return existing.ID, fmt.Errorf("add phone numbers to existing guardian: %w", err)
			}
			return existing.ID, nil
		}
	}

	guardian, err := c.Guardians.CreateGuardian(ctx, ports.GuardianInput{
		FirstName:          strings.TrimSpace(data.FirstName),
		LastName:           strings.TrimSpace(data.LastName),
		Email:              strutil.TrimToNil(data.Email),
		AddressStreet:      strutil.TrimToNil(data.AddressStreet),
		AddressCity:        strutil.TrimToNil(data.AddressCity),
		AddressPostalCode:  strutil.TrimToNil(data.AddressPostalCode),
		Notes:              strutil.TrimToNil(data.Notes),
		LanguagePreference: guardianLanguagePreference(data.LanguagePreference),
	})
	if err != nil {
		return 0, err
	}
	if err := c.createGuardianPhoneNumbers(ctx, guardian.ID, data.PhoneNumbers); err != nil {
		return 0, fmt.Errorf("create phone numbers: %w", err)
	}
	return guardian.ID, nil
}

// findGuardianByEmail resolves the tenant's guardian with exactly this e-mail
// (case-insensitive, like the legacy lookup); nil when none exists.
func (c *StudentImportConfig) findGuardianByEmail(ctx context.Context, email string) (*ports.Guardian, error) {
	guardian, err := c.Guardians.FindGuardianByEmail(ctx, email)
	if errors.Is(err, ports.ErrGuardianNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &guardian, nil
}

// updateExistingGuardianProfile merges non-empty import fields into an
// existing guardian. Only fields the row carries change; the owner update is
// a full replace, so the current values are carried along.
func (c *StudentImportConfig) updateExistingGuardianProfile(ctx context.Context, existing ports.Guardian, data importModels.GuardianImportData) error {
	input := ports.GuardianInput{
		FirstName:              existing.FirstName,
		LastName:               existing.LastName,
		Email:                  existing.Email,
		AddressStreet:          existing.AddressStreet,
		AddressCity:            existing.AddressCity,
		AddressPostalCode:      existing.AddressPostalCode,
		PreferredContactMethod: existing.PreferredContactMethod,
		LanguagePreference:     existing.LanguagePreference,
		Notes:                  existing.Notes,
	}
	updated := false
	if v := strings.TrimSpace(data.AddressStreet); v != "" && !ptrEquals(existing.AddressStreet, v) {
		input.AddressStreet = strutil.TrimToNil(v)
		updated = true
	}
	if v := strings.TrimSpace(data.AddressCity); v != "" && !ptrEquals(existing.AddressCity, v) {
		input.AddressCity = strutil.TrimToNil(v)
		updated = true
	}
	if v := strings.TrimSpace(data.AddressPostalCode); v != "" && !ptrEquals(existing.AddressPostalCode, v) {
		input.AddressPostalCode = strutil.TrimToNil(v)
		updated = true
	}
	if v := strings.TrimSpace(data.Notes); v != "" && !ptrEquals(existing.Notes, v) {
		input.Notes = strutil.TrimToNil(v)
		updated = true
	}
	if v := guardianLanguagePreference(data.LanguagePreference); data.LanguagePreference != "" && v != existing.LanguagePreference {
		input.LanguagePreference = v
		updated = true
	}
	if !updated {
		return nil
	}
	return c.Guardians.UpdateGuardian(ctx, existing.ID, input)
}

// createGuardianPhoneNumbers adds the phone numbers of the row the guardian
// does not have yet. Priority and the primary flag are the owner's decision.
func (c *StudentImportConfig) createGuardianPhoneNumbers(ctx context.Context, guardianID int64, phones []importModels.PhoneImportData) error {
	if len(phones) == 0 {
		return nil
	}
	existing, err := c.Guardians.ListGuardianPhones(ctx, guardianID)
	if err != nil {
		return fmt.Errorf("Telefonnummern laden: %w", err) //nolint:staticcheck // ST1005: user-facing German message
	}
	existingNumbers := make(map[string]struct{}, len(existing))
	for _, phone := range existing {
		existingNumbers[phone.PhoneNumber] = struct{}{}
	}
	for i, phoneData := range phones {
		if phoneData.PhoneNumber == "" {
			continue
		}
		if _, exists := existingNumbers[phoneData.PhoneNumber]; exists {
			continue
		}
		var label *string
		if phoneData.Label != "" {
			label = &phoneData.Label
		}
		if _, err := c.Guardians.AddGuardianPhone(ctx, guardianID, ports.GuardianPhoneInput{
			PhoneNumber: phoneData.PhoneNumber,
			PhoneType:   string(mapPhoneType(phoneData.PhoneType)),
			Label:       label,
			IsPrimary:   phoneData.IsPrimary,
		}); err != nil {
			return fmt.Errorf("phone %d: %w", i+1, err)
		}
		existingNumbers[phoneData.PhoneNumber] = struct{}{}
	}
	return nil
}

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
	return tenant.WithSavepoint(ctx, func(ctx context.Context) error {
		return c.updateAllEntities(ctx, studentID, row)
	})
}

func (c *StudentImportConfig) updateAllEntities(ctx context.Context, studentID int64, row importModels.StudentImportRow) error {
	// The owner takes the shared class gate and the row lock before the
	// import reads the profile it is about to patch.
	record, err := c.Students.ReadEnrollmentStudent(ctx, studentID, "update")
	if err != nil {
		if errors.Is(err, ports.ErrStudentNotFound) {
			return errors.New("Kind nicht gefunden") //nolint:staticcheck // ST1005: user-facing German message
		}
		return fmt.Errorf("Kind laden: %w", err) //nolint:staticcheck // ST1005: user-facing German message
	}
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
		input.TagID = strutil.TrimToNil(row.TagID)
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

func (c *StudentImportConfig) updateStudentFromRow(ctx context.Context, record ports.EnrollmentRecord, row importModels.StudentImportRow) error {
	// Class, lifecycle window and status go through the renewal command.
	renewal := ports.EnrollmentStudent{
		PersonID: record.PersonID, SchoolClass: record.SchoolClass, Status: record.Status,
		EnrolledFrom: record.EnrolledFrom, EnrolledUntil: record.EnrolledUntil,
		GuardianEmail: record.GuardianEmail, GuardianPhone: record.GuardianPhone,
	}
	if class := strings.TrimSpace(row.SchoolClass); class != "" {
		renewal.SchoolClass = class
	}
	if d := parseOptionalImportCalendarDate(row.EnrolledFrom); d != nil {
		renewal.EnrolledFrom = d.String()
		if enrollmentStartsInFuture(d) {
			renewal.Status = string(users.StudentStatusPending)
		} else if renewal.Status == string(users.StudentStatusPending) {
			renewal.Status = string(users.StudentStatusActive)
		}
	}
	if d := parseOptionalImportCalendarDate(row.EnrolledUntil); d != nil {
		renewal.EnrolledUntil = d.String()
	}
	if renewal.SchoolClass != record.SchoolClass || renewal.Status != record.Status ||
		renewal.EnrolledFrom != record.EnrolledFrom || renewal.EnrolledUntil != record.EnrolledUntil {
		if err := c.Students.RenewEnrollmentStudent(ctx, record.ID, renewal); err != nil {
			return fmt.Errorf("Kind aktualisieren: %w", err) //nolint:staticcheck // ST1005: user-facing German message
		}
	}

	// Everything else is a profile patch; only the cells the row carries
	// are written.
	patch := ports.EnrollmentProfilePatch{}
	changed := false
	setStr := func(set *bool, dst **string, v string) {
		if strings.TrimSpace(v) != "" {
			*set, *dst = true, strutil.TrimToNil(v)
			changed = true
		}
	}
	setStr(&patch.ExtraInfoSet, &patch.ExtraInfo, row.ExtraInfo)
	setStr(&patch.SupervisorNotesSet, &patch.SupervisorNotes, row.SupervisorNotes)
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
			allowed = users.AllowedDepartureModesFromDeparture(days)
		}
		if note == nil {
			note = record.DepartureCompanionNote
		}
		departure, err := importDeparturePatch(allowed, note)
		if err != nil {
			return fmt.Errorf("Kind aktualisieren: %w", err) //nolint:staticcheck // ST1005: user-facing German message
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

	if changed {
		if err := c.Students.ApplyEnrollmentProfile(ctx, record.ID, patch); err != nil {
			return fmt.Errorf("Kind aktualisieren: %w", err) //nolint:staticcheck // ST1005: user-facing German message
		}
	}
	if c.ConsentHistory != nil {
		before := consentSnapshot(record.ID, record.AGBAcceptedAt, record.DataProcessingAcceptedAt, record.EmailContactAcceptedAt, record.PhotoConsentGivenAt)
		if err := ports.ObserveCommand(ctx, "audit-platform", "record_consent_transitions", func() error {
			return c.ConsentHistory.RecordTransitions(ctx, before, after, auditModels.StudentConsentSourceImport, nil, time.Now())
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
		return strutil.TrimToNil(cell)
	}
	return current
}

// mergeGuardianRelationships links guardians the child does not have yet and
// patches the relationship of the ones it has (role, pickup note, priority,
// relationship type when given). The Ja/Nein flags are changed only when their
// cells are supplied; an empty cell must not revoke an existing permission.
func (c *StudentImportConfig) mergeGuardianRelationships(ctx context.Context, studentID int64, guardians []importModels.GuardianImportData) error {
	if len(guardians) == 0 {
		return nil
	}
	existing, err := c.Guardians.ListStudentGuardians(ctx, studentID)
	if err != nil {
		return fmt.Errorf("Erziehungsberechtigte laden: %w", err) //nolint:staticcheck // ST1005: user-facing German message
	}
	byGuardianID := make(map[int64]ports.GuardianLink, len(existing))
	for _, entry := range existing {
		byGuardianID[entry.Link.GuardianProfileID] = entry.Link
	}
	linkedProfiles, err := c.loadLinkedGuardianProfiles(ctx, existing)
	if err != nil {
		return err
	}

	for i, data := range guardians {
		guardianID, err := c.resolveLinkedGuardian(ctx, linkedProfiles, data)
		if err != nil {
			return fmt.Errorf("guardian %d: %w", i+1, err)
		}
		if guardianID == 0 {
			guardianID, err = c.createOrFindGuardian(ctx, data)
			if err != nil {
				return fmt.Errorf("guardian %d: %w", i+1, err)
			}
		}
		link, linked := byGuardianID[guardianID]
		if !linked {
			if err := c.createGuardianRelationship(ctx, studentID, guardianID, data, i+1); err != nil {
				return err
			}
			continue
		}

		update := ports.GuardianLinkUpdate{}
		changed := false
		if strings.TrimSpace(data.RelationshipType) != "" {
			if mapped := MapRelationshipType(data.RelationshipType); mapped != link.RelationshipType {
				update.RelationshipType = &mapped
				changed = true
			}
		}
		if role, ok := MapGuardianRole(data.GuardianRole); ok && role != "" && role != link.GuardianRole {
			update.GuardianRole = &role
			changed = true
		}
		if notes := strings.TrimSpace(data.PickupNotes); notes != "" && !ptrEquals(link.PickupNotes, notes) {
			update.PickupNotes = &notes
			changed = true
		}
		if data.EmergencyPriority > 0 && data.EmergencyPriority != link.EmergencyPriority {
			priority := data.EmergencyPriority
			update.EmergencyPriority = &priority
			changed = true
		}
		if data.IsPrimarySet && data.IsPrimary != link.IsPrimary {
			primary := data.IsPrimary
			update.IsPrimary = &primary
			changed = true
		}
		if data.IsEmergencyContactSet && data.IsEmergencyContact != link.IsEmergencyContact {
			emergency := data.IsEmergencyContact
			update.IsEmergencyContact = &emergency
			changed = true
		}
		if data.CanPickupSet && data.CanPickup != link.CanPickup {
			pickup := data.CanPickup
			update.CanPickup = &pickup
			changed = true
		}
		if changed {
			if err := c.Guardians.UpdateGuardianLink(ctx, link.ID, update); err != nil {
				return fmt.Errorf("guardian %d: Zuordnung aktualisieren: %w", i+1, err)
			}
		}
	}
	return nil
}

// linkedGuardianProfile is one guardian already linked to the child, together
// with the phone numbers stored for it, so a re-import can recognise the
// guardian without an e-mail address.
type linkedGuardianProfile struct {
	profile ports.Guardian
	phones  []string
}

// loadLinkedGuardianProfiles loads the phone numbers of the guardians already
// linked to the child.
func (c *StudentImportConfig) loadLinkedGuardianProfiles(ctx context.Context, existing []ports.GuardianWithLink) ([]linkedGuardianProfile, error) {
	linked := make([]linkedGuardianProfile, 0, len(existing))
	for _, entry := range existing {
		phones, err := c.Guardians.ListGuardianPhones(ctx, entry.Guardian.ID)
		if err != nil {
			return nil, fmt.Errorf("Telefonnummern der Erziehungsberechtigten laden: %w", err) //nolint:staticcheck // ST1005: user-facing German message
		}
		var normalized []string
		for _, phone := range phones {
			if value := normalizeImportPhone(phone.PhoneNumber); value != "" {
				normalized = append(normalized, value)
			}
		}
		linked = append(linked, linkedGuardianProfile{profile: entry.Guardian, phones: normalized})
	}
	return linked, nil
}

// resolveLinkedGuardian recognises an imported guardian among the guardians the
// child already has: by e-mail, otherwise by a stored phone number, otherwise
// by a unique first and last name. The match is confined to the child's own
// relationships, so a shared landline of two different children never merges
// strangers. It returns 0 when nothing matches; the caller then falls back to
// the school-wide e-mail lookup or creates the guardian. A recognised guardian
// receives the non-empty profile fields and any new phone numbers of the row.
func (c *StudentImportConfig) resolveLinkedGuardian(ctx context.Context, linked []linkedGuardianProfile, data importModels.GuardianImportData) (int64, error) {
	match := matchLinkedGuardian(linked, data)
	if match == nil {
		return 0, nil
	}
	if err := c.updateExistingGuardianProfile(ctx, *match, data); err != nil {
		return 0, fmt.Errorf("existing guardian profile aktualisieren: %w", err) //nolint:staticcheck // ST1005: user-facing German message
	}
	if err := c.createGuardianPhoneNumbers(ctx, match.ID, data.PhoneNumbers); err != nil {
		return 0, fmt.Errorf("add phone numbers to existing guardian: %w", err)
	}
	return match.ID, nil
}

// matchLinkedGuardian is the pure matching step of resolveLinkedGuardian.
func matchLinkedGuardian(linked []linkedGuardianProfile, data importModels.GuardianImportData) *ports.Guardian {
	if len(linked) == 0 {
		return nil
	}
	if email := strings.ToLower(strings.TrimSpace(data.Email)); email != "" {
		for i := range linked {
			g := &linked[i]
			if g.profile.Email != nil && strings.ToLower(strings.TrimSpace(*g.profile.Email)) == email {
				return &g.profile
			}
		}
		// An e-mail that matches nobody linked is resolved school-wide by the
		// caller; a phone or name match would otherwise override the e-mail.
		return nil
	}

	phones := importGuardianPhones(data)
	for i := range linked {
		g := &linked[i]
		for _, stored := range g.phones {
			if _, ok := phones[stored]; ok {
				return &g.profile
			}
		}
	}

	first := strings.ToLower(strings.TrimSpace(data.FirstName))
	last := strings.ToLower(strings.TrimSpace(data.LastName))
	if first == "" || last == "" {
		return nil
	}
	var byName *ports.Guardian
	for i := range linked {
		g := &linked[i]
		if strings.ToLower(strings.TrimSpace(g.profile.FirstName)) != first ||
			strings.ToLower(strings.TrimSpace(g.profile.LastName)) != last {
			continue
		}
		if byName != nil {
			return nil // ambiguous: two linked guardians share the name
		}
		byName = &g.profile
	}
	return byName
}

// --- weekly schedules ---

func parseWallClock(value string) (time.Time, error) {
	parsed, err := time.Parse("15:04", value)
	if err != nil {
		return time.Time{}, err
	}
	// Use a valid reference date: time.Parse("15:04") produces year 0000,
	// which PostgreSQL rejects.
	return timezone.NormalizeWallClock(parsed), nil
}

// createArrivalSchedules creates weekly arrival schedule records for a student.
func (c *StudentImportConfig) createArrivalSchedules(ctx context.Context, studentID int64, schedules []importModels.ArrivalScheduleImportData) error {
	if len(schedules) == 0 || c.Schedules == nil {
		return nil
	}
	for i, sched := range schedules {
		arrival, err := parseWallClock(sched.ExpectedArrival)
		if err != nil {
			return fmt.Errorf("arrival schedule %d: invalid time '%s': %w", i+1, sched.ExpectedArrival, err)
		}
		if _, err := c.Schedules.CreateArrivalSchedule(ctx, ports.ArrivalSchedule{
			StudentID: studentID, Weekday: sched.Weekday, ExpectedArrival: arrival,
			Notes: strutil.TrimToNil(sched.Notes), CreatedBy: ImporterIDFromContext(ctx),
		}); err != nil {
			return fmt.Errorf("create arrival schedule (weekday %d): %w", sched.Weekday, err)
		}
	}
	return nil
}

// createPickupSchedules creates weekly pickup schedule records for a student.
func (c *StudentImportConfig) createPickupSchedules(ctx context.Context, studentID int64, schedules []importModels.PickupScheduleImportData) error {
	if len(schedules) == 0 || c.Schedules == nil {
		return nil
	}
	for i, sched := range schedules {
		pickup, err := parseWallClock(sched.PickupTime)
		if err != nil {
			return fmt.Errorf("pickup schedule %d: invalid time '%s': %w", i+1, sched.PickupTime, err)
		}
		if _, err := c.Schedules.CreatePickupSchedule(ctx, ports.PickupSchedule{
			StudentID: studentID, Weekday: sched.Weekday, PickupTime: pickup,
			Notes: strutil.TrimToNil(sched.Notes), CreatedBy: ImporterIDFromContext(ctx), Source: ports.ScheduleSourceStaff,
		}); err != nil {
			return fmt.Errorf("create pickup schedule (weekday %d): %w", sched.Weekday, err)
		}
	}
	return nil
}

func (c *StudentImportConfig) upsertArrivalSchedules(ctx context.Context, studentID int64, schedules []importModels.ArrivalScheduleImportData) error {
	if len(schedules) == 0 || c.Schedules == nil {
		return nil
	}
	for _, sched := range schedules {
		existing, err := c.Schedules.ListArrivalSchedules(ctx, ports.StudentScheduleFilter{StudentIDs: []int64{studentID}, Weekday: sched.Weekday})
		if err != nil {
			return fmt.Errorf("Ankunftszeit für Wochentag %d laden: %w", sched.Weekday, err) //nolint:staticcheck // ST1005: user-facing German message
		}
		if len(existing) == 0 {
			if err := c.createArrivalSchedules(ctx, studentID, []importModels.ArrivalScheduleImportData{sched}); err != nil {
				return err
			}
			continue
		}
		arrival, err := parseWallClock(sched.ExpectedArrival)
		if err != nil {
			return fmt.Errorf("Ungültige Ankunftszeit '%s': %w", sched.ExpectedArrival, err) //nolint:staticcheck // ST1005: user-facing German message
		}
		current := existing[0]
		current.ExpectedArrival = arrival
		if strings.TrimSpace(sched.Notes) != "" {
			current.Notes = strutil.TrimToNil(sched.Notes)
		}
		if err := c.Schedules.UpdateArrivalSchedule(ctx, current); err != nil {
			return fmt.Errorf("Ankunftszeit für Wochentag %d aktualisieren: %w", sched.Weekday, err) //nolint:staticcheck // ST1005: user-facing German message
		}
	}
	return nil
}

func (c *StudentImportConfig) upsertPickupSchedules(ctx context.Context, studentID int64, schedules []importModels.PickupScheduleImportData) error {
	if len(schedules) == 0 || c.Schedules == nil {
		return nil
	}
	for _, sched := range schedules {
		existing, err := c.Schedules.ListPickupSchedules(ctx, ports.StudentScheduleFilter{StudentIDs: []int64{studentID}, Weekday: sched.Weekday})
		if err != nil {
			return fmt.Errorf("Abholzeit für Wochentag %d laden: %w", sched.Weekday, err) //nolint:staticcheck // ST1005: user-facing German message
		}
		if len(existing) == 0 {
			if err := c.createPickupSchedules(ctx, studentID, []importModels.PickupScheduleImportData{sched}); err != nil {
				return err
			}
			continue
		}
		pickup, err := parseWallClock(sched.PickupTime)
		if err != nil {
			return fmt.Errorf("Ungültige Abholzeit '%s': %w", sched.PickupTime, err) //nolint:staticcheck // ST1005: user-facing German message
		}
		current := existing[0]
		current.PickupTime = pickup
		if strings.TrimSpace(sched.Notes) != "" {
			current.Notes = strutil.TrimToNil(sched.Notes)
		}
		if err := c.Schedules.UpdatePickupSchedule(ctx, current); err != nil {
			return fmt.Errorf("Abholzeit für Wochentag %d aktualisieren: %w", sched.Weekday, err) //nolint:staticcheck // ST1005: user-facing German message
		}
	}
	return nil
}
