package api

import (
	"context"
	"encoding/json"
	"errors"

	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory/departure"

	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/compose"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess"
	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"

	"github.com/moto-nrw/project-phoenix/modules/securityruntime"
	"github.com/moto-nrw/project-phoenix/modules/transactionruntime"
	timezone "github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// The bindings below connect Enrollment's decision flow (#3564) to the People
// Directory. The child's person, its details, a guardian's account link and
// the removal of a relationship are the owner's commands; the other guardian
// profile, relationship and phone writes still travel whole through the
// retained repositories, so such an update writes back exactly what was read
// plus the decision's changes.

// EnrollmentDecisionPeople is the People Directory the decision flow writes
// through: the child's enrollment and person, the person's details, a
// guardian's account link and the removal of a relationship.
type EnrollmentDecisionPeople interface {
	enrollmentCompose.DecisionStudentEnrollment
	peopledirectory.EnrollmentPersonCommand
	peopledirectory.PersonDetailsCommand
	LinkGuardianProfileToAccount(ctx context.Context, guardianID, accountID int64) error
	DeleteStudentGuardianLink(ctx context.Context, linkID int64) error
}

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
	row := &peopledirectory.GuardianRelationshipSnapshot{StudentID: link.StudentID, GuardianProfileID: link.GuardianProfileID, RelationshipType: link.RelationshipType, GuardianRole: link.GuardianRole}
	if link.Permissions != nil {
		row.Permissions = []byte(`{}`)
	}
	err := row.Validate()
	link.RelationshipType, link.GuardianRole = row.RelationshipType, row.GuardianRole
	if row.Permissions != nil && link.Permissions == nil {
		link.Permissions = make(map[string]any)
	}
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
	return importModels.MapRelationshipType(label)
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

// enrollmentPersons creates and renames persons through the People Directory
// and resolves a reviewer's person.
type enrollmentPersons struct {
	repo   enrollmentPersonRows
	people EnrollmentDecisionPeople
}

func (p enrollmentPersons) CreatePerson(ctx context.Context, person *enrollmentCompose.Person) error {
	created, err := p.people.CreateChildPerson(ctx, peopledirectory.ChildPerson{
		FirstName: person.FirstName, LastName: person.LastName, Birthday: decisionBirthday(person.Birthday),
	})
	if err != nil {
		return err
	}
	*person = enrollmentCompose.Person{
		ID: created.ID, TenantID: created.TenantID, CreatedAt: created.CreatedAt, UpdatedAt: created.UpdatedAt,
		FirstName: created.FirstName, LastName: created.LastName, Birthday: person.Birthday,
		TagID: created.TagID, AccountID: created.AccountID,
	}
	return nil
}

func (p enrollmentPersons) PersonByID(ctx context.Context, id int64) (*enrollmentCompose.Person, error) {
	row, err := p.repo.FindByID(ctx, id)
	return decisionPersonRecord(row), err
}

func (p enrollmentPersons) UpdatePerson(ctx context.Context, person *enrollmentCompose.Person) error {
	return p.people.SetPersonDetails(ctx, person.ID, peopledirectory.PersonDetails{
		FirstName: person.FirstName, LastName: person.LastName, Birthday: decisionBirthday(person.Birthday),
	})
}

// decisionBirthday is the People Directory's form of a decision's birthday.
func decisionBirthday(birthday *timezone.Date) string {
	if birthday == nil {
		return ""
	}
	return birthday.String()
}

func (p enrollmentPersons) PersonByAccount(ctx context.Context, accountID int64) (*enrollmentCompose.Person, error) {
	row, err := p.repo.FindByAccountID(ctx, accountID)
	return decisionPersonRecord(row), err
}

func decisionPersonRow(person *enrollmentCompose.Person) *peopledirectory.DirectoryPersonSnapshot {
	row := &peopledirectory.DirectoryPersonSnapshot{
		FirstName: person.FirstName, LastName: person.LastName, Birthday: person.Birthday,
		TagID: person.TagID, AccountID: person.AccountID,
	}
	row.ID, row.CreatedAt, row.UpdatedAt = person.ID, person.CreatedAt, person.UpdatedAt
	row.TenantID = person.TenantID
	return row
}

func decisionPersonRecord(row *peopledirectory.DirectoryPersonSnapshot) *enrollmentCompose.Person {
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
type enrollmentReviewerStaff struct{ repo enrollmentStaffRows }

func (s enrollmentReviewerStaff) StaffIDByPerson(ctx context.Context, personID int64) (int64, error) {
	staff, err := s.repo.FindByPersonID(ctx, personID)
	if err != nil || staff == nil {
		return 0, err
	}
	return staff.ID, nil
}

// enrollmentStudentRecords confirms that a student exists.
type enrollmentStudentRecords struct{ repo enrollmentStudentRows }

func (s enrollmentStudentRecords) FindStudent(ctx context.Context, id int64) error {
	_, err := s.repo.FindByID(ctx, id)
	return err
}

// decisionStudentRow is the People Directory row of a decision's student.
func decisionStudentRow(student *enrollmentCompose.Student) *peopledirectory.StudentState {
	row := &peopledirectory.StudentState{
		PersonID: student.PersonID, SchoolClass: student.SchoolClass, GroupID: student.GroupID,
		AddressStreet: student.AddressStreet, AddressCity: student.AddressCity, AddressPostalCode: student.AddressPostalCode,
		ExtraInfo: student.ExtraInfo, SupervisorNotes: student.SupervisorNotes, HealthInfo: student.HealthInfo,
		PickupStatus: student.PickupStatus, DepartureDays: student.DepartureDays,
		AllowedDepartureModes: student.AllowedDepartureModes, DepartureCompanionNote: student.DepartureCompanionNote,
		DepartureCompanionDays: student.DepartureCompanionDays, PickupDays: student.PickupDays, BusDays: student.BusDays,
		Sick: student.Sick, SickSince: student.SickSince, Excused: student.Excused, ExcusedSince: student.ExcusedSince,
		Status: string(student.Status), EnrolledFrom: student.EnrolledFrom, EnrolledUntil: student.EnrolledUntil,
		PhotoPath: student.PhotoPath, PhotoConsentGivenAt: student.PhotoConsentGivenAt, PhotoConsentGivenBy: student.PhotoConsentGivenBy,
		AGBAcceptedAt: student.AGBAcceptedAt, DataProcessingAcceptedAt: student.DataProcessingAcceptedAt,
		EmailContactAcceptedAt: student.EmailContactAcceptedAt,
	}
	row.ID, row.CreatedAt, row.UpdatedAt = student.ID, student.CreatedAt, student.UpdatedAt
	row.TenantID = student.TenantID
	if baseline := student.DepartureBaseline; baseline != nil {
		row.DepartureBaseline = &departure.Plan{
			DepartureDays: baseline.DepartureDays, AllowedDepartureModes: baseline.AllowedDepartureModes,
			BusDays: baseline.BusDays, PickupDays: baseline.PickupDays,
		}
	}
	return row
}

// enrollmentStudentAudit records the decision flow's student changes on the
// People Directory's change trail.
type enrollmentStudentAudit struct {
	auditor EnrollmentStudentAuditor
}

func (a enrollmentStudentAudit) RecordChangesForActor(ctx context.Context, before, after *enrollmentCompose.Student, editedBy int64) error {
	return a.auditor.RecordChangesForActor(ctx, decisionStudentRow(before), decisionStudentRow(after), editedBy)
}

func (a enrollmentStudentAudit) RecordSystemStatusChange(ctx context.Context, studentID int64, before, after string) error {
	return a.auditor.RecordSystemStatusChange(ctx, studentID, string(before), string(after))
}

// enrollmentGuardianProfiles reads and writes the tenant's guardian profiles;
// accounts links a profile's account, which only the decision flow does.
type enrollmentGuardianProfiles struct {
	repo     enrollmentGuardianProfileRows
	accounts EnrollmentDecisionPeople
}

func (g enrollmentGuardianProfiles) GuardianProfileByAccount(ctx context.Context, accountID int64) (*enrollmentCompose.GuardianProfile, error) {
	row, err := g.repo.FindByAccountID(ctx, accountID)
	if errors.Is(err, peopledirectory.ErrGuardianProfileNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return decisionGuardianProfileRecord(row), nil
}

func (g enrollmentGuardianProfiles) GuardianProfileByEmail(ctx context.Context, email string) (*enrollmentCompose.GuardianProfile, error) {
	row, err := g.repo.FindByEmail(ctx, email)
	if errors.Is(err, peopledirectory.ErrGuardianProfileNotFound) || errors.Is(err, transactionruntime.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return decisionGuardianProfileRecord(row), nil
}

func (g enrollmentGuardianProfiles) GuardianProfileHasActivePortalAccount(ctx context.Context, profileID int64) (bool, error) {
	profiles, err := g.repo.FindLoginReadyPortalProfilesByIDs(ctx, []int64{profileID})
	if err != nil {
		return false, err
	}
	_, reachable := profiles[profileID]
	return reachable, nil
}

type guardianInvitationReader interface {
	ListOpenGuardianInvitations(context.Context, []int64) ([]identityaccess.GuardianInvitation, error)
}

type enrollmentGuardianInvitationAvailability struct {
	invitations guardianInvitationReader
}

func (a enrollmentGuardianInvitationAvailability) HasRedeemableGuardianInvitation(ctx context.Context, profileID int64) (bool, error) {
	invitations, err := a.invitations.ListOpenGuardianInvitations(ctx, []int64{profileID})
	if err != nil {
		return false, err
	}
	for _, invitation := range invitations {
		if invitation.ApprovalAllowsRedemption() {
			return true, nil
		}
	}
	return false, nil
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
	if g.accounts == nil {
		return errors.New("enrollment guardian profiles: the account link is bound to the decision flow only")
	}
	return g.accounts.LinkGuardianProfileToAccount(ctx, profileID, accountID)
}

func decisionGuardianProfileRow(profile *enrollmentCompose.GuardianProfile) *peopledirectory.GuardianProfileSnapshot {
	row := &peopledirectory.GuardianProfileSnapshot{
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

func decisionGuardianProfileRecord(row *peopledirectory.GuardianProfileSnapshot) *enrollmentCompose.GuardianProfile {
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
// through the retained composition seam over their three owners and removes
// them through the People Directory.
type enrollmentStudentGuardians struct {
	repo   enrollmentStudentGuardianRows
	people EnrollmentDecisionPeople
}

func (s enrollmentStudentGuardians) GuardianLinksOfStudent(ctx context.Context, studentID int64) ([]*enrollmentCompose.StudentGuardian, error) {
	rows, err := s.repo.FindByStudentID(ctx, studentID)
	if err != nil {
		return nil, err
	}
	links := make([]*enrollmentCompose.StudentGuardian, 0, len(rows))
	for _, row := range rows {
		value, mapErr := decisionStudentGuardianRecord(row)
		if mapErr != nil {
			return nil, mapErr
		}
		links = append(links, value)
	}
	return links, nil
}

func (s enrollmentStudentGuardians) CreateGuardianLink(ctx context.Context, link *enrollmentCompose.StudentGuardian) error {
	row, err := decisionStudentGuardianRow(link)
	if err != nil {
		return err
	}
	if err := s.repo.Create(ctx, row); err != nil {
		return err
	}
	mapped, mapErr := decisionStudentGuardianRecord(row)
	if mapErr != nil {
		return mapErr
	}
	*link = *mapped
	return nil
}

func (s enrollmentStudentGuardians) UpdateGuardianLink(ctx context.Context, link *enrollmentCompose.StudentGuardian) error {
	row, err := decisionStudentGuardianRow(link)
	if err != nil {
		return err
	}
	return s.repo.Update(ctx, row)
}

func (s enrollmentStudentGuardians) DeleteGuardianLink(ctx context.Context, id int64) error {
	return s.people.DeleteStudentGuardianLink(ctx, id)
}

func (s enrollmentStudentGuardians) GuardianLinkForUpdate(ctx context.Context, studentID, guardianProfileID int64) (*enrollmentCompose.StudentGuardian, error) {
	row, err := s.repo.FindByStudentAndGuardianForUpdate(ctx, studentID, guardianProfileID)
	if err != nil {
		return nil, err
	}
	return decisionStudentGuardianRecord(row)
}

func (s enrollmentStudentGuardians) LinkGuardianIfMissing(ctx context.Context, link *enrollmentCompose.StudentGuardian) (bool, error) {
	row, err := decisionStudentGuardianRow(link)
	if err != nil {
		return false, err
	}
	inserted, err := s.repo.LinkIfNotExists(ctx, row)
	if err == nil {
		mapped, mapErr := decisionStudentGuardianRecord(row)
		if mapErr != nil {
			return false, mapErr
		}
		*link = *mapped
	}
	return inserted, err
}

func (s enrollmentStudentGuardians) UpdateGuardianLinkRole(ctx context.Context, link *enrollmentCompose.StudentGuardian) (int64, error) {
	row, err := decisionStudentGuardianRow(link)
	if err != nil {
		return 0, err
	}
	return s.repo.UpdateColumns(ctx, row, peopledirectory.GuardianRoleColumns()...)
}

func decisionStudentGuardianRow(link *enrollmentCompose.StudentGuardian) (*peopledirectory.GuardianRelationshipSnapshot, error) {
	row := &peopledirectory.GuardianRelationshipSnapshot{
		StudentID: link.StudentID, GuardianProfileID: link.GuardianProfileID,
		RelationshipType: link.RelationshipType, GuardianRole: link.GuardianRole,
		IsPrimary: link.IsPrimary, IsEmergencyContact: link.IsEmergencyContact, CanPickup: link.CanPickup,
		PickupNotes: link.PickupNotes, EmergencyPriority: link.EmergencyPriority, IsPayer: link.IsPayer,
	}
	row.ID, row.CreatedAt, row.UpdatedAt = link.ID, link.CreatedAt, link.UpdatedAt
	row.TenantID = link.TenantID
	if link.Permissions != nil {
		raw, err := json.Marshal(link.Permissions)
		if err != nil {
			return nil, err
		}
		row.Permissions = raw
	}
	return row, nil
}

func decisionStudentGuardianRecord(row *peopledirectory.GuardianRelationshipSnapshot) (*enrollmentCompose.StudentGuardian, error) {
	if row == nil {
		return nil, nil
	}
	value := &enrollmentCompose.StudentGuardian{
		ID: row.ID, TenantID: row.TenantID, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt,
		StudentID: row.StudentID, GuardianProfileID: row.GuardianProfileID,
		RelationshipType: row.RelationshipType, GuardianRole: row.GuardianRole,
		IsPrimary: row.IsPrimary, IsEmergencyContact: row.IsEmergencyContact, CanPickup: row.CanPickup,
		PickupNotes: row.PickupNotes, EmergencyPriority: row.EmergencyPriority, IsPayer: row.IsPayer,
	}
	if row.Permissions != nil {
		if err := json.Unmarshal(row.Permissions, &value.Permissions); err != nil {
			return nil, err
		}
	}
	return value, nil

}

// enrollmentGuardianPhones reads and adds guardian phone numbers.
type enrollmentGuardianPhones struct {
	repo enrollmentGuardianPhoneRows
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
	row := &peopledirectory.GuardianProfilePhoneSnapshot{
		GuardianProfileID: phone.GuardianProfileID, PhoneNumber: phone.PhoneNumber,
		PhoneType: string(phone.PhoneType), Label: phone.Label,
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

func decisionGuardianPhoneRecords(rows []*peopledirectory.GuardianProfilePhoneSnapshot) []*enrollmentCompose.GuardianPhone {
	phones := make([]*enrollmentCompose.GuardianPhone, 0, len(rows))
	for _, row := range rows {
		phones = append(phones, decisionGuardianPhoneRecord(row))
	}
	return phones
}

func decisionGuardianPhoneRecord(row *peopledirectory.GuardianProfilePhoneSnapshot) *enrollmentCompose.GuardianPhone {
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
