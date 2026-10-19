package contracttest_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/api"
	owners "github.com/moto-nrw/project-phoenix/roots/owners"

	userModels "github.com/moto-nrw/project-phoenix/modules/peopledirectory/internal/rows"
	"github.com/moto-nrw/project-phoenix/modules/workforce"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// Staff Stammdaten write/read paths (#1423): every section change writes
// field-level audit rows in the same transaction, financial reads are
// access-logged before any value is served, and financial audit rows carry
// masked values only.

type stammdatenScenario struct {
	db                    *bun.DB
	repos                 *api.RepositoryTestFactory
	reposPeople           *api.PeopleRepositorySuiteFactory
	reposPeopleMembership *api.SchoolMembershipSuiteRepositories
	svc                   workforce.StaffRecordAdmin
	ctx                   context.Context
}

func newStammdatenScenario(t *testing.T) *stammdatenScenario {
	t.Helper()

	db := testpkg.SetupTestDB(t)

	repos := api.NewRepositoryTestFactory(testpkg.NewDatabase(t, db))
	reposPeople := api.NewPeopleRepositorySuiteFactory(testpkg.NewDatabase(t, db))
	reposPeopleMembership := api.NewSchoolMembershipSuiteRepositories(testpkg.NewDatabase(t, db))
	svc, err := api.NewStaffAdmin(testpkg.MustDatabase(db), reposPeople.Staff, reposPeople.Person, reposPeopleMembership.SchoolMembership(), stammdatenAdminDeps(db))
	require.NoError(t, err)

	return &stammdatenScenario{db: db, repos: repos, reposPeople: reposPeople, reposPeopleMembership: reposPeopleMembership, svc: svc, ctx: testpkg.Ctx(t)}
}

func stammdatenAdminDeps(db *bun.DB) api.StaffAdminDependencies {
	return api.StaffAdminDependencies{
		MasterDataAudit: owners.NewTestAuditPersistence(testpkg.MustDatabase(db), nil).StaffMasterDataChange, PersonnelNumber: owners.NewTestAuditPersistence(testpkg.MustDatabase(db), nil).PersonnelNumberChange, DataAccessLog: owners.NewTestAuditPersistence(testpkg.MustDatabase(db), nil).DataAccessLog,
	}
}

func (s *stammdatenScenario) auditRows(t *testing.T, staffID int64) []*testpkg.StaffMasterDataChange {
	t.Helper()
	var rows []*testpkg.StaffMasterDataChange
	err := s.db.NewSelect().
		Model(&rows).
		ModelTableExpr(`audit.staff_master_data_changes AS "staff_master_data_change"`).
		Where(`"staff_master_data_change".staff_id = ?`, staffID).
		Order("id ASC").
		Scan(context.Background())
	require.NoError(t, err)
	return rows
}

func (s *stammdatenScenario) accessLogRows(t *testing.T, accountID int64) []*testpkg.DataAccessLog {
	t.Helper()
	var rows []*testpkg.DataAccessLog
	err := s.db.NewSelect().
		Model(&rows).
		ModelTableExpr(`audit.data_access_log AS "data_access_log"`).
		Where(`"data_access_log".actor_account_id = ?`, accountID).
		Order("id ASC").
		Scan(context.Background())
	require.NoError(t, err)
	return rows
}

func stammdatenStrPtr(v string) *string { return &v }

func TestStaffStammdaten_PersonSection(t *testing.T) {
	t.Parallel()

	s := newStammdatenScenario(t)
	staff := testpkg.CreateTestStaff(t, s.db, "Stamm", "Person")
	actor := testpkg.CreateTestStaff(t, s.db, "Stamm", "Aktor")

	birthday := "1990-04-12"
	err := testpkg.TenantCommand4(t, s.ctx, s.db, s.svc.UpdateStaffStammdatenPerson, staff.ID, workforce.StammdatenPersonInput{
		FirstName: "Stamm",
		LastName:  "Geändert",
		Birthday:  &birthday,
		Gender:    stammdatenStrPtr(userModels.GenderFemale),
	}, actor.ID, "Ersteinrichtung")
	require.NoError(t, err)

	rows := s.auditRows(t, staff.ID)
	require.Len(t, rows, 3, "last_name, birthday and gender changed")
	fields := map[string]*testpkg.StaffMasterDataChange{}
	for _, row := range rows {
		fields[row.FieldName] = row
		assert.Equal(t, testpkg.AuditStammdatenSectionPerson, row.Section)
		assert.Equal(t, actor.ID, row.ChangedBy)
		assert.Equal(t, "Ersteinrichtung", row.Note)
	}
	require.Contains(t, fields, "last_name")
	assert.Equal(t, "Person", fields["last_name"].OldValue)
	assert.Equal(t, "Geändert", fields["last_name"].NewValue)
	require.Contains(t, fields, "birthday")
	assert.Equal(t, "1990-04-12", fields["birthday"].NewValue)
	require.Contains(t, fields, "gender")
	assert.Equal(t, userModels.GenderFemale, fields["gender"].NewValue)

	data, err := testpkg.TenantQuery1(t, s.ctx, s.db, s.svc.StaffStammdaten, staff.ID)
	require.NoError(t, err)
	assert.Equal(t, "Geändert", data.Staff.LastName)
	assert.Equal(t, "1990-04-12", data.Staff.Birthday)
	require.NotNil(t, data.MasterData)
	require.NotNil(t, data.MasterData.Gender)
	assert.Equal(t, userModels.GenderFemale, *data.MasterData.Gender)

	// No-op submit writes no additional audit rows.
	err = testpkg.TenantCommand4(t, s.ctx, s.db, s.svc.UpdateStaffStammdatenPerson, staff.ID, workforce.StammdatenPersonInput{
		FirstName: "Stamm",
		LastName:  "Geändert",
		Birthday:  &birthday,
		Gender:    stammdatenStrPtr(userModels.GenderFemale),
	}, actor.ID, "")
	require.NoError(t, err)
	assert.Len(t, s.auditRows(t, staff.ID), 3)

	// Validation
	err = testpkg.TenantCommand4(t, s.ctx, s.db, s.svc.UpdateStaffStammdatenPerson, staff.ID, workforce.StammdatenPersonInput{
		FirstName: "", LastName: "X",
	}, actor.ID, "")
	require.ErrorIs(t, err, workforce.ErrStaffStammdatenInvalid)
	err = testpkg.TenantCommand4(t, s.ctx, s.db, s.svc.UpdateStaffStammdatenPerson, staff.ID, workforce.StammdatenPersonInput{
		FirstName: "A", LastName: "B", Gender: stammdatenStrPtr("other"),
	}, actor.ID, "")
	require.ErrorIs(t, err, workforce.ErrStaffStammdatenInvalid)
}

func TestStaffStammdaten_KontaktSection(t *testing.T) {
	t.Parallel()

	s := newStammdatenScenario(t)
	staff := testpkg.CreateTestStaff(t, s.db, "Stamm", "Kontakt")
	actor := testpkg.CreateTestStaff(t, s.db, "Stamm", "Aktor")

	err := testpkg.TenantCommand4(t, s.ctx, s.db, s.svc.UpdateStaffStammdatenKontakt, staff.ID, workforce.StammdatenKontaktInput{
		AddressStreet:         stammdatenStrPtr("Musterweg 1"),
		AddressPostalCode:     stammdatenStrPtr("48143"),
		AddressCity:           stammdatenStrPtr("Münster"),
		Phone:                 stammdatenStrPtr("+49 251 123456"),
		Email:                 stammdatenStrPtr("kontakt@example.com"),
		EmergencyContactName:  stammdatenStrPtr("Erika Muster"),
		EmergencyContactPhone: stammdatenStrPtr("+49 170 000000"),
	}, actor.ID, "Erstpflege")
	require.NoError(t, err)
	assert.Len(t, s.auditRows(t, staff.ID), 7)

	// Clearing one field writes exactly one more row.
	err = testpkg.TenantCommand4(t, s.ctx, s.db, s.svc.UpdateStaffStammdatenKontakt, staff.ID, workforce.StammdatenKontaktInput{
		AddressStreet:         stammdatenStrPtr("Musterweg 1"),
		AddressPostalCode:     stammdatenStrPtr("48143"),
		AddressCity:           stammdatenStrPtr("Münster"),
		Phone:                 nil,
		Email:                 stammdatenStrPtr("kontakt@example.com"),
		EmergencyContactName:  stammdatenStrPtr("Erika Muster"),
		EmergencyContactPhone: stammdatenStrPtr("+49 170 000000"),
	}, actor.ID, "Telefon entfernt")
	require.NoError(t, err)
	rows := s.auditRows(t, staff.ID)
	require.Len(t, rows, 8)
	last := rows[len(rows)-1]
	assert.Equal(t, "phone", last.FieldName)
	assert.Equal(t, "+49 251 123456", last.OldValue)
	assert.Equal(t, "", last.NewValue)

	// Malformed email is rejected.
	err = testpkg.TenantCommand4(t, s.ctx, s.db, s.svc.UpdateStaffStammdatenKontakt, staff.ID, workforce.StammdatenKontaktInput{
		Email: stammdatenStrPtr("not-an-email"),
	}, actor.ID, "")
	require.ErrorIs(t, err, workforce.ErrStaffStammdatenInvalid)
}

func TestStaffStammdaten_ArbeitsvertragSection(t *testing.T) {
	t.Parallel()

	s := newStammdatenScenario(t)
	staff := testpkg.CreateTestStaff(t, s.db, "Stamm", "Vertrag")
	actor := testpkg.CreateTestStaff(t, s.db, "Stamm", "Aktor")

	entry := "2024-08-01"
	probation := "2025-01-31"
	hours := 29.5
	err := testpkg.TenantCommand4(t, s.ctx, s.db, s.svc.UpdateStaffStammdatenArbeitsvertrag, staff.ID, workforce.StammdatenArbeitsvertragInput{
		EntryDate:        &entry,
		ProbationEndDate: &probation,
		WeeklyHours:      &hours,
		EmploymentType:   stammdatenStrPtr(userModels.EmploymentTypePartTime),
	}, actor.ID, "Vertragsdaten")
	require.NoError(t, err)

	rows := s.auditRows(t, staff.ID)
	require.Len(t, rows, 4)

	data, err := testpkg.TenantQuery1(t, s.ctx, s.db, s.svc.StaffStammdaten, staff.ID)
	require.NoError(t, err)
	require.NotNil(t, data.MasterData)
	assert.Equal(t, "2024-08-01", data.MasterData.EntryDate)
	require.NotNil(t, data.MasterData.WeeklyHours)
	assert.InDelta(t, 29.5, *data.MasterData.WeeklyHours, 0.001)
	require.NotNil(t, data.Staff.EmploymentType)
	assert.Equal(t, userModels.EmploymentTypePartTime, *data.Staff.EmploymentType)

	// Out-of-range hours rejected.
	bad := 90.0
	err = testpkg.TenantCommand4(t, s.ctx, s.db, s.svc.UpdateStaffStammdatenArbeitsvertrag, staff.ID, workforce.StammdatenArbeitsvertragInput{
		WeeklyHours: &bad,
	}, actor.ID, "")
	require.ErrorIs(t, err, workforce.ErrStaffStammdatenInvalid)
}

func TestStaffStammdaten_Qualifikationen(t *testing.T) {
	t.Parallel()

	s := newStammdatenScenario(t)
	staff := testpkg.CreateTestStaff(t, s.db, "Stamm", "Quali")
	actor := testpkg.CreateTestStaff(t, s.db, "Stamm", "Aktor")

	acquired := "2023-03-10"
	expires := "2026-03-10"
	err := testpkg.TenantCommand4(t, s.ctx, s.db, s.svc.ReplaceStaffQualificationList, staff.ID, []workforce.StammdatenQualificationInput{
		{Name: "Erste-Hilfe-Kurs", AcquiredOn: &acquired, ExpiresOn: &expires},
		{Name: "Schwimmschein"},
	}, actor.ID, "Nachweise erfasst")
	require.NoError(t, err)

	data, err := testpkg.TenantQuery1(t, s.ctx, s.db, s.svc.StaffStammdaten, staff.ID)
	require.NoError(t, err)
	require.Len(t, data.Qualifications, 2)
	assert.Equal(t, "Erste-Hilfe-Kurs", data.Qualifications[0].Name)

	rows := s.auditRows(t, staff.ID)
	require.Len(t, rows, 1)
	assert.Equal(t, "qualifications", rows[0].FieldName)
	assert.Contains(t, rows[0].NewValue, "Erste-Hilfe-Kurs (erworben 2023-03-10) (bis 2026-03-10)")

	// Changing only the acquisition date must persist and create an audit row.
	updatedAcquired := "2023-04-10"
	err = testpkg.TenantCommand4(t, s.ctx, s.db, s.svc.ReplaceStaffQualificationList, staff.ID, []workforce.StammdatenQualificationInput{
		{Name: "Erste-Hilfe-Kurs", AcquiredOn: &updatedAcquired, ExpiresOn: &expires},
		{Name: "Schwimmschein"},
	}, actor.ID, "Erwerbsdatum korrigiert")
	require.NoError(t, err)
	data, err = testpkg.TenantQuery1(t, s.ctx, s.db, s.svc.StaffStammdaten, staff.ID)
	require.NoError(t, err)
	assert.Equal(t, updatedAcquired, data.Qualifications[0].AcquiredOn)
	rows = s.auditRows(t, staff.ID)
	require.Len(t, rows, 2)
	assert.Contains(t, rows[1].OldValue, "erworben 2023-03-10")
	assert.Contains(t, rows[1].NewValue, "erworben 2023-04-10")

	// Replace with a shorter list.
	err = testpkg.TenantCommand4(t, s.ctx, s.db, s.svc.ReplaceStaffQualificationList, staff.ID, []workforce.StammdatenQualificationInput{
		{Name: "Schwimmschein"},
	}, actor.ID, "Erste Hilfe abgelaufen")
	require.NoError(t, err)
	data, err = testpkg.TenantQuery1(t, s.ctx, s.db, s.svc.StaffStammdaten, staff.ID)
	require.NoError(t, err)
	require.Len(t, data.Qualifications, 1)
	assert.Equal(t, "Schwimmschein", data.Qualifications[0].Name)
	require.Len(t, s.auditRows(t, staff.ID), 3)

	// Expiry before acquisition is rejected.
	err = testpkg.TenantCommand4(t, s.ctx, s.db, s.svc.ReplaceStaffQualificationList, staff.ID, []workforce.StammdatenQualificationInput{
		{Name: "Kaputt", AcquiredOn: &expires, ExpiresOn: &acquired},
	}, actor.ID, "")
	require.ErrorIs(t, err, workforce.ErrStaffStammdatenInvalid)
}

func TestStaffStammdaten_FinancialMaskingAndAudit(t *testing.T) {
	t.Parallel()

	s := newStammdatenScenario(t)
	staff := testpkg.CreateTestStaff(t, s.db, "Stamm", "Finanz")
	actor, account := testpkg.CreateTestStaffWithAccount(t, s.db, "Stamm", "Lohn")

	// Standard test IBAN with a valid mod-97 checksum.
	err := testpkg.TenantCommand4(t, s.ctx, s.db, s.svc.UpdateStaffFinancial, staff.ID, workforce.StammdatenFinancialInput{
		IBAN:                 stammdatenStrPtr("DE89 3704 0044 0532 0130 00"),
		TaxID:                stammdatenStrPtr("12345678911"),
		SocialSecurityNumber: stammdatenStrPtr("65 170839 J 003"),
	}, actor.ID, "Lohnbüro-Erfassung")
	require.NoError(t, err)

	rows := s.auditRows(t, staff.ID)
	require.Len(t, rows, 3)
	for _, row := range rows {
		assert.Equal(t, testpkg.AuditStammdatenSectionBankSteuer, row.Section)
		assert.NotContains(t, row.NewValue, "DE89", "audit rows must not carry plaintext IBAN")
		assert.NotContains(t, row.NewValue, "12345678911", "audit rows must not carry plaintext Steuer-ID")
		assert.NotContains(t, row.NewValue, "65170839J003", "audit rows must not carry plaintext SV-Nummer")
	}

	// Masked read: IBAN last 4 visible, others fully masked; access-logged.
	masked, err := testpkg.TenantQuery3(t, s.ctx, s.db, s.svc.StaffFinancialMasked, staff.ID, account.ID, "admin")
	require.NoError(t, err)
	require.NotNil(t, masked.IBANMasked)
	assert.Equal(t, "•••• 3000", *masked.IBANMasked)
	require.NotNil(t, masked.TaxIDMasked)
	assert.NotContains(t, *masked.TaxIDMasked, "1234567")
	require.NotNil(t, masked.SocialSecurityNumberMasked)

	logs := s.accessLogRows(t, account.ID)
	require.Len(t, logs, 1)
	assert.Equal(t, testpkg.AuditResourceTypeStaffFinancialView, logs[0].ResourceType)

	// Reveal: full values, second access log row.
	plain, err := testpkg.TenantQuery3(t, s.ctx, s.db, s.svc.RevealStaffFinancial, staff.ID, account.ID, "admin")
	require.NoError(t, err)
	require.NotNil(t, plain.IBAN)
	assert.Equal(t, "DE89370400440532013000", *plain.IBAN)
	require.NotNil(t, plain.TaxID)
	assert.Equal(t, "12345678911", *plain.TaxID)
	require.NotNil(t, plain.SocialSecurityNumber)
	assert.Equal(t, "65170839J003", *plain.SocialSecurityNumber)

	logs = s.accessLogRows(t, account.ID)
	require.Len(t, logs, 2)
	assert.Equal(t, testpkg.AuditResourceTypeStaffFinancialReveal, logs[1].ResourceType)

	// No-op write leaves no additional audit rows.
	err = testpkg.TenantCommand4(t, s.ctx, s.db, s.svc.UpdateStaffFinancial, staff.ID, workforce.StammdatenFinancialInput{
		IBAN:                 stammdatenStrPtr("DE89370400440532013000"),
		TaxID:                stammdatenStrPtr("12345678911"),
		SocialSecurityNumber: stammdatenStrPtr("65170839J003"),
	}, actor.ID, "")
	require.NoError(t, err)
	assert.Len(t, s.auditRows(t, staff.ID), 3)
}

func TestStaffStammdaten_FinancialValidation(t *testing.T) {
	t.Parallel()

	s := newStammdatenScenario(t)
	staff := testpkg.CreateTestStaff(t, s.db, "Stamm", "FinanzValid")
	actor := testpkg.CreateTestStaff(t, s.db, "Stamm", "Aktor")

	// Broken checksum
	err := testpkg.TenantCommand4(t, s.ctx, s.db, s.svc.UpdateStaffFinancial, staff.ID, workforce.StammdatenFinancialInput{
		IBAN: stammdatenStrPtr("DE89370400440532013001"),
	}, actor.ID, "")
	require.ErrorIs(t, err, workforce.ErrStaffStammdatenInvalid)

	err = testpkg.TenantCommand4(t, s.ctx, s.db, s.svc.UpdateStaffFinancial, staff.ID, workforce.StammdatenFinancialInput{
		TaxID: stammdatenStrPtr("123"),
	}, actor.ID, "")
	require.ErrorIs(t, err, workforce.ErrStaffStammdatenInvalid)

	err = testpkg.TenantCommand4(t, s.ctx, s.db, s.svc.UpdateStaffFinancial, staff.ID, workforce.StammdatenFinancialInput{
		SocialSecurityNumber: stammdatenStrPtr("XX"),
	}, actor.ID, "")
	require.ErrorIs(t, err, workforce.ErrStaffStammdatenInvalid)

	// Unaudited financial reads are refused outright: the administration
	// cannot be composed without the access-log repository.
	noAccessLog := stammdatenAdminDeps(s.db)
	noAccessLog.DataAccessLog = nil
	_, err = api.NewStaffAdmin(testpkg.MustDatabase(s.db), s.reposPeople.Staff, s.reposPeople.Person, s.reposPeopleMembership.SchoolMembership(), noAccessLog)
	require.Error(t, err, "without the access-log repository no financial value may be served")

	// Unaudited writes are refused too.
	noAudit := stammdatenAdminDeps(s.db)
	noAudit.MasterDataAudit = nil
	_, err = api.NewStaffAdmin(testpkg.MustDatabase(s.db), s.reposPeople.Staff, s.reposPeople.Person, s.reposPeopleMembership.SchoolMembership(), noAudit)
	require.Error(t, err)
}
