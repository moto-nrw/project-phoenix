package services

import (
	"context"
	"database/sql"
	"errors"

	userModels "github.com/moto-nrw/project-phoenix/models/users"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	importService "github.com/moto-nrw/project-phoenix/services/import"
)

// The bindings below connect Enrollment's decision flow (#3564) to the People
// Directory rows the retained repositories still hold: persons, students,
// guardian profiles, student-guardian links and guardian phones. Every record
// travels whole, so an update writes back exactly what was read plus the
// decision's changes.

// enrollmentPeopleRules applies the People Directory's validation and role
// rules to the decision flow's records.
type enrollmentPeopleRules struct{}

func (enrollmentPeopleRules) ValidatePerson(person *enrollmentCompose.Person) error {
	row := decisionPersonRow(person)
	err := row.Validate()
	person.FirstName, person.LastName = row.FirstName, row.LastName
	return err
}

func (enrollmentPeopleRules) ValidateStudent(student *enrollmentCompose.Student) error {
	row := decisionStudentRow(student)
	err := row.Validate()
	student.SchoolClass = row.SchoolClass
	student.AddressStreet, student.AddressCity, student.AddressPostalCode = row.AddressStreet, row.AddressCity, row.AddressPostalCode
	student.DepartureCompanionNote = row.DepartureCompanionNote
	return err
}

func (enrollmentPeopleRules) ValidateGuardianProfile(profile *enrollmentCompose.GuardianProfile) error {
	row := decisionGuardianProfileRow(profile)
	err := row.Validate()
	profile.FirstName, profile.LastName, profile.Email = row.FirstName, row.LastName, row.Email
	return err
}

func (enrollmentPeopleRules) ValidateStudentGuardian(link *enrollmentCompose.StudentGuardian) error {
	row := decisionStudentGuardianRow(link)
	err := row.Validate()
	link.RelationshipType, link.GuardianRole, link.Permissions = row.RelationshipType, row.GuardianRole, row.Permissions
	return err
}

func (enrollmentPeopleRules) ApplyGuardianRole(link *enrollmentCompose.StudentGuardian, role enrollmentCompose.GuardianRole) {
	normalized, granted := securityruntime.StudentGuardianRolePreset(enrollmentGuardianRoleName(role))
	permissions := make(map[string]any, len(granted))
	for _, permission := range granted {
		permissions[permission] = true
	}
	link.GuardianRole, link.Permissions = normalized, permissions
}

func (enrollmentPeopleRules) IsFullGuardianRole(role string) bool {
	return securityruntime.IsFullGuardianRole(role)
}

func (enrollmentPeopleRules) MapRelationshipType(label string) string {
	return importService.MapRelationshipType(label)
}

// enrollmentGuardianRoleName is the stored role of a decision's role preset.
func enrollmentGuardianRoleName(role enrollmentCompose.GuardianRole) string {
	switch role {
	case enrollmentCompose.GuardianRolePrimary:
		return securityruntime.GuardianRolePrimaryGuardian
	case enrollmentCompose.GuardianRoleEmergency:
		return securityruntime.GuardianRoleEmergency
	case enrollmentCompose.GuardianRolePickupOnly:
		return securityruntime.GuardianRolePickupOnly
	default:
		return securityruntime.GuardianRoleCustom
	}
}

// enrollmentPersons creates and renames persons and resolves a reviewer's
// person.
type enrollmentPersons struct{ repo userModels.PersonRepository }

func (p enrollmentPersons) CreatePerson(ctx context.Context, person *enrollmentCompose.Person) error {
	row := decisionPersonRow(person)
	if err := p.repo.Create(ctx, row); err != nil {
		return err
	}
	*person = *decisionPersonRecord(row)
	return nil
}

func (p enrollmentPersons) PersonByID(ctx context.Context, id int64) (*enrollmentCompose.Person, error) {
	row, err := p.repo.FindByID(ctx, id)
	return decisionPersonRecord(row), err
}

func (p enrollmentPersons) UpdatePerson(ctx context.Context, person *enrollmentCompose.Person) error {
	return p.repo.Update(ctx, decisionPersonRow(person))
}

func (p enrollmentPersons) PersonByAccount(ctx context.Context, accountID int64) (*enrollmentCompose.Person, error) {
	row, err := p.repo.FindByAccountID(ctx, accountID)
	return decisionPersonRecord(row), err
}

func decisionPersonRow(person *enrollmentCompose.Person) *userModels.Person {
	row := &userModels.Person{
		FirstName: person.FirstName, LastName: person.LastName, Birthday: person.Birthday,
		TagID: person.TagID, AccountID: person.AccountID,
	}
	row.ID, row.CreatedAt, row.UpdatedAt = person.ID, person.CreatedAt, person.UpdatedAt
	row.TenantID = person.TenantID
	return row
}

func decisionPersonRecord(row *userModels.Person) *enrollmentCompose.Person {
	if row == nil {
		return nil
	}
	return &enrollmentCompose.Person{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		FirstName: row.FirstName, LastName: row.LastName, Birthday: row.Birthday,
		TagID: row.TagID, AccountID: row.AccountID,
	}
}

// enrollmentReviewerStaff resolves the staff member of a reviewer's person.
type enrollmentReviewerStaff struct{ repo userModels.StaffRepository }

func (s enrollmentReviewerStaff) StaffIDByPerson(ctx context.Context, personID int64) (int64, error) {
	staff, err := s.repo.FindByPersonID(ctx, personID)
	if err != nil || staff == nil {
		return 0, err
	}
	return staff.ID, nil
}

// enrollmentStudentRecords confirms that a student exists.
type enrollmentStudentRecords struct{ repo userModels.StudentRepository }

func (s enrollmentStudentRecords) FindStudent(ctx context.Context, id int64) error {
	_, err := s.repo.FindByID(ctx, id)
	return err
}

// decisionStudentRow is the People Directory row of a decision's student.
func decisionStudentRow(student *enrollmentCompose.Student) *userModels.Student {
	row := &userModels.Student{
		PersonID: student.PersonID, SchoolClass: student.SchoolClass, GroupID: student.GroupID,
		AddressStreet: student.AddressStreet, AddressCity: student.AddressCity, AddressPostalCode: student.AddressPostalCode,
		ExtraInfo: student.ExtraInfo, SupervisorNotes: student.SupervisorNotes, HealthInfo: student.HealthInfo,
		PickupStatus: student.PickupStatus, DepartureDays: student.DepartureDays,
		AllowedDepartureModes: student.AllowedDepartureModes, DepartureCompanionNote: student.DepartureCompanionNote,
		DepartureCompanionDays: student.DepartureCompanionDays, PickupDays: student.PickupDays, BusDays: student.BusDays,
		Sick: student.Sick, SickSince: student.SickSince, Excused: student.Excused, ExcusedSince: student.ExcusedSince,
		Status: userModels.StudentStatus(student.Status), EnrolledFrom: student.EnrolledFrom, EnrolledUntil: student.EnrolledUntil,
		PhotoPath: student.PhotoPath, PhotoConsentGivenAt: student.PhotoConsentGivenAt, PhotoConsentGivenBy: student.PhotoConsentGivenBy,
		AGBAcceptedAt: student.AGBAcceptedAt, DataProcessingAcceptedAt: student.DataProcessingAcceptedAt,
		EmailContactAcceptedAt: student.EmailContactAcceptedAt,
	}
	row.ID, row.CreatedAt, row.UpdatedAt = student.ID, student.CreatedAt, student.UpdatedAt
	row.TenantID = student.TenantID
	if baseline := student.DepartureBaseline; baseline != nil {
		row.DepartureBaseline = &userModels.DeparturePlanSnapshot{
			DepartureDays: baseline.DepartureDays, AllowedDepartureModes: baseline.AllowedDepartureModes,
			BusDays: baseline.BusDays, PickupDays: baseline.PickupDays,
		}
	}
	return row
}

// enrollmentStudentAudit records the decision flow's student changes on the
// People Directory's change trail.
type enrollmentStudentAudit struct{ auditor EnrollmentStudentAuditor }

func (a enrollmentStudentAudit) RecordChangesForActor(ctx context.Context, before, after *enrollmentCompose.Student, editedBy int64) error {
	return a.auditor.RecordChangesForActor(ctx, decisionStudentRow(before), decisionStudentRow(after), editedBy)
}

func (a enrollmentStudentAudit) RecordSystemStatusChange(ctx context.Context, studentID int64, before, after string) error {
	return a.auditor.RecordSystemStatusChange(ctx, studentID, userModels.StudentStatus(before), userModels.StudentStatus(after))
}

// enrollmentGuardianProfiles reads and writes the tenant's guardian profiles.
type enrollmentGuardianProfiles struct {
	repo userModels.GuardianProfileRepository
}

func (g enrollmentGuardianProfiles) GuardianProfileByAccount(ctx context.Context, accountID int64) (*enrollmentCompose.GuardianProfile, error) {
	row, err := g.repo.FindByAccountID(ctx, accountID)
	if errors.Is(err, userModels.ErrGuardianProfileNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return decisionGuardianProfileRecord(row), nil
}

func (g enrollmentGuardianProfiles) GuardianProfileByEmail(ctx context.Context, email string) (*enrollmentCompose.GuardianProfile, error) {
	row, err := g.repo.FindByEmail(ctx, email)
	if errors.Is(err, userModels.ErrGuardianProfileNotFound) || errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return decisionGuardianProfileRecord(row), nil
}

func (g enrollmentGuardianProfiles) GuardianProfilesByEmails(ctx context.Context, emails []string) ([]*enrollmentCompose.GuardianProfile, error) {
	rows, err := g.repo.FindByEmails(ctx, emails)
	if err != nil {
		return nil, err
	}
	profiles := make([]*enrollmentCompose.GuardianProfile, 0, len(rows))
	for _, row := range rows {
		profiles = append(profiles, decisionGuardianProfileRecord(row))
	}
	return profiles, nil
}

func (g enrollmentGuardianProfiles) GuardianProfilesByID(ctx context.Context, ids []int64) (map[int64]*enrollmentCompose.GuardianProfile, error) {
	rows, err := g.repo.FindByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	profiles := make(map[int64]*enrollmentCompose.GuardianProfile, len(rows))
	for id, row := range rows {
		profiles[id] = decisionGuardianProfileRecord(row)
	}
	return profiles, nil
}

func (g enrollmentGuardianProfiles) CreateGuardianProfile(ctx context.Context, profile *enrollmentCompose.GuardianProfile) error {
	row := decisionGuardianProfileRow(profile)
	if err := g.repo.Create(ctx, row); err != nil {
		return err
	}
	*profile = *decisionGuardianProfileRecord(row)
	return nil
}

func (g enrollmentGuardianProfiles) UpdateGuardianProfile(ctx context.Context, profile *enrollmentCompose.GuardianProfile) error {
	return g.repo.Update(ctx, decisionGuardianProfileRow(profile))
}

func (g enrollmentGuardianProfiles) LinkGuardianAccount(ctx context.Context, profileID, accountID int64) error {
	return g.repo.LinkAccount(ctx, profileID, accountID)
}

func decisionGuardianProfileRow(profile *enrollmentCompose.GuardianProfile) *userModels.GuardianProfile {
	row := &userModels.GuardianProfile{
		FirstName: profile.FirstName, LastName: profile.LastName, Email: profile.Email,
		AddressStreet: profile.AddressStreet, AddressCity: profile.AddressCity, AddressPostalCode: profile.AddressPostalCode,
		AccountID: profile.AccountID, HasAccount: profile.HasAccount,
		PreferredContactMethod: profile.PreferredContactMethod, LanguagePreference: profile.LanguagePreference,
		PortalLocale: profile.PortalLocale, Notes: profile.Notes,
	}
	row.ID, row.CreatedAt, row.UpdatedAt = profile.ID, profile.CreatedAt, profile.UpdatedAt
	row.TenantID = profile.TenantID
	return row
}

func decisionGuardianProfileRecord(row *userModels.GuardianProfile) *enrollmentCompose.GuardianProfile {
	if row == nil {
		return nil
	}
	return &enrollmentCompose.GuardianProfile{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		FirstName: row.FirstName, LastName: row.LastName, Email: row.Email,
		AddressStreet: row.AddressStreet, AddressCity: row.AddressCity, AddressPostalCode: row.AddressPostalCode,
		AccountID: row.AccountID, HasAccount: row.HasAccount,
		PreferredContactMethod: row.PreferredContactMethod, LanguagePreference: row.LanguagePreference,
		PortalLocale: row.PortalLocale, Notes: row.Notes,
	}
}

// enrollmentStudentGuardians reads and writes the student-guardian links
// through the retained composition seam over their three owners.
type enrollmentStudentGuardians struct {
	repo userModels.StudentGuardianRepository
}

func (s enrollmentStudentGuardians) GuardianLinksOfStudent(ctx context.Context, studentID int64) ([]*enrollmentCompose.StudentGuardian, error) {
	rows, err := s.repo.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, err
	}
	links := make([]*enrollmentCompose.StudentGuardian, 0, len(rows))
	for _, row := range rows {
		links = append(links, decisionStudentGuardianRecord(row))
	}
	return links, nil
}

func (s enrollmentStudentGuardians) CreateGuardianLink(ctx context.Context, link *enrollmentCompose.StudentGuardian) error {
	row := decisionStudentGuardianRow(link)
	if err := s.repo.Create(ctx, row); err != nil {
		return err
	}
	*link = *decisionStudentGuardianRecord(row)
	return nil
}

func (s enrollmentStudentGuardians) UpdateGuardianLink(ctx context.Context, link *enrollmentCompose.StudentGuardian) error {
	return s.repo.Update(ctx, decisionStudentGuardianRow(link))
}

func (s enrollmentStudentGuardians) DeleteGuardianLink(ctx context.Context, id int64) error {
	return s.repo.Delete(ctx, id)
}

func (s enrollmentStudentGuardians) GuardianLinkForUpdate(ctx context.Context, studentID, guardianProfileID int64) (*enrollmentCompose.StudentGuardian, error) {
	row, err := s.repo.FindByStudentAndGuardianForUpdate(ctx, studentID, guardianProfileID)
	if err != nil {
		return nil, err
	}
	return decisionStudentGuardianRecord(row), nil
}

func (s enrollmentStudentGuardians) LinkGuardianIfMissing(ctx context.Context, link *enrollmentCompose.StudentGuardian) (bool, error) {
	row := decisionStudentGuardianRow(link)
	inserted, err := s.repo.LinkIfNotExists(ctx, row)
	if err == nil {
		*link = *decisionStudentGuardianRecord(row)
	}
	return inserted, err
}

func (s enrollmentStudentGuardians) UpdateGuardianLinkRole(ctx context.Context, link *enrollmentCompose.StudentGuardian) (int64, error) {
	return s.repo.UpdateColumns(
		ctx,
		decisionStudentGuardianRow(link),
		"relationship_type",
		"is_emergency_contact",
		"can_pickup",
		"emergency_priority",
		"guardian_role",
		"permissions",
	)
}

func decisionStudentGuardianRow(link *enrollmentCompose.StudentGuardian) *userModels.StudentGuardian {
	row := &userModels.StudentGuardian{
		StudentID: link.StudentID, GuardianProfileID: link.GuardianProfileID,
		RelationshipType: link.RelationshipType, GuardianRole: link.GuardianRole,
		IsPrimary: link.IsPrimary, IsEmergencyContact: link.IsEmergencyContact, CanPickup: link.CanPickup,
		PickupNotes: link.PickupNotes, EmergencyPriority: link.EmergencyPriority, IsPayer: link.IsPayer,
		Permissions: link.Permissions,
	}
	row.ID, row.CreatedAt, row.UpdatedAt = link.ID, link.CreatedAt, link.UpdatedAt
	row.TenantID = link.TenantID
	return row
}

func decisionStudentGuardianRecord(row *userModels.StudentGuardian) *enrollmentCompose.StudentGuardian {
	if row == nil {
		return nil
	}
	return &enrollmentCompose.StudentGuardian{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		StudentID: row.StudentID, GuardianProfileID: row.GuardianProfileID,
		RelationshipType: row.RelationshipType, GuardianRole: row.GuardianRole,
		IsPrimary: row.IsPrimary, IsEmergencyContact: row.IsEmergencyContact, CanPickup: row.CanPickup,
		PickupNotes: row.PickupNotes, EmergencyPriority: row.EmergencyPriority, IsPayer: row.IsPayer,
		Permissions: row.Permissions,
	}
}

// enrollmentGuardianPhones reads and adds guardian phone numbers.
type enrollmentGuardianPhones struct {
	repo userModels.GuardianPhoneNumberRepository
}

func (p enrollmentGuardianPhones) GuardianPhones(ctx context.Context, guardianProfileID int64) ([]*enrollmentCompose.GuardianPhone, error) {
	rows, err := p.repo.FindByGuardianID(ctx, guardianProfileID)
	if err != nil {
		return nil, err
	}
	return decisionGuardianPhoneRecords(rows), nil
}

func (p enrollmentGuardianPhones) GuardianPhonesByGuardian(ctx context.Context, guardianProfileIDs []int64) (map[int64][]*enrollmentCompose.GuardianPhone, error) {
	rows, err := p.repo.FindByGuardianIDs(ctx, guardianProfileIDs)
	if err != nil {
		return nil, err
	}
	phones := make(map[int64][]*enrollmentCompose.GuardianPhone, len(rows))
	for id, list := range rows {
		phones[id] = decisionGuardianPhoneRecords(list)
	}
	return phones, nil
}

func (p enrollmentGuardianPhones) CreateGuardianPhone(ctx context.Context, phone *enrollmentCompose.GuardianPhone) error {
	row := &userModels.GuardianPhoneNumber{
		GuardianProfileID: phone.GuardianProfileID, PhoneNumber: phone.PhoneNumber,
		PhoneType: userModels.PhoneType(phone.PhoneType), Label: phone.Label,
		IsPrimary: phone.IsPrimary, Priority: phone.Priority,
	}
	row.ID, row.CreatedAt, row.UpdatedAt = phone.ID, phone.CreatedAt, phone.UpdatedAt
	row.TenantID = phone.TenantID
	if err := p.repo.Create(ctx, row); err != nil {
		return err
	}
	*phone = *decisionGuardianPhoneRecord(row)
	return nil
}

func decisionGuardianPhoneRecords(rows []*userModels.GuardianPhoneNumber) []*enrollmentCompose.GuardianPhone {
	phones := make([]*enrollmentCompose.GuardianPhone, 0, len(rows))
	for _, row := range rows {
		phones = append(phones, decisionGuardianPhoneRecord(row))
	}
	return phones
}

func decisionGuardianPhoneRecord(row *userModels.GuardianPhoneNumber) *enrollmentCompose.GuardianPhone {
	if row == nil {
		return nil
	}
	return &enrollmentCompose.GuardianPhone{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		GuardianProfileID: row.GuardianProfileID, PhoneNumber: row.PhoneNumber, PhoneType: string(row.PhoneType),
		Label: row.Label, IsPrimary: row.IsPrimary, Priority: row.Priority,
	}
}

// enrollmentDepartureCompanions reads the "läuft mit" edges Care Plan keeps
// and deletes those a narrowed plan no longer allows.
type enrollmentDepartureCompanions struct {
	edges  EnrollmentDepartureCompanions
	delete func(context.Context, []int64) error
}

func (c enrollmentDepartureCompanions) CompanionsOfStudent(ctx context.Context, studentID int64) ([]*enrollmentCompose.CompanionEdge, error) {
	rows, err := c.edges.ListForStudent(ctx, studentID)
	if err != nil {
		return nil, err
	}
	edges := make([]*enrollmentCompose.CompanionEdge, 0, len(rows))
	for _, row := range rows {
		edges = append(edges, &enrollmentCompose.CompanionEdge{
			ID: row.ID, StudentLowID: row.StudentLowID, StudentHighID: row.StudentHighID, Weekday: row.Weekday,
		})
	}
	return edges, nil
}

func (c enrollmentDepartureCompanions) CompanionDaysCoveredExcluding(ctx context.Context, studentIDs []int64, excludedStudentID int64) (map[int64]map[string]bool, error) {
	return c.edges.CompanionDaysCoveredExcluding(ctx, studentIDs, excludedStudentID)
}

func (c enrollmentDepartureCompanions) DeleteCompanionEdges(ctx context.Context, ids []int64) error {
	return c.delete(ctx, ids)
}

// isPostgresLockNotAvailable reports PostgreSQL's lock_not_available (55P03),
// what a NOWAIT row lock raises when another transaction holds the row.
func isPostgresLockNotAvailable(err error) bool {
	var pgErr interface {
		error
		Field(byte) string
	}
	return errors.As(err, &pgErr) && pgErr.Field('C') == "55P03"
}
