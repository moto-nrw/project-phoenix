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
	PickupStatus    string `json:"pickup_status,omitempty"` // "Geht alleine nach Hause" or "Wird abgeholt"
	// BusPermission is the legacy single "Bus" column (Ja/Nein). When true and no
	// per-day columns are present it maps to all weekdays (Mo–Fr).
	BusPermission bool `json:"bus_permission"`
	// BusDays holds optional per-day "Bus.Mo".."Bus.Fr" columns keyed by the
	// canonical weekday keys (mon/tue/wed/thu/fri). nil when no per-day column is
	// present, in which case BusPermission is used. bus_days is the SSOT (#1582).
	BusDays map[string]bool `json:"bus_days,omitempty"`
	// DepartureDays holds the unified per-day "Gehweise.Mo".."Gehweise.Fr"
	// columns keyed by the canonical weekday keys (mon/tue/wed/thu/fri), each
	// value one of alone/bus/pickup. nil when no per-day Gehweise column is
	// present, in which case the legacy Bus/Abholstatus columns are folded into
	// the departure plan instead. departure_days is the SSOT (#1610).
	DepartureDays map[string]string `json:"departure_days,omitempty"`
	// DepartureCompanionNote is the free-text "mit wem" detail for the
	// accompanied ("mit anderem Kind") departure mode: who the child leaves
	// with. Optional; bounded on persist. The note never outlives the
	// accompanied mode (the repository clears it when no day is accompanied),
	// so a value supplied without any accompanied day is dropped (#1694).
	DepartureCompanionNote string `json:"departure_companion_note,omitempty"`
	EnrolledFrom           string `json:"enrolled_from,omitempty"`  // YYYY-MM-DD (also accepts DD.MM.YYYY / DD.MM.YY)
	EnrolledUntil          string `json:"enrolled_until,omitempty"` // YYYY-MM-DD (also accepts DD.MM.YYYY / DD.MM.YY)

	// Consent dates — the explicit date each consent was given (e.g. from a
	// signed paper form). Empty = no consent recorded. A set date asserts the
	// consent was given on that date. Photo consent's "given_by" is left empty
	// on import.
	AGBAcceptedAt            string `json:"agb_accepted_at,omitempty"`
	DataProcessingAcceptedAt string `json:"data_processing_accepted_at,omitempty"`
	EmailContactAcceptedAt   string `json:"email_contact_accepted_at,omitempty"`
	PhotoConsentGivenAt      string `json:"photo_consent_given_at,omitempty"`

	// Multiple guardians (extensible: Erz1, Erz2, Erz3, ...)
	Guardians []GuardianImportData `json:"guardians,omitempty"`

	// Pickup schedule (weekly recurring)
	PickupSchedules []PickupScheduleImportData `json:"pickup_schedules,omitempty"`

	// Arrival schedule (weekly recurring)
	ArrivalSchedules []ArrivalScheduleImportData `json:"arrival_schedules,omitempty"`

	// Privacy consent
	PrivacyAccepted   bool `json:"privacy_accepted"`
	DataRetentionDays int  `json:"data_retention_days"` // 1-31, default 30

	// Resolved IDs (populated during validation, not in CSV)
	GroupID *int64 `json:"-"`
}

// PickupScheduleImportData represents a weekly pickup schedule entry from CSV
type PickupScheduleImportData struct {
	Weekday    int    `json:"weekday"`         // 1-5 (Mon-Fri)
	PickupTime string `json:"pickup_time"`     // HH:MM format
	Notes      string `json:"notes,omitempty"` // Day-specific notes
}

// ArrivalScheduleImportData represents a weekly arrival schedule entry from CSV
type ArrivalScheduleImportData struct {
	Weekday         int    `json:"weekday"`          // 1-5 (Mon-Fri)
	ExpectedArrival string `json:"expected_arrival"` // HH:MM format
	Notes           string `json:"notes,omitempty"`  // Day-specific notes
}

// PhoneImportData represents a phone number from CSV import
type PhoneImportData struct {
	PhoneNumber string `json:"phone_number"`
	PhoneType   string `json:"phone_type"` // mobile, home, work, other
	Label       string `json:"label"`      // e.g., "Dienstlich", "Geschäftlich"
	IsPrimary   bool   `json:"is_primary"`
}

// GuardianImportData represents guardian information from CSV
type GuardianImportData struct {
	FirstName          string `json:"first_name,omitempty"`
	LastName           string `json:"last_name,omitempty"`
	Email              string `json:"email,omitempty"`
	Phone              string `json:"phone,omitempty"`             // Deprecated: prefer PhoneNumbers
	MobilePhone        string `json:"mobile_phone,omitempty"`      // Deprecated: prefer PhoneNumbers
	RelationshipType   string `json:"relationship_type,omitempty"` // "Mutter", "Vater", "Oma", etc.
	IsPrimary          bool   `json:"is_primary"`
	IsEmergencyContact bool   `json:"is_emergency_contact"`
	CanPickup          bool   `json:"can_pickup"`
	// PhoneNumbers contains flexible phone number data (from CSV columns like Erz{N}.Dienstlich)
	PhoneNumbers []PhoneImportData `json:"phone_numbers,omitempty"`

	// Address
	AddressStreet     string `json:"address_street,omitempty"`
	AddressCity       string `json:"address_city,omitempty"`
	AddressPostalCode string `json:"address_postal_code,omitempty"`

	// Preferences & Notes
	Notes              string `json:"notes,omitempty"`
	LanguagePreference string `json:"language_preference,omitempty"`
}
