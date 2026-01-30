package importpkg

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// =============================================================================
// StudentImportRow Tests
// =============================================================================

func TestStudentImportRow_DefaultValues(t *testing.T) {
	row := StudentImportRow{}

	assert.Empty(t, row.FirstName)
	assert.Empty(t, row.LastName)
	assert.Empty(t, row.Birthday)
	assert.Empty(t, row.TagID)
	assert.Empty(t, row.SchoolClass)
	assert.Empty(t, row.GroupName)
	assert.False(t, row.BusPermission)
	assert.Empty(t, row.Guardians)
	assert.False(t, row.PrivacyAccepted)
	assert.Zero(t, row.DataRetentionDays)
	assert.Nil(t, row.GroupID)
}

func TestStudentImportRow_WithValues(t *testing.T) {
	groupID := int64(5)
	row := StudentImportRow{
		FirstName:       "Max",
		LastName:        "Mustermann",
		Birthday:        "2015-03-15",
		TagID:           "RFID-001",
		SchoolClass:     "1a",
		GroupName:       "Gruppe 1A",
		ExtraInfo:       "Allergien: keine",
		SupervisorNotes: "Pünktlich",
		HealthInfo:      "Keine",
		PickupStatus:    "Geht alleine nach Hause",
		BusPermission:   true,
		Guardians: []GuardianImportData{
			{FirstName: "Anna", LastName: "Mustermann", RelationshipType: "Mutter", IsPrimary: true},
		},
		PrivacyAccepted:   true,
		DataRetentionDays: 30,
		GroupID:           &groupID,
	}

	assert.Equal(t, "Max", row.FirstName)
	assert.Equal(t, "2015-03-15", row.Birthday)
	assert.Equal(t, "RFID-001", row.TagID)
	assert.Equal(t, "1a", row.SchoolClass)
	assert.True(t, row.BusPermission)
	assert.Len(t, row.Guardians, 1)
	assert.True(t, row.PrivacyAccepted)
	assert.Equal(t, 30, row.DataRetentionDays)
	assert.Equal(t, int64(5), *row.GroupID)
}

// =============================================================================
// PhoneImportData Tests
// =============================================================================

func TestPhoneImportData_Fields(t *testing.T) {
	phone := PhoneImportData{
		PhoneNumber: "+49 123 456789",
		PhoneType:   "mobile",
		Label:       "Dienstlich",
		IsPrimary:   true,
	}

	assert.Equal(t, "+49 123 456789", phone.PhoneNumber)
	assert.Equal(t, "mobile", phone.PhoneType)
	assert.Equal(t, "Dienstlich", phone.Label)
	assert.True(t, phone.IsPrimary)
}

// =============================================================================
// GuardianImportData Tests
// =============================================================================

func TestGuardianImportData_Fields(t *testing.T) {
	guardian := GuardianImportData{
		FirstName:          "Anna",
		LastName:           "Mustermann",
		Email:              "anna@example.com",
		Phone:              "+49 123 456",
		MobilePhone:        "+49 171 456",
		RelationshipType:   "Mutter",
		IsPrimary:          true,
		IsEmergencyContact: true,
		CanPickup:          true,
		PhoneNumbers: []PhoneImportData{
			{PhoneNumber: "+49 123 456", PhoneType: "home", IsPrimary: true},
		},
	}

	assert.Equal(t, "Anna", guardian.FirstName)
	assert.Equal(t, "anna@example.com", guardian.Email)
	assert.Equal(t, "Mutter", guardian.RelationshipType)
	assert.True(t, guardian.IsPrimary)
	assert.True(t, guardian.IsEmergencyContact)
	assert.True(t, guardian.CanPickup)
	assert.Len(t, guardian.PhoneNumbers, 1)
}

func TestGuardianImportData_MinimalFields(t *testing.T) {
	guardian := GuardianImportData{
		FirstName: "Peter",
		LastName:  "Test",
	}

	assert.Equal(t, "Peter", guardian.FirstName)
	assert.Empty(t, guardian.Email)
	assert.False(t, guardian.IsPrimary)
	assert.Empty(t, guardian.PhoneNumbers)
}
