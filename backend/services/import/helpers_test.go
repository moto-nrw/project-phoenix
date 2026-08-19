package importpkg

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNormalizeHeaderKey tests header key normalization with annotation stripping
func TestNormalizeHeaderKey(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected string
	}{
		{"Vorname", "vorname"},
		{"Erz1.Abholberechtigt (optional)", "erz1.abholberechtigt"},
		{"Erz1.Hauptansprechpartner (optional)", "erz1.hauptansprechpartner"},
		{"Datenschutz (pflicht)", "datenschutz"},
		{"  Klasse  ", "klasse"},
		{"Aufbewahrung(Tage) (optional)", "aufbewahrung(tage)"},
		{"Gruppe (optional)", "gruppe"},
		{"Nachname", "nachname"},
		// Backward-compatible aliases for renamed columns
		{"Erz1.Primär", "erz1.hauptansprechpartner"},
		{"Erz2.Primär", "erz2.hauptansprechpartner"},
		{"Erz3.Primär", "erz3.hauptansprechpartner"},
		{"Erz1.Abholung", "erz1.abholberechtigt"},
		{"Erz2.Abholung", "erz2.abholberechtigt"},
		{"Erz3.Abholung", "erz3.abholberechtigt"},
		// Aliases with annotations also work
		{"Erz1.Primär (optional)", "erz1.hauptansprechpartner"},
		{"Erz1.Abholung (optional)", "erz1.abholberechtigt"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := normalizeHeaderKey(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestSanitizeCellValue tests CSV injection protection
func TestSanitizeCellValue(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "safe value unchanged",
			input:    "Normal Text",
			expected: "Normal Text",
		},
		{
			name:     "empty string unchanged",
			input:    "",
			expected: "",
		},
		{
			name:     "formula starting with = is prefixed",
			input:    "=SUM(A1:A10)",
			expected: "'=SUM(A1:A10)",
		},
		{
			name:     "formula starting with + is prefixed",
			input:    "+1+2",
			expected: "'+1+2",
		},
		{
			name:     "formula starting with - is prefixed",
			input:    "-A1",
			expected: "'-A1",
		},
		{
			name:     "formula starting with @ is prefixed",
			input:    "@SUM(A1)",
			expected: "'@SUM(A1)",
		},
		{
			name:     "value starting with tab is prefixed",
			input:    "\tTabStart",
			expected: "'\tTabStart",
		},
		{
			name:     "value starting with carriage return is prefixed",
			input:    "\rCRStart",
			expected: "'\rCRStart",
		},
		{
			name:     "safe value with special char in middle",
			input:    "A=B",
			expected: "A=B",
		},
		{
			name:     "safe value starting with letter",
			input:    "abc123",
			expected: "abc123",
		},
		{
			name:     "dangerous cmd injection attempt",
			input:    "=cmd|'/c calc'!A1",
			expected: "'=cmd|'/c calc'!A1",
		},
		{
			name:     "DDE attack attempt",
			input:    "=DDE(\"cmd\";\"calc\")",
			expected: "'=DDE(\"cmd\";\"calc\")",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeCellValue(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestColumnMapper tests the ColumnMapper functionality
func TestColumnMapper(t *testing.T) {
	t.Parallel()

	t.Run("GetCol returns sanitized value", func(t *testing.T) {
		mapping := map[string]int{"name": 0, "formula": 1}
		values := []string{"John Doe", "=EVIL()"}
		mapper := NewColumnMapper(mapping, values)

		assert.Equal(t, "John Doe", mapper.GetCol("name"))
		assert.Equal(t, "'=EVIL()", mapper.GetCol("formula"))
	})

	t.Run("GetCol returns empty for non-existent column", func(t *testing.T) {
		mapping := map[string]int{"name": 0}
		values := []string{"John Doe"}
		mapper := NewColumnMapper(mapping, values)

		assert.Equal(t, "", mapper.GetCol("nonexistent"))
	})

	t.Run("GetCol returns empty for out-of-range index", func(t *testing.T) {
		mapping := map[string]int{"name": 5}
		values := []string{"John Doe"}
		mapper := NewColumnMapper(mapping, values)

		assert.Equal(t, "", mapper.GetCol("name"))
	})

	t.Run("GetCol trims whitespace", func(t *testing.T) {
		mapping := map[string]int{"name": 0}
		values := []string{"  John Doe  "}
		mapper := NewColumnMapper(mapping, values)

		assert.Equal(t, "John Doe", mapper.GetCol("name"))
	})

	t.Run("GetRawCol preserves phone numbers with +", func(t *testing.T) {
		mapping := map[string]int{"phone": 0}
		values := []string{"+49123456789"}
		mapper := NewColumnMapper(mapping, values)

		assert.Equal(t, "+49123456789", mapper.GetRawCol("phone"))
	})

	t.Run("GetRawCol returns empty for non-existent column", func(t *testing.T) {
		mapping := map[string]int{"phone": 0}
		values := []string{"+49123456789"}
		mapper := NewColumnMapper(mapping, values)

		assert.Equal(t, "", mapper.GetRawCol("nonexistent"))
	})

	t.Run("HasColumn returns true for existing column", func(t *testing.T) {
		mapping := map[string]int{"name": 0, "phone": 1}
		values := []string{"John", "+49123"}
		mapper := NewColumnMapper(mapping, values)

		assert.True(t, mapper.HasColumn("name"))
		assert.True(t, mapper.HasColumn("phone"))
	})

	t.Run("HasColumn returns false for non-existent column", func(t *testing.T) {
		mapping := map[string]int{"name": 0}
		values := []string{"John"}
		mapper := NewColumnMapper(mapping, values)

		assert.False(t, mapper.HasColumn("email"))
	})
}

// TestParseBool tests German boolean parsing
func TestParseBool(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"German yes", "Ja", true},
		{"German yes lowercase", "ja", true},
		{"German yes uppercase", "JA", true},
		{"English yes", "Yes", true},
		{"English yes lowercase", "yes", true},
		{"English yes uppercase", "YES", true},
		{"Boolean true", "true", true},
		{"Boolean True", "True", true},
		{"Boolean TRUE", "TRUE", true},
		{"Number 1", "1", true},
		{"German no", "Nein", false},
		{"German no lowercase", "nein", false},
		{"English no", "No", false},
		{"Boolean false", "false", false},
		{"Number 0", "0", false},
		{"Empty string", "", false},
		{"Random text", "maybe", false},
		{"Whitespace yes", "  ja  ", true},
		{"Whitespace no", "  nein  ", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ParseBool(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// TestMapStudentRow tests the student row mapping
func TestMapStudentRow(t *testing.T) {
	t.Parallel()

	t.Run("maps basic student fields", func(t *testing.T) {
		mapping := map[string]int{
			"vorname":     0,
			"nachname":    1,
			"klasse":      2,
			"gruppe":      3,
			"geburtstag":  4,
			"rfid":        5,
			"datenschutz": 6,
		}
		values := []string{
			"Max",
			"Mustermann",
			"1a",
			"Gruppe A",
			"2015-01-15",
			"ABC123",
			"Ja",
		}
		mapper := NewColumnMapper(mapping, values)

		row, err := MapStudentRow(mapper)

		require.NoError(t, err)
		assert.Equal(t, "Max", row.FirstName)
		assert.Equal(t, "Mustermann", row.LastName)
		assert.Equal(t, "1a", row.SchoolClass)
		assert.Equal(t, "Gruppe A", row.GroupName)
		assert.Equal(t, "2015-01-15", row.Birthday)
		assert.Equal(t, "ABC123", row.TagID)
		assert.True(t, row.PrivacyAccepted)
		assert.Equal(t, 30, row.DataRetentionDays)
	})

	t.Run("maps optional fields", func(t *testing.T) {
		mapping := map[string]int{
			"vorname":                         0,
			"nachname":                        1,
			"gesundheitsinfo":                 2,
			"betreuernotizen":                 3,
			"zusatzinfo":                      4,
			"abholstatus":                     5,
			"bus":                             6,
			"aufbewahrung(tage)":              7,
			"einschreibung von":               8,
			"einschreibung bis":               9,
			"agb akzeptiert am":               10,
			"datenverarbeitung akzeptiert am": 11,
			"e-mail-kontakt akzeptiert am":    12,
			"foto-einwilligung am":            13,
		}
		values := []string{
			"Anna",
			"Schmidt",
			"Allergies: None",
			"Notes here",
			"Extra info",
			"Authorized",
			"ja",
			"7",
			"2026-08-01",
			"2027-07-31",
			"2026-05-10",
			"2026-05-11",
			"2026-05-12",
			"2026-05-13",
		}
		mapper := NewColumnMapper(mapping, values)

		row, err := MapStudentRow(mapper)

		require.NoError(t, err)
		assert.Equal(t, "Allergies: None", row.HealthInfo)
		assert.Equal(t, "Notes here", row.SupervisorNotes)
		assert.Equal(t, "Extra info", row.ExtraInfo)
		assert.Equal(t, "Authorized", row.PickupStatus)
		assert.True(t, row.BusPermission)
		assert.Equal(t, 7, row.DataRetentionDays)
		assert.Equal(t, "2026-08-01", row.EnrolledFrom)
		assert.Equal(t, "2027-07-31", row.EnrolledUntil)
		assert.Equal(t, "2026-05-10", row.AGBAcceptedAt)
		assert.Equal(t, "2026-05-11", row.DataProcessingAcceptedAt)
		assert.Equal(t, "2026-05-12", row.EmailContactAcceptedAt)
		assert.Equal(t, "2026-05-13", row.PhotoConsentGivenAt)
	})

	t.Run("returns error for invalid retention days", func(t *testing.T) {
		mapping := map[string]int{
			"vorname":            0,
			"nachname":           1,
			"aufbewahrung(tage)": 2,
		}
		values := []string{
			"Max",
			"Mustermann",
			"invalid",
		}
		mapper := NewColumnMapper(mapping, values)

		_, err := MapStudentRow(mapper)

		require.Error(t, err)
		assert.Contains(t, err.Error(), "ungültiger Wert für Aufbewahrung(Tage)")
	})

	t.Run("maps single guardian", func(t *testing.T) {
		mapping := map[string]int{
			"vorname":                   0,
			"nachname":                  1,
			"erz1.vorname":              2,
			"erz1.nachname":             3,
			"erz1.email":                4,
			"erz1.telefon":              5,
			"erz1.mobil":                6,
			"erz1.verhältnis":           7,
			"erz1.hauptansprechpartner": 8,
			"erz1.notfall":              9,
			"erz1.abholberechtigt":      10,
		}
		values := []string{
			"Max",
			"Mustermann",
			"Peter",
			"Mustermann",
			"peter@example.com",
			"+4912345",
			"+4967890",
			"Vater",
			"Ja",
			"Ja",
			"Ja",
		}
		mapper := NewColumnMapper(mapping, values)

		row, err := MapStudentRow(mapper)

		require.NoError(t, err)
		require.Len(t, row.Guardians, 1)

		guardian := row.Guardians[0]
		assert.Equal(t, "Peter", guardian.FirstName)
		assert.Equal(t, "Mustermann", guardian.LastName)
		assert.Equal(t, "peter@example.com", guardian.Email)
		assert.Equal(t, "+4912345", guardian.Phone)
		assert.Equal(t, "+4967890", guardian.MobilePhone)
		assert.Equal(t, "Vater", guardian.RelationshipType)
		assert.True(t, guardian.IsPrimary)
		assert.True(t, guardian.IsEmergencyContact)
		assert.True(t, guardian.CanPickup)
	})

	t.Run("maps multiple guardians", func(t *testing.T) {
		mapping := map[string]int{
			"vorname":      0,
			"nachname":     1,
			"erz1.email":   2,
			"erz1.telefon": 3,
			"erz2.email":   4,
			"erz2.telefon": 5,
			"erz3.email":   6,
		}
		values := []string{
			"Max",
			"Mustermann",
			"erz1@example.com",
			"+111",
			"erz2@example.com",
			"+222",
			"erz3@example.com",
		}
		mapper := NewColumnMapper(mapping, values)

		row, err := MapStudentRow(mapper)

		require.NoError(t, err)
		assert.Len(t, row.Guardians, 3)
		assert.Equal(t, "erz1@example.com", row.Guardians[0].Email)
		assert.Equal(t, "erz2@example.com", row.Guardians[1].Email)
		assert.Equal(t, "erz3@example.com", row.Guardians[2].Email)
	})

	t.Run("skips empty guardians", func(t *testing.T) {
		mapping := map[string]int{
			"vorname":      0,
			"nachname":     1,
			"erz1.email":   2,
			"erz2.telefon": 3,
			"erz2.email":   4,
		}
		values := []string{
			"Max",
			"Mustermann",
			"erz1@example.com",
			"", // Empty email for erz2
			"", // Empty phone for erz2
		}
		mapper := NewColumnMapper(mapping, values)

		row, err := MapStudentRow(mapper)

		require.NoError(t, err)
		assert.Len(t, row.Guardians, 1, "should skip guardian with no contact info")
	})
}

// TestDefaultPhoneMappings tests the default phone mapping configuration
func TestDefaultPhoneMappings(t *testing.T) {
	t.Parallel()

	mappings := DefaultPhoneMappings()

	assert.Len(t, mappings, 6)

	// Check expected mappings exist
	expectedMappings := map[string]struct {
		phoneType string
		label     string
	}{
		"telefon":     {"home", ""},
		"telefon2":    {"home", ""},
		"mobil":       {"mobile", ""},
		"mobil2":      {"mobile", ""},
		"dienstlich":  {"work", "Dienstlich"},
		"dienstlich2": {"work", "Dienstlich"},
	}

	for _, mapping := range mappings {
		expected, found := expectedMappings[mapping.Suffix]
		assert.True(t, found, "unexpected suffix: %s", mapping.Suffix)
		assert.Equal(t, expected.phoneType, mapping.PhoneType)
		assert.Equal(t, expected.label, mapping.Label)
	}
}

// TestParseGuardianPhoneNumbers tests phone number extraction
func TestParseGuardianPhoneNumbers(t *testing.T) {
	t.Parallel()

	t.Run("parses multiple phone types", func(t *testing.T) {
		getCol := func(key string) string {
			phones := map[string]string{
				"erz1.telefon":    "+49111",
				"erz1.mobil":      "+49222",
				"erz1.dienstlich": "+49333",
			}
			return phones[key]
		}

		phones := ParseGuardianPhoneNumbers(1, getCol)

		require.Len(t, phones, 3)

		// First phone should be primary
		assert.Equal(t, "+49111", phones[0].PhoneNumber)
		assert.Equal(t, "home", phones[0].PhoneType)
		assert.True(t, phones[0].IsPrimary)

		assert.Equal(t, "+49222", phones[1].PhoneNumber)
		assert.Equal(t, "mobile", phones[1].PhoneType)
		assert.False(t, phones[1].IsPrimary)

		assert.Equal(t, "+49333", phones[2].PhoneNumber)
		assert.Equal(t, "work", phones[2].PhoneType)
		assert.Equal(t, "Dienstlich", phones[2].Label)
		assert.False(t, phones[2].IsPrimary)
	})

	t.Run("skips empty phone numbers", func(t *testing.T) {
		getCol := func(key string) string {
			phones := map[string]string{
				"erz1.telefon":  "+49111",
				"erz1.telefon2": "", // Empty
				"erz1.mobil":    "+49222",
			}
			return phones[key]
		}

		phones := ParseGuardianPhoneNumbers(1, getCol)

		require.Len(t, phones, 2)
		assert.Equal(t, "+49111", phones[0].PhoneNumber)
		assert.Equal(t, "+49222", phones[1].PhoneNumber)
	})

	t.Run("handles guardian number in column keys", func(t *testing.T) {
		getCol := func(key string) string {
			phones := map[string]string{
				"erz2.telefon": "+49999",
			}
			return phones[key]
		}

		phones := ParseGuardianPhoneNumbers(2, getCol)

		require.Len(t, phones, 1)
		assert.Equal(t, "+49999", phones[0].PhoneNumber)
	})

	t.Run("returns empty slice when no phones", func(t *testing.T) {
		getCol := func(key string) string {
			return ""
		}

		phones := ParseGuardianPhoneNumbers(1, getCol)

		assert.Empty(t, phones)
	})

	t.Run("maintains priority order", func(t *testing.T) {
		getCol := func(key string) string {
			phones := map[string]string{
				"erz1.telefon":  "+49111",
				"erz1.telefon2": "+49112",
				"erz1.mobil":    "+49222",
				"erz1.mobil2":   "+49223",
			}
			return phones[key]
		}

		phones := ParseGuardianPhoneNumbers(1, getCol)

		require.Len(t, phones, 4)

		// Check priority
		for i, phone := range phones {
			if i == 0 {
				assert.True(t, phone.IsPrimary, "first phone should be primary")
			} else {
				assert.False(t, phone.IsPrimary, "subsequent phones should not be primary")
			}
		}
	})
}

// TestMapStudentRow_GuardianProfileFields tests mapping of new guardian profile fields
func TestMapStudentRow_GuardianProfileFields(t *testing.T) {
	t.Parallel()

	mapping := map[string]int{
		"vorname":      0,
		"nachname":     1,
		"erz1.email":   2,
		"erz1.straße":  3,
		"erz1.stadt":   4,
		"erz1.plz":     5,
		"erz1.notizen": 6,
		"erz1.sprache": 7,
	}
	values := []string{
		"Max", "Mustermann",
		"guardian@example.com",
		"Musterstr. 1", "Köln", "50667",
		"Allergien beachten", "de",
	}
	mapper := NewColumnMapper(mapping, values)

	row, err := MapStudentRow(mapper)

	require.NoError(t, err)
	require.Len(t, row.Guardians, 1)

	g := row.Guardians[0]
	assert.Equal(t, "Musterstr. 1", g.AddressStreet)
	assert.Equal(t, "Köln", g.AddressCity)
	assert.Equal(t, "50667", g.AddressPostalCode)
	assert.Equal(t, "Allergien beachten", g.Notes)
	assert.Equal(t, "de", g.LanguagePreference)
}

// TestMapStudentRow_PickupSchedules tests mapping of pickup schedule columns
func TestMapStudentRow_PickupSchedules(t *testing.T) {
	t.Parallel()

	t.Run("maps all five weekdays", func(t *testing.T) {
		mapping := map[string]int{
			"vorname":             0,
			"nachname":            1,
			"abholung.mo":         2,
			"abholung.mo.notizen": 3,
			"abholung.di":         4,
			"abholung.di.notizen": 5,
			"abholung.mi":         6,
			"abholung.mi.notizen": 7,
			"abholung.do":         8,
			"abholung.do.notizen": 9,
			"abholung.fr":         10,
			"abholung.fr.notizen": 11,
		}
		values := []string{
			"Max", "Mustermann",
			"16:00", "Hort",
			"15:30", "",
			"16:00", "",
			"15:30", "",
			"14:00", "Frühschluss",
		}
		mapper := NewColumnMapper(mapping, values)

		row, err := MapStudentRow(mapper)

		require.NoError(t, err)
		require.Len(t, row.PickupSchedules, 5)

		assert.Equal(t, 1, row.PickupSchedules[0].Weekday)
		assert.Equal(t, "16:00", row.PickupSchedules[0].PickupTime)
		assert.Equal(t, "Hort", row.PickupSchedules[0].Notes)

		assert.Equal(t, 5, row.PickupSchedules[4].Weekday)
		assert.Equal(t, "14:00", row.PickupSchedules[4].PickupTime)
		assert.Equal(t, "Frühschluss", row.PickupSchedules[4].Notes)
	})

	t.Run("skips empty days", func(t *testing.T) {
		mapping := map[string]int{
			"vorname":     0,
			"nachname":    1,
			"abholung.mo": 2,
			"abholung.di": 3,
			"abholung.mi": 4,
		}
		values := []string{
			"Max", "Mustermann",
			"16:00", "", "15:00",
		}
		mapper := NewColumnMapper(mapping, values)

		row, err := MapStudentRow(mapper)

		require.NoError(t, err)
		require.Len(t, row.PickupSchedules, 2)
		assert.Equal(t, 1, row.PickupSchedules[0].Weekday)
		assert.Equal(t, 3, row.PickupSchedules[1].Weekday)
	})

	t.Run("no pickup columns returns empty", func(t *testing.T) {
		mapping := map[string]int{
			"vorname":  0,
			"nachname": 1,
		}
		values := []string{"Max", "Mustermann"}
		mapper := NewColumnMapper(mapping, values)

		row, err := MapStudentRow(mapper)

		require.NoError(t, err)
		assert.Empty(t, row.PickupSchedules)
	})
}

func TestMapStudentRow_ArrivalSchedules(t *testing.T) {
	t.Parallel()

	t.Run("maps all five weekdays", func(t *testing.T) {
		mapping := map[string]int{
			"vorname":            0,
			"nachname":           1,
			"ankunft.mo":         2,
			"ankunft.mo.notizen": 3,
			"ankunft.di":         4,
			"ankunft.di.notizen": 5,
			"ankunft.mi":         6,
			"ankunft.mi.notizen": 7,
			"ankunft.do":         8,
			"ankunft.do.notizen": 9,
			"ankunft.fr":         10,
			"ankunft.fr.notizen": 11,
		}
		values := []string{
			"Max", "Mustermann",
			"08:00", "Frühdienst",
			"08:10", "",
			"08:00", "",
			"08:15", "",
			"08:30", "Später Start",
		}
		mapper := NewColumnMapper(mapping, values)

		row, err := MapStudentRow(mapper)

		require.NoError(t, err)
		require.Len(t, row.ArrivalSchedules, 5)

		assert.Equal(t, 1, row.ArrivalSchedules[0].Weekday)
		assert.Equal(t, "08:00", row.ArrivalSchedules[0].ExpectedArrival)
		assert.Equal(t, "Frühdienst", row.ArrivalSchedules[0].Notes)
		assert.Equal(t, 5, row.ArrivalSchedules[4].Weekday)
		assert.Equal(t, "08:30", row.ArrivalSchedules[4].ExpectedArrival)
		assert.Equal(t, "Später Start", row.ArrivalSchedules[4].Notes)
	})

	t.Run("skips empty days", func(t *testing.T) {
		mapping := map[string]int{
			"vorname":    0,
			"nachname":   1,
			"ankunft.mo": 2,
			"ankunft.di": 3,
			"ankunft.mi": 4,
		}
		values := []string{
			"Max", "Mustermann",
			"08:00", "", "08:15",
		}
		mapper := NewColumnMapper(mapping, values)

		row, err := MapStudentRow(mapper)

		require.NoError(t, err)
		require.Len(t, row.ArrivalSchedules, 2)
		assert.Equal(t, 1, row.ArrivalSchedules[0].Weekday)
		assert.Equal(t, 3, row.ArrivalSchedules[1].Weekday)
	})

	t.Run("no arrival columns returns empty", func(t *testing.T) {
		mapping := map[string]int{
			"vorname":  0,
			"nachname": 1,
		}
		values := []string{"Max", "Mustermann"}
		mapper := NewColumnMapper(mapping, values)

		row, err := MapStudentRow(mapper)

		require.NoError(t, err)
		assert.Empty(t, row.ArrivalSchedules)
	})
}

// TestMapStudentRow_GuardianProfileFieldsEmpty tests that empty guardian profile fields are mapped as empty strings
func TestMapStudentRow_GuardianProfileFieldsEmpty(t *testing.T) {
	t.Parallel()

	mapping := map[string]int{
		"vorname":      0,
		"nachname":     1,
		"erz1.email":   2,
		"erz1.straße":  3,
		"erz1.stadt":   4,
		"erz1.plz":     5,
		"erz1.notizen": 6,
		"erz1.sprache": 7,
	}
	values := []string{
		"Max", "Mustermann",
		"guardian@example.com",
		"", "", "", // Empty address
		"", "", // Empty notes/language
	}
	mapper := NewColumnMapper(mapping, values)

	row, err := MapStudentRow(mapper)

	require.NoError(t, err)
	require.Len(t, row.Guardians, 1)

	g := row.Guardians[0]
	assert.Equal(t, "", g.AddressStreet)
	assert.Equal(t, "", g.AddressCity)
	assert.Equal(t, "", g.AddressPostalCode)
	assert.Equal(t, "", g.Notes)
	assert.Equal(t, "", g.LanguagePreference)
}

// TestMapStudentRow_MultipleGuardiansWithProfileFields tests two guardians each with different profile data
func TestMapStudentRow_MultipleGuardiansWithProfileFields(t *testing.T) {
	t.Parallel()

	mapping := map[string]int{
		"vorname":      0,
		"nachname":     1,
		"erz1.email":   2,
		"erz1.straße":  3,
		"erz1.stadt":   4,
		"erz1.notizen": 5,
		"erz2.email":   6,
		"erz2.straße":  7,
		"erz2.stadt":   8,
		"erz2.notizen": 9,
	}
	values := []string{
		"Max", "Mustermann",
		"mother@example.com", "Hauptstr. 1", "Köln", "Mutter-Notiz",
		"father@example.com", "Nebenstr. 5", "Bonn", "Vater-Notiz",
	}
	mapper := NewColumnMapper(mapping, values)

	row, err := MapStudentRow(mapper)

	require.NoError(t, err)
	require.Len(t, row.Guardians, 2)

	g1 := row.Guardians[0]
	assert.Equal(t, "mother@example.com", g1.Email)
	assert.Equal(t, "Hauptstr. 1", g1.AddressStreet)
	assert.Equal(t, "Köln", g1.AddressCity)
	assert.Equal(t, "Mutter-Notiz", g1.Notes)

	g2 := row.Guardians[1]
	assert.Equal(t, "father@example.com", g2.Email)
	assert.Equal(t, "Nebenstr. 5", g2.AddressStreet)
	assert.Equal(t, "Bonn", g2.AddressCity)
	assert.Equal(t, "Vater-Notiz", g2.Notes)
}

// TestMapStudentRow_GuardianProfileWithPhoneNumbers tests combining profile fields and phone numbers
func TestMapStudentRow_GuardianProfileWithPhoneNumbers(t *testing.T) {
	t.Parallel()

	mapping := map[string]int{
		"vorname":         0,
		"nachname":        1,
		"erz1.email":      2,
		"erz1.telefon":    3,
		"erz1.mobil":      4,
		"erz1.dienstlich": 5,
		"erz1.straße":     6,
		"erz1.plz":        7,
	}
	values := []string{
		"Max", "Mustermann",
		"guardian@example.com",
		"+49221111", "+49177222", "+49221333",
		"Musterstr. 1", "50667",
	}
	mapper := NewColumnMapper(mapping, values)

	row, err := MapStudentRow(mapper)

	require.NoError(t, err)
	require.Len(t, row.Guardians, 1)

	g := row.Guardians[0]
	// Profile fields
	assert.Equal(t, "Musterstr. 1", g.AddressStreet)
	assert.Equal(t, "50667", g.AddressPostalCode)
	// Phone numbers (from ParseGuardianPhoneNumbers)
	require.Len(t, g.PhoneNumbers, 3)
	assert.Equal(t, "+49221111", g.PhoneNumbers[0].PhoneNumber)
	assert.Equal(t, "home", g.PhoneNumbers[0].PhoneType)
	assert.Equal(t, "+49177222", g.PhoneNumbers[1].PhoneNumber)
	assert.Equal(t, "mobile", g.PhoneNumbers[1].PhoneType)
	assert.Equal(t, "+49221333", g.PhoneNumbers[2].PhoneNumber)
	assert.Equal(t, "work", g.PhoneNumbers[2].PhoneType)
}

// TestMapStudentRow_PickupNotesWithoutTime tests that notes-only columns (no time) don't create schedule entries
func TestMapStudentRow_PickupNotesWithoutTime(t *testing.T) {
	t.Parallel()

	mapping := map[string]int{
		"vorname":             0,
		"nachname":            1,
		"abholung.mo":         2,
		"abholung.mo.notizen": 3,
		"abholung.di":         4,
		"abholung.di.notizen": 5,
	}
	values := []string{
		"Max", "Mustermann",
		"16:00", "Hort", // Monday: time + notes
		"", "Nur Notiz", // Tuesday: notes only (no time) -> should be skipped
	}
	mapper := NewColumnMapper(mapping, values)

	row, err := MapStudentRow(mapper)

	require.NoError(t, err)
	require.Len(t, row.PickupSchedules, 1, "should only include Monday since Tuesday has no time")
	assert.Equal(t, 1, row.PickupSchedules[0].Weekday)
	assert.Equal(t, "16:00", row.PickupSchedules[0].PickupTime)
	assert.Equal(t, "Hort", row.PickupSchedules[0].Notes)
}

// TestMapStudentRow_PickupScheduleWithEmptyNotes tests schedule entries with time but no notes
func TestMapStudentRow_PickupScheduleWithEmptyNotes(t *testing.T) {
	t.Parallel()

	mapping := map[string]int{
		"vorname":             0,
		"nachname":            1,
		"abholung.mo":         2,
		"abholung.mo.notizen": 3,
	}
	values := []string{
		"Max", "Mustermann",
		"14:30", "", // time but empty notes
	}
	mapper := NewColumnMapper(mapping, values)

	row, err := MapStudentRow(mapper)

	require.NoError(t, err)
	require.Len(t, row.PickupSchedules, 1)
	assert.Equal(t, "14:30", row.PickupSchedules[0].PickupTime)
	assert.Equal(t, "", row.PickupSchedules[0].Notes)
}

// TestMapStudentRow_FullCSVRow tests a realistic complete CSV row with all fields
func TestMapStudentRow_FullCSVRow(t *testing.T) {
	t.Parallel()

	mapping := map[string]int{
		"vorname":                   0,
		"nachname":                  1,
		"klasse":                    2,
		"geburtstag":                3,
		"erz1.vorname":              4,
		"erz1.nachname":             5,
		"erz1.email":                6,
		"erz1.telefon":              7,
		"erz1.verhältnis":           8,
		"erz1.hauptansprechpartner": 9,
		"erz1.notfall":              10,
		"erz1.abholberechtigt":      11,
		"erz1.straße":               12,
		"erz1.stadt":                13,
		"erz1.plz":                  14,
		"erz1.notizen":              15,
		"erz1.sprache":              16,
		"abholung.mo":               17,
		"abholung.mo.notizen":       18,
		"abholung.fr":               19,
		"abholung.fr.notizen":       20,
	}
	values := []string{
		"Max", "Mustermann", "1A", "2015-08-15",
		"Maria", "Müller", "maria@example.com", "+49123456",
		"Mutter", "Ja", "Ja", "Ja",
		"Musterstr. 1", "Köln", "50667",
		"Allergien beachten", "de",
		"16:00", "Hort",
		"14:00", "Frühschluss",
	}
	mapper := NewColumnMapper(mapping, values)

	row, err := MapStudentRow(mapper)

	require.NoError(t, err)

	// Student fields
	assert.Equal(t, "Max", row.FirstName)
	assert.Equal(t, "Mustermann", row.LastName)
	assert.Equal(t, "1A", row.SchoolClass)

	// Guardian
	require.Len(t, row.Guardians, 1)
	g := row.Guardians[0]
	assert.Equal(t, "Maria", g.FirstName)
	assert.Equal(t, "Müller", g.LastName)
	assert.Equal(t, "maria@example.com", g.Email)
	assert.Equal(t, "Mutter", g.RelationshipType)
	assert.True(t, g.IsPrimary)
	assert.True(t, g.IsEmergencyContact)
	assert.True(t, g.CanPickup)
	assert.Equal(t, "Musterstr. 1", g.AddressStreet)
	assert.Equal(t, "Köln", g.AddressCity)
	assert.Equal(t, "50667", g.AddressPostalCode)
	assert.Equal(t, "Allergien beachten", g.Notes)
	assert.Equal(t, "de", g.LanguagePreference)

	// Pickup schedules
	require.Len(t, row.PickupSchedules, 2)
	assert.Equal(t, 1, row.PickupSchedules[0].Weekday)
	assert.Equal(t, "16:00", row.PickupSchedules[0].PickupTime)
	assert.Equal(t, "Hort", row.PickupSchedules[0].Notes)
	assert.Equal(t, 5, row.PickupSchedules[1].Weekday)
	assert.Equal(t, "14:00", row.PickupSchedules[1].PickupTime)
	assert.Equal(t, "Frühschluss", row.PickupSchedules[1].Notes)
}

// TestColumnMapperIntegration tests the integration between ColumnMapper and other helpers
func TestColumnMapperIntegration(t *testing.T) {
	t.Parallel()

	t.Run("MapStudentRow uses GetRawCol for phone numbers", func(t *testing.T) {
		mapping := map[string]int{
			"vorname":      0,
			"nachname":     1,
			"erz1.email":   2,
			"erz1.telefon": 3,
		}
		values := []string{
			"Max",
			"Mustermann",
			"parent@example.com",
			"+49123456789", // Phone number starting with +
		}
		mapper := NewColumnMapper(mapping, values)

		row, err := MapStudentRow(mapper)

		require.NoError(t, err)
		require.Len(t, row.Guardians, 1)

		// Phone number should preserve the + prefix
		phones := row.Guardians[0].PhoneNumbers
		require.Len(t, phones, 1)
		assert.Equal(t, "+49123456789", phones[0].PhoneNumber)
	})
}
