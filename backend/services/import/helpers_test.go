package importpkg

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSanitizeCellValue tests CSV injection protection
func TestSanitizeCellValue(t *testing.T) {
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
			"vorname":            0,
			"nachname":           1,
			"gesundheitsinfo":    2,
			"betreuernotizen":    3,
			"zusatzinfo":         4,
			"abholstatus":        5,
			"bus":                6,
			"aufbewahrung(tage)": 7,
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
			"vorname":         0,
			"nachname":        1,
			"erz1.vorname":    2,
			"erz1.nachname":   3,
			"erz1.email":      4,
			"erz1.telefon":    5,
			"erz1.mobil":      6,
			"erz1.verhältnis": 7,
			"erz1.primär":     8,
			"erz1.notfall":    9,
			"erz1.abholung":   10,
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

// TestColumnMapperIntegration tests the integration between ColumnMapper and other helpers
func TestColumnMapperIntegration(t *testing.T) {
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
