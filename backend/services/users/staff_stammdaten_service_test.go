package users_test

import (
	"context"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	usersSvc "github.com/moto-nrw/project-phoenix/services/users"
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
	db    *bun.DB
	repos *repositories.Factory
	svc   usersSvc.PersonService
	ctx   context.Context
}

func newStammdatenScenario(t *testing.T) *stammdatenScenario {
	t.Helper()

	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })

	repos := repositories.NewFactory(db)
	svc := usersSvc.NewPersonService(usersSvc.PersonServiceDependencies{
		PersonRepo:             repos.Person,
		RFIDRepo:               repos.RFIDCard,
		AccountRepo:            repos.Account,
		StudentRepo:            repos.Student,
		StaffRepo:              repos.Staff,
		TeacherRepo:            repos.Teacher,
		PersonnelNumberAudit:   repos.PersonnelNumberChange,
		StaffMasterDataRepo:    repos.StaffMasterData,
		StaffQualificationRepo: repos.StaffQualification,
		StaffFinancialRepo:     repos.StaffFinancialData,
		StammdatenAudit:        repos.StaffMasterDataChange,
		DataAccessLog:          repos.DataAccessLog,
		DB:                     db,
	})

	return &stammdatenScenario{db: db, repos: repos, svc: svc, ctx: testpkg.TenantContext(1)}
}

func (s *stammdatenScenario) auditRows(t *testing.T, staffID int64) []*auditModels.StaffMasterDataChange {
	t.Helper()
	var rows []*auditModels.StaffMasterDataChange
	err := s.db.NewSelect().
		Model(&rows).
		ModelTableExpr(`audit.staff_master_data_changes AS "staff_master_data_change"`).
		Where(`"staff_master_data_change".staff_id = ?`, staffID).
		Order("id ASC").
		Scan(context.Background())
	require.NoError(t, err)
	return rows
}

func (s *stammdatenScenario) accessLogRows(t *testing.T, accountID int64) []*auditModels.DataAccessLog {
	t.Helper()
	var rows []*auditModels.DataAccessLog
	err := s.db.NewSelect().
		Model(&rows).
		ModelTableExpr(`audit.data_access_log AS "data_access_log"`).
		Where(`"data_access_log".actor_account_id = ?`, accountID).
		Order("id ASC").
		Scan(context.Background())
	require.NoError(t, err)
	return rows
}

func strPtr(v string) *string { return &v }

func TestStaffStammdaten_PersonSection(t *testing.T) {
	s := newStammdatenScenario(t)
	staff := testpkg.CreateTestStaff(t, s.db, "Stamm", "Person")
	actor := testpkg.CreateTestStaff(t, s.db, "Stamm", "Aktor")

	birthday := timezone.NewDate(1990, 4, 12)
	err := s.svc.UpdateStaffStammdatenPerson(s.ctx, staff.ID, usersSvc.StammdatenPersonInput{
		FirstName: "Stamm",
		LastName:  "Geändert",
		Birthday:  &birthday,
		Gender:    strPtr(userModels.GenderFemale),
	}, actor.ID, "Ersteinrichtung")
	require.NoError(t, err)

	rows := s.auditRows(t, staff.ID)
	require.Len(t, rows, 3, "last_name, birthday and gender changed")
	fields := map[string]*auditModels.StaffMasterDataChange{}
	for _, row := range rows {
		fields[row.FieldName] = row
		assert.Equal(t, auditModels.StammdatenSectionPerson, row.Section)
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

	data, err := s.svc.GetStaffStammdaten(s.ctx, staff.ID)
	require.NoError(t, err)
	require.NotNil(t, data.Staff.Person)
	assert.Equal(t, "Geändert", data.Staff.Person.LastName)
	require.NotNil(t, data.Staff.Person.Birthday)
	assert.Equal(t, "1990-04-12", data.Staff.Person.Birthday.String())
	require.NotNil(t, data.MasterData)
	require.NotNil(t, data.MasterData.Gender)
	assert.Equal(t, userModels.GenderFemale, *data.MasterData.Gender)

	// No-op submit writes no additional audit rows.
	err = s.svc.UpdateStaffStammdatenPerson(s.ctx, staff.ID, usersSvc.StammdatenPersonInput{
		FirstName: "Stamm",
		LastName:  "Geändert",
		Birthday:  &birthday,
		Gender:    strPtr(userModels.GenderFemale),
	}, actor.ID, "")
	require.NoError(t, err)
	assert.Len(t, s.auditRows(t, staff.ID), 3)

	// Validation
	err = s.svc.UpdateStaffStammdatenPerson(s.ctx, staff.ID, usersSvc.StammdatenPersonInput{
		FirstName: "", LastName: "X",
	}, actor.ID, "")
	require.ErrorIs(t, err, usersSvc.ErrStaffStammdatenInvalid)
	err = s.svc.UpdateStaffStammdatenPerson(s.ctx, staff.ID, usersSvc.StammdatenPersonInput{
		FirstName: "A", LastName: "B", Gender: strPtr("other"),
	}, actor.ID, "")
	require.ErrorIs(t, err, usersSvc.ErrStaffStammdatenInvalid)
}

func TestStaffStammdaten_KontaktSection(t *testing.T) {
	s := newStammdatenScenario(t)
	staff := testpkg.CreateTestStaff(t, s.db, "Stamm", "Kontakt")
	actor := testpkg.CreateTestStaff(t, s.db, "Stamm", "Aktor")

	err := s.svc.UpdateStaffStammdatenKontakt(s.ctx, staff.ID, usersSvc.StammdatenKontaktInput{
		AddressStreet:         strPtr("Musterweg 1"),
		AddressPostalCode:     strPtr("48143"),
		AddressCity:           strPtr("Münster"),
		Phone:                 strPtr("+49 251 123456"),
		Email:                 strPtr("kontakt@example.com"),
		EmergencyContactName:  strPtr("Erika Muster"),
		EmergencyContactPhone: strPtr("+49 170 000000"),
	}, actor.ID, "Erstpflege")
	require.NoError(t, err)
	assert.Len(t, s.auditRows(t, staff.ID), 7)

	// Clearing one field writes exactly one more row.
	err = s.svc.UpdateStaffStammdatenKontakt(s.ctx, staff.ID, usersSvc.StammdatenKontaktInput{
		AddressStreet:         strPtr("Musterweg 1"),
		AddressPostalCode:     strPtr("48143"),
		AddressCity:           strPtr("Münster"),
		Phone:                 nil,
		Email:                 strPtr("kontakt@example.com"),
		EmergencyContactName:  strPtr("Erika Muster"),
		EmergencyContactPhone: strPtr("+49 170 000000"),
	}, actor.ID, "Telefon entfernt")
	require.NoError(t, err)
	rows := s.auditRows(t, staff.ID)
	require.Len(t, rows, 8)
	last := rows[len(rows)-1]
	assert.Equal(t, "phone", last.FieldName)
	assert.Equal(t, "+49 251 123456", last.OldValue)
	assert.Equal(t, "", last.NewValue)

	// Malformed email is rejected.
	err = s.svc.UpdateStaffStammdatenKontakt(s.ctx, staff.ID, usersSvc.StammdatenKontaktInput{
		Email: strPtr("not-an-email"),
	}, actor.ID, "")
	require.ErrorIs(t, err, usersSvc.ErrStaffStammdatenInvalid)
}

func TestStaffStammdaten_ArbeitsvertragSection(t *testing.T) {
	s := newStammdatenScenario(t)
	staff := testpkg.CreateTestStaff(t, s.db, "Stamm", "Vertrag")
	actor := testpkg.CreateTestStaff(t, s.db, "Stamm", "Aktor")

	entry := timezone.NewDate(2024, 8, 1)
	probation := timezone.NewDate(2025, 1, 31)
	hours := 29.5
	err := s.svc.UpdateStaffStammdatenArbeitsvertrag(s.ctx, staff.ID, usersSvc.StammdatenArbeitsvertragInput{
		EntryDate:        &entry,
		ProbationEndDate: &probation,
		WeeklyHours:      &hours,
		EmploymentType:   strPtr(userModels.EmploymentTypePartTime),
	}, actor.ID, "Vertragsdaten")
	require.NoError(t, err)

	rows := s.auditRows(t, staff.ID)
	require.Len(t, rows, 4)

	data, err := s.svc.GetStaffStammdaten(s.ctx, staff.ID)
	require.NoError(t, err)
	require.NotNil(t, data.MasterData)
	require.NotNil(t, data.MasterData.EntryDate)
	assert.Equal(t, "2024-08-01", data.MasterData.EntryDate.String())
	require.NotNil(t, data.MasterData.WeeklyHours)
	assert.InDelta(t, 29.5, *data.MasterData.WeeklyHours, 0.001)
	require.NotNil(t, data.Staff.EmploymentType)
	assert.Equal(t, userModels.EmploymentTypePartTime, *data.Staff.EmploymentType)

	// Out-of-range hours rejected.
	bad := 90.0
	err = s.svc.UpdateStaffStammdatenArbeitsvertrag(s.ctx, staff.ID, usersSvc.StammdatenArbeitsvertragInput{
		WeeklyHours: &bad,
	}, actor.ID, "")
	require.ErrorIs(t, err, usersSvc.ErrStaffStammdatenInvalid)
}

func TestStaffStammdaten_Qualifikationen(t *testing.T) {
	s := newStammdatenScenario(t)
	staff := testpkg.CreateTestStaff(t, s.db, "Stamm", "Quali")
	actor := testpkg.CreateTestStaff(t, s.db, "Stamm", "Aktor")

	acquired := timezone.NewDate(2023, 3, 10)
	expires := timezone.NewDate(2026, 3, 10)
	err := s.svc.ReplaceStaffQualifications(s.ctx, staff.ID, []usersSvc.StammdatenQualificationInput{
		{Name: "Erste-Hilfe-Kurs", AcquiredOn: &acquired, ExpiresOn: &expires},
		{Name: "Schwimmschein"},
	}, actor.ID, "Nachweise erfasst")
	require.NoError(t, err)

	data, err := s.svc.GetStaffStammdaten(s.ctx, staff.ID)
	require.NoError(t, err)
	require.Len(t, data.Qualifications, 2)
	assert.Equal(t, "Erste-Hilfe-Kurs", data.Qualifications[0].Name)

	rows := s.auditRows(t, staff.ID)
	require.Len(t, rows, 1)
	assert.Equal(t, "qualifications", rows[0].FieldName)
	assert.Contains(t, rows[0].NewValue, "Erste-Hilfe-Kurs (erworben 2023-03-10) (bis 2026-03-10)")

	// Changing only the acquisition date must persist and create an audit row.
	updatedAcquired := timezone.NewDate(2023, 4, 10)
	err = s.svc.ReplaceStaffQualifications(s.ctx, staff.ID, []usersSvc.StammdatenQualificationInput{
		{Name: "Erste-Hilfe-Kurs", AcquiredOn: &updatedAcquired, ExpiresOn: &expires},
		{Name: "Schwimmschein"},
	}, actor.ID, "Erwerbsdatum korrigiert")
	require.NoError(t, err)
	data, err = s.svc.GetStaffStammdaten(s.ctx, staff.ID)
	require.NoError(t, err)
	assert.Equal(t, updatedAcquired, *data.Qualifications[0].AcquiredOn)
	rows = s.auditRows(t, staff.ID)
	require.Len(t, rows, 2)
	assert.Contains(t, rows[1].OldValue, "erworben 2023-03-10")
	assert.Contains(t, rows[1].NewValue, "erworben 2023-04-10")

	// Replace with a shorter list.
	err = s.svc.ReplaceStaffQualifications(s.ctx, staff.ID, []usersSvc.StammdatenQualificationInput{
		{Name: "Schwimmschein"},
	}, actor.ID, "Erste Hilfe abgelaufen")
	require.NoError(t, err)
	data, err = s.svc.GetStaffStammdaten(s.ctx, staff.ID)
	require.NoError(t, err)
	require.Len(t, data.Qualifications, 1)
	assert.Equal(t, "Schwimmschein", data.Qualifications[0].Name)
	require.Len(t, s.auditRows(t, staff.ID), 3)

	// Expiry before acquisition is rejected.
	err = s.svc.ReplaceStaffQualifications(s.ctx, staff.ID, []usersSvc.StammdatenQualificationInput{
		{Name: "Kaputt", AcquiredOn: &expires, ExpiresOn: &acquired},
	}, actor.ID, "")
	require.ErrorIs(t, err, usersSvc.ErrStaffStammdatenInvalid)
}

func TestStaffStammdaten_FinancialMaskingAndAudit(t *testing.T) {
	s := newStammdatenScenario(t)
	staff := testpkg.CreateTestStaff(t, s.db, "Stamm", "Finanz")
	actor, account := testpkg.CreateTestStaffWithAccount(t, s.db, "Stamm", "Lohn")

	// Standard test IBAN with a valid mod-97 checksum.
	err := s.svc.UpdateStaffFinancial(s.ctx, staff.ID, usersSvc.StammdatenFinancialInput{
		IBAN:                 strPtr("DE89 3704 0044 0532 0130 00"),
		TaxID:                strPtr("12345678911"),
		SocialSecurityNumber: strPtr("65 170839 J 003"),
	}, actor.ID, "Lohnbüro-Erfassung")
	require.NoError(t, err)

	rows := s.auditRows(t, staff.ID)
	require.Len(t, rows, 3)
	for _, row := range rows {
		assert.Equal(t, auditModels.StammdatenSectionBankSteuer, row.Section)
		assert.NotContains(t, row.NewValue, "DE89", "audit rows must not carry plaintext IBAN")
		assert.NotContains(t, row.NewValue, "12345678911", "audit rows must not carry plaintext Steuer-ID")
		assert.NotContains(t, row.NewValue, "65170839J003", "audit rows must not carry plaintext SV-Nummer")
	}

	// Masked read: IBAN last 4 visible, others fully masked; access-logged.
	masked, err := s.svc.GetStaffFinancialMasked(s.ctx, staff.ID, account.ID, "admin")
	require.NoError(t, err)
	require.NotNil(t, masked.IBANMasked)
	assert.Equal(t, "•••• 3000", *masked.IBANMasked)
	require.NotNil(t, masked.TaxIDMasked)
	assert.NotContains(t, *masked.TaxIDMasked, "1234567")
	require.NotNil(t, masked.SocialSecurityNumberMasked)

	logs := s.accessLogRows(t, account.ID)
	require.Len(t, logs, 1)
	assert.Equal(t, auditModels.ResourceTypeStaffFinancialView, logs[0].ResourceType)

	// Reveal: full values, second access log row.
	plain, err := s.svc.RevealStaffFinancial(s.ctx, staff.ID, account.ID, "admin")
	require.NoError(t, err)
	require.NotNil(t, plain.IBAN)
	assert.Equal(t, "DE89370400440532013000", *plain.IBAN)
	require.NotNil(t, plain.TaxID)
	assert.Equal(t, "12345678911", *plain.TaxID)
	require.NotNil(t, plain.SocialSecurityNumber)
	assert.Equal(t, "65170839J003", *plain.SocialSecurityNumber)

	logs = s.accessLogRows(t, account.ID)
	require.Len(t, logs, 2)
	assert.Equal(t, auditModels.ResourceTypeStaffFinancialReveal, logs[1].ResourceType)

	// No-op write leaves no additional audit rows.
	err = s.svc.UpdateStaffFinancial(s.ctx, staff.ID, usersSvc.StammdatenFinancialInput{
		IBAN:                 strPtr("DE89370400440532013000"),
		TaxID:                strPtr("12345678911"),
		SocialSecurityNumber: strPtr("65170839J003"),
	}, actor.ID, "")
	require.NoError(t, err)
	assert.Len(t, s.auditRows(t, staff.ID), 3)
}

func TestStaffStammdaten_FinancialValidation(t *testing.T) {
	s := newStammdatenScenario(t)
	staff := testpkg.CreateTestStaff(t, s.db, "Stamm", "FinanzValid")
	actor := testpkg.CreateTestStaff(t, s.db, "Stamm", "Aktor")

	// Broken checksum
	err := s.svc.UpdateStaffFinancial(s.ctx, staff.ID, usersSvc.StammdatenFinancialInput{
		IBAN: strPtr("DE89370400440532013001"),
	}, actor.ID, "")
	require.ErrorIs(t, err, usersSvc.ErrStaffStammdatenInvalid)

	err = s.svc.UpdateStaffFinancial(s.ctx, staff.ID, usersSvc.StammdatenFinancialInput{
		TaxID: strPtr("123"),
	}, actor.ID, "")
	require.ErrorIs(t, err, usersSvc.ErrStaffStammdatenInvalid)

	err = s.svc.UpdateStaffFinancial(s.ctx, staff.ID, usersSvc.StammdatenFinancialInput{
		SocialSecurityNumber: strPtr("XX"),
	}, actor.ID, "")
	require.ErrorIs(t, err, usersSvc.ErrStaffStammdatenInvalid)

	// Unaudited financial reads are refused outright.
	unwired := usersSvc.NewPersonService(usersSvc.PersonServiceDependencies{
		StaffRepo:          s.repos.Staff,
		StaffFinancialRepo: s.repos.StaffFinancialData,
		DB:                 s.db,
	})
	_, err = unwired.GetStaffFinancialMasked(s.ctx, staff.ID, actor.ID, "admin")
	require.Error(t, err, "without the access-log repository no financial value may be served")

	// Unaudited writes are refused too.
	err = unwired.UpdateStaffFinancial(s.ctx, staff.ID, usersSvc.StammdatenFinancialInput{}, actor.ID, "")
	require.Error(t, err)
}
