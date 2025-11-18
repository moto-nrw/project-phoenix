package importpkg

// StudentImportRow represents a single row of student import data
type StudentImportRow struct {
	// Person fields
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Birthday  string `json:"birthday,omitempty"` // YYYY-MM-DD
	TagID     string `json:"tag_id,omitempty"`   // RFID card

	// Student fields
	SchoolClass     string `json:"school_class"`
	GroupName       string `json:"group_name,omitempty"` // Human-readable (e.g., "Gruppe 1A")
	ExtraInfo       string `json:"extra_info,omitempty"`
	SupervisorNotes string `json:"supervisor_notes,omitempty"`
	HealthInfo      string `json:"health_info,omitempty"`
	BusPermission   bool   `json:"bus_permission"`

	// Multiple guardians (extensible: Erz1, Erz2, Erz3, ...)
	Guardians []GuardianImportData `json:"guardians,omitempty"`

	// Privacy consent
	PrivacyAccepted   bool `json:"privacy_accepted"`
	DataRetentionDays int  `json:"data_retention_days"` // 1-31, default 30

	// Resolved IDs (populated during validation, not in CSV)
	GroupID *int64 `json:"-"`
}

// GuardianImportData represents guardian information from CSV
type GuardianImportData struct {
	FirstName          string `json:"first_name,omitempty"`
	LastName           string `json:"last_name,omitempty"`
	Email              string `json:"email,omitempty"`
	Phone              string `json:"phone,omitempty"`
	MobilePhone        string `json:"mobile_phone,omitempty"`
	RelationshipType   string `json:"relationship_type,omitempty"` // "Mutter", "Vater", "Oma", etc.
	IsPrimary          bool   `json:"is_primary"`
	IsEmergencyContact bool   `json:"is_emergency_contact"`
	CanPickup          bool   `json:"can_pickup"`
}
