package importpkg

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	importModels "github.com/moto-nrw/project-phoenix/models/import"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Opening balance import validation (#2132). These are pure unit tests: the
// preload caches are the only state Validate/ValidateBatch read, so the test
// sets them directly instead of going through PreloadReferenceData and a
// database.

// Fake staff IDs — in-memory map keys, never DB rows.
const (
	openingStaffAnnaID    = int64(9101)
	openingStaffBerndID   = int64(9102)
	openingStaffTwinAID   = int64(9103)
	openingStaffTwinBID   = int64(9104)
	openingDecidedByID    = int64(9105)
	openingEffectiveYear  = 2026
	openingEffectiveMonth = time.March
	openingEffectiveDay   = 1
)

func openingStaff(id int64, personnelNumber, firstName, lastName string) *userModels.Staff {
	staff := &userModels.Staff{
		Person: &userModels.Person{FirstName: firstName, LastName: lastName},
	}
	staff.ID = id
	if personnelNumber != "" {
		pn := personnelNumber
		staff.PersonnelNumber = &pn
	}
	return staff
}

// newOpeningConfig builds a config with the preload caches already populated:
// Anna (with Personalnummer), Bernd (name only), and two staff members sharing
// one name to exercise the ambiguity branch.
func newOpeningConfig() *OpeningBalanceImportConfig {
	anna := openingStaff(openingStaffAnnaID, "P-100", "Anna", "Lehmann")
	bernd := openingStaff(openingStaffBerndID, "", "Bernd", "Schulz")
	twinA := openingStaff(openingStaffTwinAID, "P-200", "Maria", "Müller")
	twinB := openingStaff(openingStaffTwinBID, "P-201", "Maria", "Müller")

	return &OpeningBalanceImportConfig{
		EffectiveDate:    timezone.NewDate(openingEffectiveYear, openingEffectiveMonth, openingEffectiveDay),
		Note:             "Übernahme aus Altsystem",
		DecidedByStaffID: openingDecidedByID,
		staffByPersonnelNumber: map[string]*userModels.Staff{
			"p-100": anna,
			"p-200": twinA,
			"p-201": twinB,
		},
		staffByName: map[string][]*userModels.Staff{
			staffNameKey("Anna", "Lehmann"): {anna},
			staffNameKey("Bernd", "Schulz"): {bernd},
			staffNameKey("Maria", "Müller"): {twinA, twinB},
		},
		staffWithHoursOpening: map[int64]bool{},
		staffWithVacOpening:   map[int64]bool{},
	}
}

// errorCodes reduces a validation result to its codes for readable assertions.
func errorCodes(errs []importModels.ValidationError) []string {
	codes := make([]string, 0, len(errs))
	for _, e := range errs {
		codes = append(codes, e.Code)
	}
	return codes
}

func TestParseGermanDecimal(t *testing.T) {
	tests := []struct {
		raw     string
		want    float64
		wantErr bool
	}{
		{raw: "12,5", want: 12.5},
		{raw: "-3,25", want: -3.25},
		{raw: "7.5", want: 7.5},
		{raw: "0", want: 0},
		{raw: " 12,5 ", want: 12.5},
		{raw: "abc", wantErr: true},
		{raw: "", wantErr: true},
		{raw: "NaN", wantErr: true},
		{raw: "+Inf", wantErr: true},
		{raw: "-Inf", wantErr: true},
		// The German thousands separator is NOT supported — "1.234,5" would
		// silently become an invalid float rather than 1234,5.
		{raw: "1.234,5", wantErr: true},
	}

	for _, tc := range tests {
		got, err := parseGermanDecimal(tc.raw)
		if tc.wantErr {
			assert.Error(t, err, "raw %q must not parse", tc.raw)
			continue
		}
		require.NoError(t, err, "raw %q", tc.raw)
		assert.InDelta(t, tc.want, got, 0.0001, "raw %q", tc.raw)
	}
}

func TestOpeningBalanceValidate_RowWithoutValues(t *testing.T) {
	c := newOpeningConfig()
	row := &importModels.OpeningBalanceImportRow{FirstName: "Anna", LastName: "Lehmann"}

	errs := c.Validate(context.Background(), row)

	assert.Contains(t, errorCodes(errs), "no_values")
	assert.Equal(t, openingStaffAnnaID, row.StaffID, "the person still resolves")
}

func TestOpeningBalanceValidate_UnknownPersonnelNumber(t *testing.T) {
	c := newOpeningConfig()
	row := &importModels.OpeningBalanceImportRow{
		PersonnelNumber: "P-999",
		FirstName:       "Anna",
		LastName:        "Lehmann",
		HoursBalance:    "5",
	}

	errs := c.Validate(context.Background(), row)

	assert.Contains(t, errorCodes(errs), "personnel_number_not_found")
	assert.Zero(t, row.StaffID,
		"an unknown Personalnummer must not fall back to the name match")
}

func TestOpeningBalanceValidate_UnknownName(t *testing.T) {
	c := newOpeningConfig()
	row := &importModels.OpeningBalanceImportRow{
		FirstName:    "Klaus",
		LastName:     "Unbekannt",
		HoursBalance: "5",
	}

	errs := c.Validate(context.Background(), row)

	assert.Contains(t, errorCodes(errs), "staff_not_found")
	assert.Zero(t, row.StaffID)
}

func TestOpeningBalanceValidate_AmbiguousName(t *testing.T) {
	c := newOpeningConfig()
	row := &importModels.OpeningBalanceImportRow{
		FirstName:    "Maria",
		LastName:     "Müller",
		HoursBalance: "5",
	}

	errs := c.Validate(context.Background(), row)

	assert.Contains(t, errorCodes(errs), "staff_ambiguous")
	assert.Zero(t, row.StaffID, "an ambiguous match must not pick a winner")
}

func TestOpeningBalanceValidate_MissingNameWithoutPersonnelNumber(t *testing.T) {
	c := newOpeningConfig()
	row := &importModels.OpeningBalanceImportRow{LastName: "Lehmann", HoursBalance: "5"}

	errs := c.Validate(context.Background(), row)

	assert.Contains(t, errorCodes(errs), "required")
}

func TestOpeningBalanceValidate_PersonnelNumberMatchIsCaseInsensitive(t *testing.T) {
	c := newOpeningConfig()
	row := &importModels.OpeningBalanceImportRow{PersonnelNumber: " p-100 ", HoursBalance: "1"}

	errs := c.Validate(context.Background(), row)

	assert.Empty(t, errs)
	assert.Equal(t, openingStaffAnnaID, row.StaffID)
}

func TestOpeningBalanceValidate_NegativeHoursBalanceBecomesMinutes(t *testing.T) {
	c := newOpeningConfig()
	row := &importModels.OpeningBalanceImportRow{
		FirstName:    "Anna",
		LastName:     "Lehmann",
		HoursBalance: "-3,25",
	}

	errs := c.Validate(context.Background(), row)

	require.Empty(t, errs)
	require.NotNil(t, row.HoursBalanceMinutes)
	// −3,25 h = −195 min. A negative opening balance is the whole point of
	// the takeover: staff members arrive with a minus on the Stundenkonto.
	assert.Equal(t, -195, *row.HoursBalanceMinutes)
}

func TestOpeningBalanceValidate_HoursBalanceRoundsToWholeMinutes(t *testing.T) {
	c := newOpeningConfig()
	row := &importModels.OpeningBalanceImportRow{
		FirstName:    "Anna",
		LastName:     "Lehmann",
		HoursBalance: "1,505", // 90,3 min
	}

	errs := c.Validate(context.Background(), row)

	require.Empty(t, errs)
	require.NotNil(t, row.HoursBalanceMinutes)
	assert.Equal(t, 90, *row.HoursBalanceMinutes)
}

func TestOpeningBalanceValidate_InvalidNumber(t *testing.T) {
	c := newOpeningConfig()
	row := &importModels.OpeningBalanceImportRow{
		FirstName:    "Anna",
		LastName:     "Lehmann",
		HoursBalance: "acht",
	}

	errs := c.Validate(context.Background(), row)

	require.Len(t, errs, 1)
	assert.Equal(t, "invalid_number", errs[0].Code)
	assert.Equal(t, "hours_balance", errs[0].Field)
	assert.Equal(t, "acht", errs[0].ActualValue)
	assert.Nil(t, row.HoursBalanceMinutes)
}

func TestOpeningBalanceValidate_OutOfRangeValues(t *testing.T) {
	tests := []struct {
		name  string
		row   importModels.OpeningBalanceImportRow
		field string
		code  string
	}{
		{
			name:  "Jahresanspruch über 366 Tagen",
			row:   importModels.OpeningBalanceImportRow{VacationEntitled: "400"},
			field: "vacation_entitled",
			code:  "out_of_range",
		},
		{
			name:  "negativer Jahresanspruch",
			row:   importModels.OpeningBalanceImportRow{VacationEntitled: "-1"},
			field: "vacation_entitled",
			code:  "out_of_range",
		},
		{
			name:  "negativer Vorjahresübertrag",
			row:   importModels.OpeningBalanceImportRow{VacationCarryover: "-1"},
			field: "vacation_carryover",
			code:  "out_of_range",
		},
		{
			name:  "Stundensaldo über der Ledger-Grenze",
			row:   importModels.OpeningBalanceImportRow{HoursBalance: "10001"},
			field: "hours_balance",
			code:  "out_of_range",
		},
		{
			name:  "Resturlaub außerhalb ±999",
			row:   importModels.OpeningBalanceImportRow{VacationRemaining: "1000"},
			field: "vacation_remaining",
			code:  "out_of_range",
		},
		{
			name:  "Resturlaub mit mehr als einer Nachkommastelle",
			row:   importModels.OpeningBalanceImportRow{VacationRemaining: "12,25"},
			field: "vacation_remaining",
			code:  "invalid_precision",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := newOpeningConfig()
			row := tc.row
			row.FirstName = "Anna"
			row.LastName = "Lehmann"

			errs := c.Validate(context.Background(), &row)

			require.Len(t, errs, 1)
			assert.Equal(t, tc.code, errs[0].Code)
			assert.Equal(t, tc.field, errs[0].Field)
		})
	}
}

func TestOpeningBalanceValidate_NegativeRemainingVacationIsAllowed(t *testing.T) {
	c := newOpeningConfig()
	row := &importModels.OpeningBalanceImportRow{
		FirstName:         "Anna",
		LastName:          "Lehmann",
		VacationRemaining: "-2",
	}

	errs := c.Validate(context.Background(), row)

	require.Empty(t, errs, "an overdrawn vacation account must be importable")
	require.NotNil(t, row.VacationRemainingDays)
	assert.InDelta(t, -2, *row.VacationRemainingDays, 0.0001)
}

func TestOpeningBalanceValidate_ExistingHoursOpening(t *testing.T) {
	c := newOpeningConfig()
	c.staffWithHoursOpening[openingStaffAnnaID] = true
	row := &importModels.OpeningBalanceImportRow{
		FirstName:    "Anna",
		LastName:     "Lehmann",
		HoursBalance: "5",
	}

	errs := c.Validate(context.Background(), row)

	assert.Contains(t, errorCodes(errs), "hours_opening_exists")
}

func TestOpeningBalanceValidate_ExistingHoursOpeningIgnoredWithoutHoursColumn(t *testing.T) {
	c := newOpeningConfig()
	c.staffWithHoursOpening[openingStaffAnnaID] = true
	// Vacation-only row for a person whose Stundenkonto was taken over
	// earlier: the second side must still be importable.
	row := &importModels.OpeningBalanceImportRow{
		FirstName:         "Anna",
		LastName:          "Lehmann",
		VacationRemaining: "12,5",
	}

	errs := c.Validate(context.Background(), row)

	assert.Empty(t, errs)
}

func TestOpeningBalanceValidate_ExistingVacationOpening(t *testing.T) {
	c := newOpeningConfig()
	c.staffWithVacOpening[openingStaffAnnaID] = true
	row := &importModels.OpeningBalanceImportRow{
		FirstName:         "Anna",
		LastName:          "Lehmann",
		VacationRemaining: "12,5",
	}

	errs := c.Validate(context.Background(), row)

	require.Len(t, errs, 1)
	assert.Equal(t, "vacation_opening_exists", errs[0].Code)
	assert.Contains(t, errs[0].Message, "2026", "the message names the affected year")
}

func TestOpeningBalanceValidate_AcceptsCompleteRow(t *testing.T) {
	c := newOpeningConfig()
	row := &importModels.OpeningBalanceImportRow{
		PersonnelNumber:   "P-100",
		FirstName:         "Anna",
		LastName:          "Lehmann",
		HoursBalance:      "12,5",
		VacationEntitled:  "30",
		VacationCarryover: "2",
		VacationRemaining: "17,5",
	}

	errs := c.Validate(context.Background(), row)

	require.Empty(t, errs)
	assert.Equal(t, openingStaffAnnaID, row.StaffID)
	require.NotNil(t, row.HoursBalanceMinutes)
	assert.Equal(t, 750, *row.HoursBalanceMinutes)
	require.NotNil(t, row.VacationEntitledDays)
	assert.InDelta(t, 30, *row.VacationEntitledDays, 0.0001)
	require.NotNil(t, row.VacationCarryoverDays)
	assert.InDelta(t, 2, *row.VacationCarryoverDays, 0.0001)
	require.NotNil(t, row.VacationRemainingDays)
	assert.InDelta(t, 17.5, *row.VacationRemainingDays, 0.0001)
}

func TestOpeningBalanceValidateBatch_DuplicateStaffInFile(t *testing.T) {
	c := newOpeningConfig()
	rows := []importModels.OpeningBalanceImportRow{
		{FirstName: "Anna", LastName: "Lehmann"},
		{FirstName: "Bernd", LastName: "Schulz"},
		{FirstName: " Anna ", LastName: " Lehmann "},
	}

	errs := c.ValidateBatch(context.Background(), rows)

	require.Len(t, errs, 1, "only the second occurrence is flagged")
	require.Contains(t, errs, 2)
	assert.Equal(t, "duplicate_in_file", errs[2][0].Code)
	// Row index 0 is spreadsheet line 2.
	assert.Contains(t, errs[2][0].Message, "erste Zeile: 2")
}

func TestOpeningBalanceValidateBatch_IgnoresUnresolvedRows(t *testing.T) {
	c := newOpeningConfig()
	rows := []importModels.OpeningBalanceImportRow{
		{FirstName: "Klaus", LastName: "Unbekannt"},
		{FirstName: "Klaus", LastName: "Unbekannt"},
	}

	errs := c.ValidateBatch(context.Background(), rows)

	assert.Empty(t, errs, "rows without a resolved person already carry their own error")
}

func TestOpeningBalanceConfig_FindExistingAlwaysNew(t *testing.T) {
	c := newOpeningConfig()

	existing, err := c.FindExisting(context.Background(), importModels.OpeningBalanceImportRow{StaffID: openingStaffAnnaID})

	require.NoError(t, err)
	assert.Nil(t, existing, "duplicates are reported per side in Validate, not here")
}

func TestOpeningBalanceConfig_UpdateIsNoOp(t *testing.T) {
	c := newOpeningConfig()

	require.NoError(t, c.Update(context.Background(), openingStaffAnnaID, importModels.OpeningBalanceImportRow{}))
	assert.Equal(t, "Eröffnungssaldo", c.EntityName())
}

func TestOpeningBalanceConfig_CreateRejectsUnresolvedRow(t *testing.T) {
	c := newOpeningConfig()

	_, err := c.Create(context.Background(), importModels.OpeningBalanceImportRow{})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "ohne aufgelöste Person")
}
