package importpkg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/moto-nrw/project-phoenix/internal/strutil"
	authModels "github.com/moto-nrw/project-phoenix/models/auth"
	importModels "github.com/moto-nrw/project-phoenix/models/import"
	authsvc "github.com/moto-nrw/project-phoenix/services/auth"
	"github.com/moto-nrw/project-phoenix/services/import/ports"
	"github.com/moto-nrw/project-phoenix/tenant"
)

// This file holds the owner-backed read and write paths of the staff import
// (#2708): People Directory files the person, School Membership the staff
// and caregiver rows, Workforce the master data and qualifications, and
// Identity & Access issues the invitation.

// PreloadReferenceData loads the tenant's role names (for fuzzy suggestions on
// unresolved roles), the school display name (for the invitation email) and the
// existing staff (for duplicate detection and update matching).
func (c *StaffImportConfig) PreloadReferenceData(ctx context.Context) error {
	roles, err := c.RoleRepo.List(ctx, map[string]interface{}{})
	if err != nil {
		return fmt.Errorf("preload roles: %w", err)
	}
	c.roleDisplayNames = make([]string, 0, len(roles))
	c.rolesByID = make(map[int64]*authModels.Role, len(roles))
	for _, role := range roles {
		c.roleDisplayNames = append(c.roleDisplayNames, roleDisplayName(role.Name))
		c.rolesByID[role.ID] = role
	}

	// School name is best-effort: a missing name only degrades the email text,
	// it must not abort the import.
	if c.SchoolRepo != nil {
		if school, err := c.SchoolRepo.FindByID(ctx, tenant.FromContext(ctx)); err == nil && school != nil {
			c.schoolName = school.Name
		}
	}

	c.staffByPersonnelNumber = make(map[string]*indexedStaff)
	c.staffByName = make(map[string][]*indexedStaff)
	if c.Membership == nil || c.Persons == nil {
		return nil
	}
	existing, err := c.Membership.ListStaff(ctx, ports.StaffFilter{})
	if err != nil {
		return fmt.Errorf("preload staff: %w", err)
	}
	personIDs := make([]int64, 0, len(existing))
	for _, staff := range existing {
		personIDs = append(personIDs, staff.PersonID)
	}
	persons, err := c.Persons.ListPersonsByID(ctx, personIDs)
	if err != nil {
		return fmt.Errorf("preload staff persons: %w", err)
	}
	personsByID := make(map[int64]ports.Person, len(persons))
	for _, person := range persons {
		personsByID[person.ID] = person
	}
	for _, staff := range existing {
		entry := &indexedStaff{ID: staff.ID, PersonID: staff.PersonID, PersonnelNumber: staff.PersonnelNumber}
		if person, ok := personsByID[staff.PersonID]; ok {
			entry.FirstName, entry.LastName = person.FirstName, person.LastName
		}
		c.indexStaff(entry)
	}
	return nil
}

// indexStaff keeps the in-memory match keys current so a later row of the
// same file resolves to this record instead of creating a twin.
func (c *StaffImportConfig) indexStaff(entry *indexedStaff) {
	if c.staffByPersonnelNumber == nil {
		c.staffByPersonnelNumber = make(map[string]*indexedStaff)
	}
	if c.staffByName == nil {
		c.staffByName = make(map[string][]*indexedStaff)
	}
	if entry.PersonnelNumber != nil {
		if pn := strings.ToLower(strings.TrimSpace(*entry.PersonnelNumber)); pn != "" {
			c.staffByPersonnelNumber[pn] = entry
		}
	}
	if entry.FirstName != "" || entry.LastName != "" {
		key := staffNameKey(entry.FirstName, entry.LastName)
		c.staffByName[key] = append(c.staffByName[key], entry)
	}
}

func (c *StaffImportConfig) unindexStaff(entry *indexedStaff) {
	if entry.PersonnelNumber != nil {
		delete(c.staffByPersonnelNumber, strings.ToLower(strings.TrimSpace(*entry.PersonnelNumber)))
	}
	key := staffNameKey(entry.FirstName, entry.LastName)
	candidates := c.staffByName[key]
	for i, candidate := range candidates {
		if candidate.ID == entry.ID {
			c.staffByName[key] = append(candidates[:i], candidates[i+1:]...)
			break
		}
	}
	if len(c.staffByName[key]) == 0 {
		delete(c.staffByName, key)
	}
}

// findStaffByLoginEmail resolves email → account → person → staff within the
// current tenant. Returns (nil, nil) at every "not here" step.
func (c *StaffImportConfig) findStaffByLoginEmail(ctx context.Context, rawEmail string) (*int64, error) {
	email, err := normalizeStaffEmail(rawEmail)
	if err != nil || email == "" || c.AccountRepo == nil {
		return nil, nil
	}

	account, err := c.AccountRepo.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return c.findStaffByPendingInvitation(ctx, email)
		}
		return nil, err
	}
	if account == nil {
		return c.findStaffByPendingInvitation(ctx, email)
	}

	exists, err := c.AccountTenantRepo.ExistsByAccountAndTenant(ctx, account.ID, tenant.FromContext(ctx))
	if err != nil {
		return nil, err
	}
	if !exists {
		return c.findStaffByPendingInvitation(ctx, email)
	}

	if c.Persons == nil || c.Membership == nil {
		// Legacy wiring without the Stammdaten owners: the account itself is
		// the duplicate marker, as before #2600.
		id := account.ID
		return &id, nil
	}
	person, err := c.Persons.FindPersonByAccount(ctx, account.ID)
	if err != nil {
		if errors.Is(err, ports.ErrPersonNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return c.liveStaffIDByPerson(ctx, person.ID)
}

// liveStaffIDByPerson returns the id of the person's live staff row, nil when
// the person has none.
func (c *StaffImportConfig) liveStaffIDByPerson(ctx context.Context, personID int64) (*int64, error) {
	staff, err := c.Membership.FindStaffByPerson(ctx, personID)
	if err != nil {
		if errors.Is(err, ports.ErrStaffNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if staff.IsDeleted() {
		return nil, nil
	}
	id := staff.ID
	return &id, nil
}

// findStaffByPendingInvitation resolves staff created by an import but not yet
// linked to an account. The invitation stores the imported person ID until it
// is accepted, so e-mail remains a stable update key in that interval.
func (c *StaffImportConfig) findStaffByPendingInvitation(ctx context.Context, email string) (*int64, error) {
	if c.InvitationRepo == nil || c.Membership == nil {
		return nil, nil
	}
	invitations, err := c.InvitationRepo.FindByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	var match *int64
	for _, invitation := range invitations {
		if invitation.UsedAt != nil || invitation.PersonID == nil {
			continue
		}
		id, err := c.liveStaffIDByPerson(ctx, *invitation.PersonID)
		if err != nil {
			return nil, err
		}
		if id == nil {
			continue
		}
		if match != nil && *match != *id {
			return nil, fmt.Errorf("mehrere offene Einladungen für E-Mail-Adresse '%s' verweisen auf unterschiedliche Personen", email)
		}
		match = id
	}
	return match, nil
}

// Create files the Stammdatensatz (Person, Staff, caregiver profile when the
// role needs one, master data, qualifications) and, when the row carries an
// e-mail, an invitation that points at the new person. Runs in a savepoint so
// a failing row leaves nothing behind while the surrounding import
// transaction keeps the other rows.
func (c *StaffImportConfig) Create(ctx context.Context, row importModels.StaffImportRow) (int64, error) {
	if c.Persons == nil || c.Membership == nil {
		return 0, errors.New("staff import: Stammdaten owners are not wired")
	}

	var staffID int64
	err := tenant.WithSavepoint(ctx, func(ctx context.Context) error {
		id, err := c.createRecords(ctx, row)
		if err != nil {
			return err
		}
		staffID = id
		return nil
	})
	if err != nil {
		return 0, err
	}
	return staffID, nil
}

func (c *StaffImportConfig) createRecords(ctx context.Context, row importModels.StaffImportRow) (int64, error) {
	person, err := c.Persons.CreatePerson(ctx, ports.CreatePerson{
		FirstName: row.FirstName,
		LastName:  row.LastName,
		Birthday:  calendarDateString(optionalImportDate(row.Birthday)),
	})
	if err != nil {
		return 0, fmt.Errorf("Person anlegen: %w", err) //nolint:staticcheck // ST1005: user-facing German message
	}

	staff, err := c.Membership.CreateStaff(ctx, ports.CreateStaff{StaffFields: ports.StaffFields{
		PersonID:        person.ID,
		StaffNotes:      row.StaffNotes,
		EmploymentType:  strutil.TrimToNil(row.EmploymentType),
		PersonnelNumber: strutil.TrimToNil(row.PersonnelNumber),
	}})
	if err != nil {
		return 0, fmt.Errorf("Mitarbeiter anlegen: %w", err) //nolint:staticcheck // ST1005: user-facing German message
	}

	if role := c.rolesByID[row.RoleID]; role != nil && authsvc.RoleNeedsCaregiverProfile(role) {
		if _, err := c.Membership.CreateTeacher(ctx, ports.CreateTeacher{TeacherFields: ports.TeacherFields{StaffID: staff.ID, Role: row.Position}}); err != nil {
			return 0, fmt.Errorf("Betreuungsprofil anlegen: %w", err) //nolint:staticcheck // ST1005: user-facing German message
		}
	}

	if err := c.writeMasterData(ctx, staff.ID, nil, row); err != nil {
		return 0, err
	}
	if err := c.writeQualifications(ctx, staff.ID, row); err != nil {
		return 0, err
	}

	if row.Email != "" && c.InvitationService != nil {
		if err := c.invite(ctx, person.ID, row); err != nil {
			return 0, err
		}
	}

	c.indexStaff(&indexedStaff{ID: staff.ID, PersonID: person.ID, PersonnelNumber: staff.PersonnelNumber, FirstName: person.FirstName, LastName: person.LastName})
	return staff.ID, nil
}

// invite issues the portal invitation for a freshly imported person.
func (c *StaffImportConfig) invite(ctx context.Context, personID int64, row importModels.StaffImportRow) error {
	email, err := normalizeStaffEmail(row.Email)
	if err != nil {
		return err
	}
	pid := personID
	req := authsvc.InvitationRequest{
		Email:            email,
		RoleID:           row.RoleID,
		TenantID:         tenant.FromContext(ctx),
		FirstName:        strutil.TrimToNil(row.FirstName),
		LastName:         strutil.TrimToNil(row.LastName),
		Position:         strutil.TrimToNil(row.Position),
		PersonID:         &pid,
		CreatedBy:        ImporterIDFromContext(ctx),
		SchoolName:       c.schoolName,
		ActorPermissions: ImporterPermissionsFromContext(ctx),
	}
	if err := ports.ObserveCommand(ctx, "identity-access", "create_invitation", func() error {
		_, err := c.InvitationService.CreateInvitation(ctx, req)
		return err
	}); err != nil {
		return fmt.Errorf("Einladung anlegen: %w", err) //nolint:staticcheck // ST1005: user-facing German message
	}
	return nil
}

// writeMasterData creates or patches the Workforce master data. Only columns
// with a value are written; an empty cell never clears a stored value, so a
// partial file can be re-imported without losing data.
func (c *StaffImportConfig) writeMasterData(ctx context.Context, staffID int64, existing *ports.StaffMasterData, row importModels.StaffImportRow) error {
	if c.Records == nil || !rowHasMasterData(row) {
		return nil
	}

	var data ports.StaffMasterData
	created := existing == nil
	if created {
		data = ports.StaffMasterData{StaffID: staffID}
	} else {
		data = *existing
	}

	setStr := func(dst **string, v string) {
		if v != "" {
			*dst = strutil.TrimToNil(v)
		}
	}
	setStr(&data.Gender, row.Gender)
	setStr(&data.AddressStreet, row.AddressStreet)
	setStr(&data.AddressPostalCode, row.AddressPostalCode)
	setStr(&data.AddressCity, row.AddressCity)
	setStr(&data.Phone, row.Phone)
	setStr(&data.Email, row.ContactEmail)
	setStr(&data.EmergencyContactName, row.EmergencyContactName)
	setStr(&data.EmergencyContactPhone, row.EmergencyContactPhone)
	if d := optionalImportDate(row.EntryDate); d != nil {
		data.EntryDate = d.String()
	}
	if d := optionalImportDate(row.ContractEndDate); d != nil {
		data.ContractEndDate = d.String()
	}
	if d := optionalImportDate(row.ProbationEndDate); d != nil {
		data.ProbationEndDate = d.String()
	}
	if row.WeeklyHours != "" {
		if hours, err := parseDecimalHours(row.WeeklyHours); err == nil {
			data.WeeklyHours = &hours
		}
	}

	var err error
	if created {
		_, err = c.Records.CreateStaffMasterData(ctx, data)
	} else {
		_, err = c.Records.UpdateStaffMasterData(ctx, data)
	}
	if err != nil {
		if errors.Is(err, ports.ErrInvalidStaffRecord) {
			return fmt.Errorf("Stammdaten ungültig: %w", err) //nolint:staticcheck // ST1005: user-facing German message
		}
		if created {
			return fmt.Errorf("Stammdaten anlegen: %w", err) //nolint:staticcheck // ST1005: user-facing German message
		}
		return fmt.Errorf("Stammdaten aktualisieren: %w", err) //nolint:staticcheck // ST1005: user-facing German message
	}
	return nil
}

func rowHasMasterData(row importModels.StaffImportRow) bool {
	return row.Gender != "" || row.AddressStreet != "" || row.AddressPostalCode != "" || row.AddressCity != "" ||
		row.Phone != "" || row.ContactEmail != "" || row.EmergencyContactName != "" || row.EmergencyContactPhone != "" ||
		row.EntryDate != "" || row.ContractEndDate != "" || row.ProbationEndDate != "" || row.WeeklyHours != ""
}

// writeQualifications replaces the qualification list when the row carries
// one. An empty cell keeps the stored list.
func (c *StaffImportConfig) writeQualifications(ctx context.Context, staffID int64, row importModels.StaffImportRow) error {
	if c.Records == nil || row.Qualifications == "" {
		return nil
	}
	entries, err := ParseStaffQualifications(row.Qualifications)
	if err != nil {
		return err
	}
	rows := make([]ports.StaffQualification, 0, len(entries))
	for _, entry := range entries {
		rows = append(rows, ports.StaffQualification{
			StaffID:    staffID,
			Name:       entry.Name,
			AcquiredOn: calendarDateString(entry.AcquiredOn),
			ExpiresOn:  calendarDateString(entry.ExpiresOn),
		})
	}
	if _, err := c.Records.ReplaceStaffQualifications(ctx, staffID, rows); err != nil {
		return fmt.Errorf("Qualifikationen speichern: %w", err) //nolint:staticcheck // ST1005: user-facing German message
	}
	return nil
}

// Update patches an existing staff member: names, birthday, notes, personnel
// number, employment type and the master data. Empty cells leave the stored
// value untouched. The role and the login are not touched — role changes and
// invitations for existing people belong to the Personalverwaltung.
func (c *StaffImportConfig) Update(ctx context.Context, staffID int64, row importModels.StaffImportRow) error {
	if c.Persons == nil || c.Membership == nil {
		return errors.New("staff import: Stammdaten owners are not wired")
	}
	return tenant.WithSavepoint(ctx, func(ctx context.Context) error {
		return c.updateRecords(ctx, staffID, row)
	})
}

func (c *StaffImportConfig) updateRecords(ctx context.Context, staffID int64, row importModels.StaffImportRow) error {
	staff, err := c.Membership.FindStaff(ctx, staffID)
	if err != nil {
		if errors.Is(err, ports.ErrStaffNotFound) {
			return errors.New("Mitarbeiter nicht gefunden") //nolint:staticcheck // ST1005: user-facing German message
		}
		return fmt.Errorf("Mitarbeiter laden: %w", err) //nolint:staticcheck // ST1005: user-facing German message
	}
	person, err := c.Persons.FindPerson(ctx, staff.PersonID)
	if err != nil {
		if errors.Is(err, ports.ErrPersonNotFound) {
			return errors.New("Person nicht gefunden") //nolint:staticcheck // ST1005: user-facing German message
		}
		return fmt.Errorf("Person laden: %w", err) //nolint:staticcheck // ST1005: user-facing German message
	}
	previous := indexedStaff{ID: staff.ID, PersonID: person.ID, PersonnelNumber: staff.PersonnelNumber, FirstName: person.FirstName, LastName: person.LastName}

	if row.PersonnelNumber != "" {
		pn := strings.ToLower(row.PersonnelNumber)
		if other, ok := c.staffByPersonnelNumber[pn]; ok && other.ID != staff.ID {
			return fmt.Errorf("Personalnummer '%s' ist bereits einer anderen Person zugeordnet", row.PersonnelNumber) //nolint:staticcheck // ST1005: user-facing German message
		}
	}

	personInput := ports.UpdatePerson{ID: person.ID, FirstName: person.FirstName, LastName: person.LastName, Birthday: person.Birthday, TagID: person.TagID, AccountID: person.AccountID}
	personChanged := false
	if row.FirstName != "" && row.FirstName != person.FirstName {
		personInput.FirstName = row.FirstName
		personChanged = true
	}
	if row.LastName != "" && row.LastName != person.LastName {
		personInput.LastName = row.LastName
		personChanged = true
	}
	if d := optionalImportDate(row.Birthday); d != nil {
		personInput.Birthday = d.String()
		personChanged = true
	}
	if personChanged {
		if person, err = c.Persons.UpdatePerson(ctx, personInput); err != nil {
			return fmt.Errorf("Person aktualisieren: %w", err) //nolint:staticcheck // ST1005: user-facing German message
		}
	}

	fields := ports.StaffFields{
		PersonID: staff.PersonID, StaffNotes: staff.StaffNotes, EmploymentType: staff.EmploymentType,
		WorkTimeModelID: staff.WorkTimeModelID, PersonnelNumber: staff.PersonnelNumber,
		RotationAnchorDate: staff.RotationAnchorDate, BirthdayDisplayOptOut: staff.BirthdayDisplayOptOut,
	}
	staffChanged := false
	if row.StaffNotes != "" && row.StaffNotes != staff.StaffNotes {
		fields.StaffNotes = row.StaffNotes
		staffChanged = true
	}
	if row.EmploymentType != "" && !ptrEquals(staff.EmploymentType, row.EmploymentType) {
		fields.EmploymentType = strutil.TrimToNil(row.EmploymentType)
		staffChanged = true
	}
	if row.PersonnelNumber != "" && !ptrEquals(staff.PersonnelNumber, row.PersonnelNumber) {
		fields.PersonnelNumber = strutil.TrimToNil(row.PersonnelNumber)
		staffChanged = true
	}
	if staffChanged {
		if staff, err = c.Membership.UpdateStaff(ctx, ports.UpdateStaff{ID: staff.ID, StaffFields: fields}); err != nil {
			return fmt.Errorf("Mitarbeiter aktualisieren: %w", err) //nolint:staticcheck // ST1005: user-facing German message
		}
	}

	if row.Position != "" {
		teacher, err := c.Membership.FindTeacherByStaff(ctx, staff.ID)
		if err != nil && !errors.Is(err, ports.ErrTeacherNotFound) {
			return fmt.Errorf("Betreuungsprofil laden: %w", err) //nolint:staticcheck // ST1005: user-facing German message
		}
		if err == nil && !teacher.IsDeleted() && teacher.Role != row.Position {
			if _, err := c.Membership.UpdateTeacher(ctx, ports.UpdateTeacher{ID: teacher.ID, TeacherFields: ports.TeacherFields{
				StaffID: teacher.StaffID, Specialization: teacher.Specialization, Role: row.Position, Qualifications: teacher.Qualifications,
			}}); err != nil {
				return fmt.Errorf("Betreuungsprofil aktualisieren: %w", err) //nolint:staticcheck // ST1005: user-facing German message
			}
		}
	}

	var existing *ports.StaffMasterData
	if c.Records != nil && rowHasMasterData(row) {
		data, err := c.Records.FindStaffMasterData(ctx, staff.ID)
		switch {
		case err == nil:
			existing = &data
		case errors.Is(err, ports.ErrStaffMasterDataNotFound):
		default:
			return fmt.Errorf("Stammdaten laden: %w", err) //nolint:staticcheck // ST1005: user-facing German message
		}
	}
	if err := c.writeMasterData(ctx, staff.ID, existing, row); err != nil {
		return err
	}
	if err := c.writeQualifications(ctx, staff.ID, row); err != nil {
		return err
	}

	c.unindexStaff(&previous)
	c.indexStaff(&indexedStaff{ID: staff.ID, PersonID: person.ID, PersonnelNumber: staff.PersonnelNumber, FirstName: person.FirstName, LastName: person.LastName})
	return nil
}
