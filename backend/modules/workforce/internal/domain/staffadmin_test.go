package domain

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func ptr[T any](value T) *T { return &value }

func TestNormalizePersonnelNumber(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value *string
		want  *string
		err   error
	}{
		{name: "nil clears", value: nil, want: nil},
		{name: "blank clears", value: ptr("   "), want: nil},
		{name: "digits are trimmed", value: ptr(" 4711 "), want: ptr("4711")},
		{name: "nine digits fit", value: ptr("123456789"), want: ptr("123456789")},
		{name: "ten digits are junk", value: ptr("1234567890"), err: ErrPersonnelNumberInvalid},
		{name: "letters are junk", value: ptr("A12"), err: ErrPersonnelNumberInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := NormalizePersonnelNumber(tt.value)
			if tt.err != nil {
				require.ErrorIs(t, err, tt.err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNormalizePersonSection(t *testing.T) {
	t.Parallel()

	got, err := NormalizePersonSection(PersonSection{
		FirstName: "  Ada ", LastName: " Lovelace ", Birthday: ptr("1990-04-12"), Gender: ptr(GenderFemale),
	})
	require.NoError(t, err)
	assert.Equal(t, "Ada", got.FirstName)
	assert.Equal(t, "Lovelace", got.LastName)

	_, err = NormalizePersonSection(PersonSection{FirstName: " ", LastName: "X"})
	require.ErrorIs(t, err, ErrStaffStammdatenInvalid)
	assert.Equal(t, "invalid stammdaten value: first and last name are required", err.Error())

	_, err = NormalizePersonSection(PersonSection{FirstName: "A", LastName: "B", Gender: ptr("robot")})
	require.ErrorIs(t, err, ErrStaffStammdatenInvalid)
	assert.Equal(t, "invalid stammdaten value: unknown gender value", err.Error())

	// The calendar day is checked before the names, in its bare wording.
	_, err = NormalizePersonSection(PersonSection{FirstName: "", LastName: "", Birthday: ptr("12.04.1990")})
	require.ErrorIs(t, err, ErrStaffStammdatenInvalid)
	assert.Equal(t, "birthday must be YYYY-MM-DD", err.Error())
}

func TestNormalizeContactSection(t *testing.T) {
	t.Parallel()

	got, err := NormalizeContactSection(ContactSection{
		AddressStreet: ptr("  Hauptstr. 1 "), Phone: ptr("   "), Email: ptr(" a@b.de "),
	})
	require.NoError(t, err)
	assert.Equal(t, "Hauptstr. 1", *got.AddressStreet)
	assert.Nil(t, got.Phone, "blank clears the field")
	assert.Equal(t, "a@b.de", *got.Email)

	for _, email := range []string{"no-at-sign", "a b@c.de"} {
		_, err = NormalizeContactSection(ContactSection{Email: ptr(email)})
		require.ErrorIs(t, err, ErrStaffStammdatenInvalid, email)
		assert.Equal(t, "invalid stammdaten value: malformed email address", err.Error())
	}
}

func TestNormalizeContractSection(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   ContractSection
		want string
	}{
		{name: "contract end before entry", in: ContractSection{EntryDate: ptr("2024-08-01"), ContractEndDate: ptr("2024-07-31")},
			want: "invalid stammdaten value: contract end date must not be before entry date"},
		{name: "probation end before entry", in: ContractSection{EntryDate: ptr("2024-08-01"), ProbationEndDate: ptr("2024-07-31")},
			want: "invalid stammdaten value: probation end date must not be before entry date"},
		{name: "weekly hours above cap", in: ContractSection{WeeklyHours: ptr(80.5)},
			want: "invalid stammdaten value: weekly hours must be between 0 and 80"},
		{name: "negative weekly hours", in: ContractSection{WeeklyHours: ptr(-1.0)},
			want: "invalid stammdaten value: weekly hours must be between 0 and 80"},
		{name: "unknown employment type", in: ContractSection{EmploymentType: ptr("freelance")},
			want: "invalid stammdaten value: unknown employment type"},
		{name: "date in wrong format", in: ContractSection{ContractEndDate: ptr("2024-13-01")},
			want: "contract_end_date must be YYYY-MM-DD"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := NormalizeContractSection(tt.in)
			require.ErrorIs(t, err, ErrStaffStammdatenInvalid)
			assert.Equal(t, tt.want, err.Error())
		})
	}

	_, err := NormalizeContractSection(ContractSection{
		EntryDate: ptr("2024-08-01"), ContractEndDate: ptr("2024-08-01"), WeeklyHours: ptr(80.0), EmploymentType: ptr(EmploymentTypeMinijob),
	})
	require.NoError(t, err, "equal days and the 80 hour cap are allowed")
}

func TestNormalizeQualificationsChecksEveryDayBeforeAnyName(t *testing.T) {
	t.Parallel()

	_, err := NormalizeQualifications([]QualificationInput{
		{Name: "  "},
		{Name: "Erste Hilfe", AcquiredOn: ptr("bald")},
	})
	require.ErrorIs(t, err, ErrStaffStammdatenInvalid)
	assert.Equal(t, "acquired_on must be YYYY-MM-DD", err.Error(), "a later row's day is reported before an earlier row's missing name")

	_, err = NormalizeQualifications([]QualificationInput{{Name: "  "}})
	assert.Equal(t, "invalid stammdaten value: qualification name is required", err.Error())

	_, err = NormalizeQualifications([]QualificationInput{
		{Name: "Kaputt", AcquiredOn: ptr("2026-03-10"), ExpiresOn: ptr("2023-03-10")},
	})
	assert.Equal(t, "invalid stammdaten value: qualification expiry before acquisition", err.Error())

	rows, err := NormalizeQualifications([]QualificationInput{{Name: " Schwimmschein ", ExpiresOn: ptr("2027-01-01")}})
	require.NoError(t, err)
	assert.Equal(t, []StaffQualification{{Name: "Schwimmschein", ExpiresOn: "2027-01-01"}}, rows)
}

func TestNormalizeFinancialSection(t *testing.T) {
	t.Parallel()

	got, err := NormalizeFinancialSection(FinancialSection{
		IBAN: ptr(" de89 3704 0044 0532 0130 00 "), TaxID: ptr("12 345 678 901"), SocialSecurityNumber: ptr("12 010190 a 123"),
	})
	require.NoError(t, err)
	assert.Equal(t, "DE89370400440532013000", *got.IBAN)
	assert.Equal(t, "12345678901", *got.TaxID)
	assert.Equal(t, "12010190A123", *got.SocialSecurityNumber)

	got, err = NormalizeFinancialSection(FinancialSection{IBAN: ptr("  "), TaxID: nil})
	require.NoError(t, err)
	assert.Nil(t, got.IBAN, "blank clears the field")

	for name, in := range map[string]FinancialSection{
		"malformed IBAN":          {IBAN: ptr("DE00370400440532013000")},
		"Steuer-ID must be":       {TaxID: ptr("123")},
		"malformed SV-Nummer":     {SocialSecurityNumber: ptr("nope")},
		"IBAN with a bad pattern": {IBAN: ptr("12345")},
	} {
		_, err := NormalizeFinancialSection(in)
		require.ErrorIs(t, err, ErrStaffStammdatenInvalid, name)
	}
}

func TestFinancialChangesCarryMaskedValuesOnly(t *testing.T) {
	t.Parallel()

	stored := &StaffFinancialData{IBAN: ptr("DE89370400440532013000"), TaxID: ptr("12345678901")}
	changes := FinancialChanges(stored, FinancialSection{
		IBAN: ptr("DE02120300000000202051"), TaxID: ptr("10987654321"),
	})
	require.Len(t, changes, 2)
	for _, change := range changes {
		assert.NotContains(t, change.OldValue, "DE89", "audit rows must not become a second store of bank data")
		assert.NotContains(t, change.NewValue, "12345678901")
	}
	assert.Equal(t, "iban", changes[0].Field)
	assert.Equal(t, "•••• 3000", changes[0].OldValue)
	assert.Equal(t, "•••• 2051", changes[0].NewValue)
	assert.Equal(t, "tax_id", changes[1].Field)
	// Both masks are the fixed full mask, so the new value is suffixed to keep
	// the row a visible change.
	assert.Equal(t, "••••••••", changes[1].OldValue)
	assert.Equal(t, "•••••••• (geändert)", changes[1].NewValue)

	assert.Empty(t, FinancialChanges(stored, FinancialSection{IBAN: stored.IBAN, TaxID: stored.TaxID}))
}

func TestMaskFinancial(t *testing.T) {
	t.Parallel()

	const staffID int64 = 7
	masked := MaskFinancial(staffID, &StaffFinancialData{IBAN: ptr("DE89370400440532013000"), TaxID: ptr("12345678901"), SocialSecurityNumber: ptr("")})
	assert.Equal(t, staffID, masked.StaffID)
	assert.Equal(t, "•••• 3000", *masked.IBANMasked)
	assert.Equal(t, "••••••••", *masked.TaxIDMasked)
	assert.Nil(t, masked.SocialSecurityNumberMasked, "an empty value is not stored")

	short := MaskFinancial(7, &StaffFinancialData{IBAN: ptr("123")})
	assert.Equal(t, "•••", *short.IBANMasked, "a value no longer than the visible tail is fully masked")

	none := MaskFinancial(9, nil)
	assert.Equal(t, StaffFinancialMasked{StaffID: 9}, none)
}

func TestSectionChangesListOnlyWhatChanged(t *testing.T) {
	t.Parallel()

	subject := StaffSubject{FirstName: "Ada", LastName: "L", Birthday: "1990-04-12", EmploymentType: ptr(EmploymentTypeFullTime)}
	masterData := &StaffMasterData{Gender: ptr(GenderFemale), EntryDate: "2024-08-01", WeeklyHours: ptr(29.5)}

	person := PersonChanges(subject, masterData, PersonSection{
		FirstName: "Ada", LastName: "Lovelace", Birthday: ptr("1990-04-12"), Gender: ptr(GenderFemale),
	})
	assert.Equal(t, []FieldChange{{Field: "last_name", OldValue: "L", NewValue: "Lovelace"}}, person)

	assert.Empty(t, PersonChanges(subject, masterData, PersonSection{
		FirstName: "Ada", LastName: "L", Birthday: ptr("1990-04-12"), Gender: ptr(GenderFemale),
	}), "a no-op submit records nothing")

	contract := ContractChanges(subject, masterData, ContractSection{
		EntryDate: ptr("2024-08-01"), WeeklyHours: ptr(30.0), EmploymentType: ptr(EmploymentTypePartTime),
	})
	assert.Equal(t, []FieldChange{
		{Field: "weekly_hours", OldValue: "29.5", NewValue: "30"},
		{Field: "employment_type", OldValue: EmploymentTypeFullTime, NewValue: EmploymentTypePartTime},
	}, contract)

	// Before the first write there is no master data row: everything reads as unset.
	contact := ContactChanges(nil, ContactSection{Phone: ptr("0123")})
	assert.Equal(t, []FieldChange{{Field: "phone", OldValue: "", NewValue: "0123"}}, contact)
}

func TestQualificationChangesRecordTheListAsOneField(t *testing.T) {
	t.Parallel()

	existing := []StaffQualification{{Name: "Erste Hilfe", AcquiredOn: "2023-03-10", ExpiresOn: "2026-03-10"}, {Name: "Schwimmschein"}}
	submitted := []StaffQualification{{Name: "Schwimmschein"}}
	changes := QualificationChanges(existing, submitted)
	require.Len(t, changes, 1)
	assert.Equal(t, "qualifications", changes[0].Field)
	assert.Equal(t, "Erste Hilfe (erworben 2023-03-10) (bis 2026-03-10); Schwimmschein", changes[0].OldValue)
	assert.Equal(t, "Schwimmschein", changes[0].NewValue)
	assert.Empty(t, QualificationChanges(submitted, submitted))
}

func newAuthority() DocumentAuthority {
	return DocumentAuthority{
		Health: "health", Financial: "financial", General: "general",
		Allows: func(required string, held []string) bool {
			for _, permission := range held {
				if permission == required || permission == "admin:*" {
					return true
				}
			}
			return false
		},
	}
}

func TestDocumentAuthorityMapsEveryCategoryToOneTier(t *testing.T) {
	t.Parallel()

	authority := newAuthority()
	assert.Equal(t, "health", authority.Required(StaffDocumentCategoryAUBescheinigung))
	assert.Equal(t, "financial", authority.Required(StaffDocumentCategoryLohnabrechnung))
	for _, category := range []string{
		StaffDocumentCategoryArbeitsvertrag, StaffDocumentCategoryZeugnis,
		StaffDocumentCategoryBewerbung, StaffDocumentCategorySonstiges,
	} {
		assert.Equal(t, "general", authority.Required(category), category)
	}

	hr := DocumentActor{AccountID: 1, Permissions: []string{"general"}}
	assert.ElementsMatch(t, []string{
		StaffDocumentCategoryArbeitsvertrag, StaffDocumentCategoryZeugnis,
		StaffDocumentCategoryBewerbung, StaffDocumentCategorySonstiges,
	}, authority.Visible(hr))
	require.NoError(t, authority.Require(StaffDocumentCategoryZeugnis, hr))
	err := authority.Require(StaffDocumentCategoryAUBescheinigung, hr)
	require.ErrorIs(t, err, ErrStaffDocumentForbidden)
	assert.Equal(t, "staff document category not permitted: au_bescheinigung", err.Error())

	admin := DocumentActor{Permissions: []string{"admin:*"}}
	assert.Len(t, authority.Visible(admin), len(StaffDocumentCategories))
	assert.Empty(t, authority.Visible(DocumentActor{}))
}

func TestSensitiveStaffDocumentCategories(t *testing.T) {
	t.Parallel()

	assert.True(t, SensitiveStaffDocumentCategory(StaffDocumentCategoryAUBescheinigung))
	assert.True(t, SensitiveStaffDocumentCategory(StaffDocumentCategoryLohnabrechnung))
	assert.False(t, SensitiveStaffDocumentCategory(StaffDocumentCategoryZeugnis))
	assert.True(t, IsValidStaffDocumentCategory(StaffDocumentCategorySonstiges))
	assert.False(t, IsValidStaffDocumentCategory("geheim"))
}

func TestStaffDocumentRetentionSchedule(t *testing.T) {
	t.Parallel()

	doc := func(category string) StaffDocument { return StaffDocument{Category: category} }

	payslip := NewStaffDocumentInfo(doc(StaffDocumentCategoryLohnabrechnung), "2026-02-10", "")
	assert.Equal(t, "2036-02-10", payslip.RetainUntil, "payslips: 10 years")
	assert.Empty(t, payslip.ReviewDue)

	sickNote := NewStaffDocumentInfo(doc(StaffDocumentCategoryAUBescheinigung), "2024-02-29", "")
	assert.Equal(t, "2028-02-29", sickNote.RetainUntil, "sick notes: 4 years")

	shifted := NewStaffDocumentInfo(doc(StaffDocumentCategoryLohnabrechnung), "2024-02-29", "")
	assert.Equal(t, "2034-03-01", shifted.RetainUntil, "Feb 29 + 10y normalizes to Mar 1 like time.Date")

	open := NewStaffDocumentInfo(doc(StaffDocumentCategoryArbeitsvertrag), "2026-02-10", "")
	assert.Empty(t, open.RetainUntil, "an unbefristeter Vertrag keeps retention open")
	ended := NewStaffDocumentInfo(doc(StaffDocumentCategoryArbeitsvertrag), "2020-01-01", "2026-06-30")
	assert.Equal(t, "2032-06-30", ended.RetainUntil, "contract end + 6 years")

	application := NewStaffDocumentInfo(doc(StaffDocumentCategoryBewerbung), "2026-08-31", "")
	assert.Equal(t, "2027-03-03", application.ReviewDue, "applications: review after 6 months")
	assert.Empty(t, application.RetainUntil)

	for _, category := range []string{StaffDocumentCategoryZeugnis, StaffDocumentCategorySonstiges} {
		info := NewStaffDocumentInfo(doc(category), "2026-02-10", "2026-06-30")
		assert.Empty(t, info.RetainUntil, category)
		assert.Empty(t, info.ReviewDue, category)
	}
}

func TestStaffDocumentCleanupDelayOutlastsTheUploadDeadline(t *testing.T) {
	t.Parallel()

	assert.Greater(t, StaffDocumentCleanupDelay, StaffDocumentUploadDeadline,
		"a queued cleanup intent must never become eligible while its upload can still run")
}

func TestRecordNotFoundErrorKeepsTheRepositoryShape(t *testing.T) {
	t.Parallel()

	err := error(&RecordNotFoundError{Op: "find staff document"})
	assert.Equal(t, "database error during find staff document: repository: not found\nsql: no rows in result set", err.Error())
	assert.ErrorIs(t, err, ErrStaffDocumentNotFound)
	var marker interface{ RepositoryNotFound() }
	require.True(t, errors.As(err, &marker), "the HTTP layer recognizes a missing row by this marker")
}

func TestActorRoleOrUnknown(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "admin", ActorRoleOrUnknown("admin"))
	assert.Equal(t, "unknown", ActorRoleOrUnknown("  "))
	assert.Equal(t, "unknown", ActorRoleOrUnknown(""))
}
