# 📋 CSV/Excel Import Architecture Plan (User-Friendly Edition)

**Created**: 2025-11-17
**Updated**: 2025-11-17
**Status**: Planning / Awaiting Approval
**Target**: Student import (initially), extensible to other entities (teachers, rooms, groups)

---

## Executive Summary

**Goal**: Create an extensible, user-friendly data import system optimized for **non-IT users** that starts with students but can easily extend to other entities.

**Target Users**: School administrators and staff with **limited IT experience** who need to:
- Import student data from Excel/CSV files
- Use human-readable names (not database IDs)
- Get helpful error messages in German
- Fix errors interactively with suggestions

**Current State**:
- ✅ Frontend UI complete (file upload, preview, validation, editing)
- ✅ Native CSV parsing (no dependencies needed)
- ❌ No backend implementation yet
- ❌ No file upload endpoints
- ❌ No bulk import logic
- ❌ No fuzzy matching for relationships

**Key Requirements**:
1. **Maximum user-friendliness** - designed for non-IT users
2. **Smart error handling** - fuzzy matching, suggestions, bulk corrections
3. **Human-readable relationships** - use names, not IDs
4. **Multiple guardians** - extensible to 2, 3, 4+ guardians per student
5. **Empty field support** - graceful handling of optional fields
6. **Extensible architecture** - teachers, rooms, groups in the future
7. **GDPR compliance** - privacy consent, data retention, audit logging
8. **Transaction safety** - atomic imports (all-or-nothing)

---

## User Experience Focus

### Target User Profile

**Who**: School administrators/staff without IT background
**Skills**: Can use Excel, fill out forms, basic computer skills
**Pain Points**:
- Don't know database IDs
- Don't understand technical error messages
- Need to import 50-200 students at once
- Have data in Excel from previous school year
- Make typos ("Gruppe A" vs "Gruppe 1A")

### User-Friendly Design Principles

1. **Human-Readable Input**
   - ✅ CSV uses "Gruppe A" (not group_id: 42)
   - ✅ Backend resolves names to IDs automatically
   - ✅ Fuzzy matching: "Gruppe A" → suggests "Gruppe 1A"

2. **Helpful Error Messages**
   - ❌ BAD: `"validation failed: guardian_email"`
   - ✅ GOOD: `"Zeile 5: Ungültiges Email-Format für Erziehungsberechtigten (maria@example)"`
   - ✅ GOOD: `"Gruppe 'Gruppe A' nicht gefunden. Meinten Sie 'Gruppe 1A'?"` (with click-to-fix button)

3. **Interactive Error Fixing**
   - Upload CSV → See all errors at once (not one-by-one)
   - Click suggestion button to auto-fix
   - Bulk fix: "Change all 'Gruppe A' to 'Gruppe 1A' in 5 rows"
   - Inline editing for complex fixes

4. **Forgiving Input**
   - Empty fields OK for optional data
   - Extra spaces trimmed automatically
   - Case-insensitive matching ("gruppe a" = "Gruppe A")
   - Multiple guardians: leave Erz2/Erz3 empty if not needed

---

## CSV Template Structure

### Student Import Template (Extensible Guardian Support)

**File**: `schueler-import-vorlage.csv`

```csv
Vorname,Nachname,Klasse,Gruppe,Geburtstag,Erz1.Vorname,Erz1.Nachname,Erz1.Email,Erz1.Telefon,Erz1.Verhältnis,Erz1.Primär,Erz2.Vorname,Erz2.Nachname,Erz2.Email,Erz2.Telefon,Erz2.Verhältnis,Erz2.Primär,Gesundheitsinfo,Betreuernotizen,Datenschutz,Aufbewahrung(Tage),Bus
Max,Mustermann,1A,Gruppe 1A,2015-08-15,Maria,Müller,maria.mueller@example.com,0123-456789,Mutter,Ja,Hans,Müller,hans.mueller@example.com,0123-987654,Vater,Nein,,Sehr ruhiges Kind,Ja,30,Nein
Anna,Schmidt,2B,Gruppe 2B,2014-03-22,Petra,Schmidt,petra.schmidt@example.com,0234-567890,Mutter,Ja,,,,,,,Allergie: Nüsse,,Ja,15,Ja
```

**Field Reference**:

| Column | Required? | Example | Notes |
|--------|-----------|---------|-------|
| Vorname | ✅ Yes | Max | Student first name |
| Nachname | ✅ Yes | Mustermann | Student last name |
| Klasse | ✅ Yes | 1A | School class |
| Gruppe | ❌ No | Gruppe 1A | **Human-readable name** (resolved to ID) |
| Geburtstag | ❌ No | 2015-08-15 | Format: YYYY-MM-DD |
| Erz1.Vorname | ❌ No* | Maria | Guardian 1 first name |
| Erz1.Nachname | ❌ No* | Müller | Guardian 1 last name |
| Erz1.Email | ❌ No* | maria@... | **Used for deduplication** |
| Erz1.Telefon | ❌ No* | 0123-456789 | Alternative contact |
| Erz1.Verhältnis | ❌ No | Mutter | Relationship type |
| Erz1.Primär | ❌ No | Ja/Nein | Is primary contact? |
| Erz2.* | ❌ No | (same as Erz1) | **Optional 2nd guardian** |
| Erz3.* | ❌ No | (same as Erz1) | **Optional 3rd guardian** (extensible) |
| Gesundheitsinfo | ❌ No | Allergie: Nüsse | Health information |
| Betreuernotizen | ❌ No | Ruhiges Kind | Supervisor notes |
| Datenschutz | ❌ No | Ja/Nein | Privacy consent |
| Aufbewahrung(Tage) | ❌ No | 30 | Data retention (1-31 days) |
| Bus | ❌ No | Ja/Nein | Bus permission flag |

*At least ONE contact method (email OR phone) required if guardian provided

### **Guardian Extensibility**

The system **automatically detects** guardian columns:
- **Erz1.*** - First guardian (most common)
- **Erz2.*** - Second guardian (e.g., both parents)
- **Erz3.*** - Third guardian (e.g., grandparent, legal guardian)
- **Erz4.*** - Fourth guardian (rare, but supported)

**CSV parser dynamically detects all `ErzN.*` columns** - no code changes needed to support more guardians!

---

## Architecture Overview

### 1. **Generic Import Framework** (Go Generics)

```go
// models/import/types.go
package importpkg

// ImportConfig defines entity-specific import behavior
type ImportConfig[T any] interface {
    // PreloadReferenceData loads all reference data (groups, rooms) into memory cache
    PreloadReferenceData(ctx context.Context) error

    // Validate validates a single row of import data
    Validate(ctx context.Context, row *T) []ValidationError

    // FindExisting checks if entity already exists (for duplicate detection)
    FindExisting(ctx context.Context, row T) (*int64, error)

    // Create creates a new entity from import data
    Create(ctx context.Context, row T) (int64, error)

    // Update updates an existing entity
    Update(ctx context.Context, id int64, row T) error

    // EntityName returns the entity type name (for logging/errors)
    EntityName() string
}

// ImportService handles generic import logic
type ImportService[T any] struct {
    config    ImportConfig[T]
    db        *bun.DB
    txHandler *base.TxHandler
    auditRepo audit.DataImportRepository
    batchSize int // Default: 100
}

// ImportRequest contains the raw import data
type ImportRequest[T any] struct {
    Rows           []T
    Mode           ImportMode // Create, Update, Upsert
    DryRun         bool       // Preview only
    StopOnError    bool       // Stop on first error (false = collect all)
    UserID         int64      // Who is importing
    SkipInvalidRows bool      // Skip invalid rows and continue
}

// ImportMode defines how to handle existing records
type ImportMode string

const (
    ImportModeCreate ImportMode = "create" // Only create new (error on duplicate)
    ImportModeUpdate ImportMode = "update" // Only update existing (error on new)
    ImportModeUpsert ImportMode = "upsert" // Create or update (recommended)
)

// ImportResult tracks import outcomes
type ImportResult[T any] struct {
    StartedAt       time.Time
    CompletedAt     time.Time
    TotalRows       int
    CreatedCount    int
    UpdatedCount    int
    SkippedCount    int
    ErrorCount      int
    WarningCount    int
    Errors          []ImportError[T]
    BulkActions     []BulkAction // Suggested bulk corrections
    DryRun          bool
}

// ImportError captures per-row failures
type ImportError[T any] struct {
    RowNumber int               // CSV row number (1-indexed, excludes header)
    Data      T                 // The row data that failed
    Errors    []ValidationError
    Timestamp time.Time
}

// ValidationError describes a specific field validation failure
type ValidationError struct {
    Field       string        `json:"field"`       // e.g., "first_name", "group"
    Message     string        `json:"message"`     // German user-friendly message
    Code        string        `json:"code"`        // Machine-readable code
    Severity    ErrorSeverity `json:"severity"`    // error, warning, info
    Suggestions []string      `json:"suggestions,omitempty"` // Autocorrect options
    AutoFix     *AutoFix      `json:"auto_fix,omitempty"`    // Suggested fix
}

// ErrorSeverity defines error importance
type ErrorSeverity string

const (
    ErrorSeverityError   ErrorSeverity = "error"   // Blocking: must fix
    ErrorSeverityWarning ErrorSeverity = "warning" // Non-blocking: can proceed
    ErrorSeverityInfo    ErrorSeverity = "info"    // Informational only
)

// AutoFix describes an automatic correction option
type AutoFix struct {
    Action      string `json:"action"`       // "replace", "create", "ignore"
    Replacement string `json:"replacement"`  // New value to use
    Description string `json:"description"`  // German explanation
}

// BulkAction represents a suggested bulk correction
type BulkAction struct {
    Title        string `json:"title"`         // "5 Zeilen verwenden 'Gruppe A'"
    Description  string `json:"description"`   // "Alle zu 'Gruppe 1A' ändern?"
    Action       string `json:"action"`        // "replace_all"
    AffectedRows []int  `json:"affected_rows"` // Row numbers
    Field        string `json:"field"`         // "group"
    OldValue     string `json:"old_value"`     // "Gruppe A"
    NewValue     string `json:"new_value"`     // "Gruppe 1A"
}
```

### 2. **Relationship Resolver** (Fuzzy Matching for Non-IT Users)

```go
// services/import/relationship_resolver.go
package importpkg

import (
    "context"
    "strings"
    "github.com/agnivade/levenshtein"
)

type RelationshipResolver struct {
    groupRepo groups.GroupRepository
    roomRepo  facilities.RoomRepository

    // In-memory caches (pre-loaded)
    groupCache map[string]*education.Group // lowercase name → group
    roomCache  map[string]*facilities.Room // lowercase name → room
}

func NewRelationshipResolver(groupRepo groups.GroupRepository, roomRepo facilities.RoomRepository) *RelationshipResolver {
    return &RelationshipResolver{
        groupRepo:  groupRepo,
        roomRepo:   roomRepo,
        groupCache: make(map[string]*education.Group),
        roomCache:  make(map[string]*facilities.Room),
    }
}

// PreloadGroups loads all groups into memory cache (called once before import)
func (r *RelationshipResolver) PreloadGroups(ctx context.Context) error {
    groups, err := r.groupRepo.List(ctx, &base.QueryOptions{Limit: 1000})
    if err != nil {
        return fmt.Errorf("preload groups: %w", err)
    }

    for _, group := range groups {
        key := strings.ToLower(strings.TrimSpace(group.Name))
        r.groupCache[key] = group
    }

    return nil
}

// ResolveGroup resolves human-readable group name to ID with fuzzy matching
func (r *RelationshipResolver) ResolveGroup(ctx context.Context, groupName string) (*int64, []ValidationError) {
    if groupName == "" {
        return nil, nil // Optional field - empty is OK
    }

    normalized := strings.ToLower(strings.TrimSpace(groupName))

    // 1. Exact match (case-insensitive)
    if group, exists := r.groupCache[normalized]; exists {
        return &group.ID, nil
    }

    // 2. Fuzzy match (Levenshtein distance ≤ 3)
    suggestions := r.findSimilarGroups(groupName, 3)

    if len(suggestions) > 0 {
        return nil, []ValidationError{{
            Field:    "group",
            Message:  fmt.Sprintf("Gruppe '%s' nicht gefunden. Meinten Sie: %s?", groupName, strings.Join(suggestions, ", ")),
            Code:     "group_not_found_with_suggestions",
            Severity: ErrorSeverityError,
            Suggestions: suggestions,
            AutoFix: &AutoFix{
                Action:      "replace",
                Replacement: suggestions[0], // Best match
                Description: fmt.Sprintf("Automatisch zu '%s' ändern", suggestions[0]),
            },
        }}
    }

    // 3. No matches - suggest creating or leaving empty
    return nil, []ValidationError{{
        Field:    "group",
        Message:  fmt.Sprintf("Gruppe '%s' existiert nicht. Bitte erstellen Sie die Gruppe zuerst oder lassen Sie das Feld leer.", groupName),
        Code:     "group_not_found",
        Severity: ErrorSeverityError,
    }}
}

// findSimilarGroups finds group names within Levenshtein distance threshold
func (r *RelationshipResolver) findSimilarGroups(input string, maxDistance int) []string {
    type match struct {
        name     string
        distance int
    }

    var matches []match
    inputLower := strings.ToLower(input)

    for _, group := range r.groupCache {
        nameLower := strings.ToLower(group.Name)
        distance := levenshtein.ComputeDistance(inputLower, nameLower)

        if distance <= maxDistance {
            matches = append(matches, match{name: group.Name, distance: distance})
        }
    }

    // Sort by distance (closest first)
    sort.Slice(matches, func(i, j int) bool {
        return matches[i].distance < matches[j].distance
    })

    // Return top 3 suggestions
    result := make([]string, 0, 3)
    for i := 0; i < len(matches) && i < 3; i++ {
        result = append(result, matches[i].name)
    }

    return result
}
```

### 3. **Student Import Implementation** (Multiple Guardians + Smart Validation)

```go
// services/import/student_import_config.go
package importpkg

type StudentImportRow struct {
    // Person fields
    FirstName string `json:"first_name"`
    LastName  string `json:"last_name"`
    Birthday  string `json:"birthday,omitempty"` // YYYY-MM-DD
    TagID     string `json:"tag_id,omitempty"`   // RFID card

    // Student fields
    SchoolClass     string `json:"school_class"`
    GroupName       string `json:"group_name,omitempty"`       // Human-readable (e.g., "Gruppe 1A")
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

type StudentImportConfig struct {
    personRepo   persons.PersonRepository
    studentRepo  students.StudentRepository
    guardianRepo guardians.GuardianProfileRepository
    relationRepo guardians.StudentGuardianRepository
    privacyRepo  privacy.PrivacyConsentRepository
    resolver     *RelationshipResolver
    txHandler    *base.TxHandler
}

func (c *StudentImportConfig) PreloadReferenceData(ctx context.Context) error {
    // Pre-load all groups for relationship resolution
    return c.resolver.PreloadGroups(ctx)
}

func (c *StudentImportConfig) Validate(ctx context.Context, row *StudentImportRow) []ValidationError {
    errors := []ValidationError{}

    // 1. REQUIRED: Person validation
    if strings.TrimSpace(row.FirstName) == "" {
        errors = append(errors, ValidationError{
            Field:    "first_name",
            Message:  "Vorname ist erforderlich",
            Code:     "required",
            Severity: ErrorSeverityError,
        })
    }

    if strings.TrimSpace(row.LastName) == "" {
        errors = append(errors, ValidationError{
            Field:    "last_name",
            Message:  "Nachname ist erforderlich",
            Code:     "required",
            Severity: ErrorSeverityError,
        })
    }

    // 2. REQUIRED: Student validation
    if strings.TrimSpace(row.SchoolClass) == "" {
        errors = append(errors, ValidationError{
            Field:    "school_class",
            Message:  "Klasse ist erforderlich",
            Code:     "required",
            Severity: ErrorSeverityError,
        })
    }

    // 3. OPTIONAL: Group resolution (with fuzzy matching)
    if row.GroupName != "" {
        groupID, groupErrors := c.resolver.ResolveGroup(ctx, row.GroupName)
        if len(groupErrors) > 0 {
            errors = append(errors, groupErrors...)
        } else {
            row.GroupID = groupID // Cache resolved ID
        }
    } else {
        // INFO: Group empty - student will be created without group
        errors = append(errors, ValidationError{
            Field:    "group",
            Message:  "Keine Gruppe zugewiesen. Der Schüler wird ohne Gruppe erstellt.",
            Code:     "group_empty",
            Severity: ErrorSeverityInfo, // Non-blocking
        })
    }

    // 4. OPTIONAL: Guardian validation
    for i, guardian := range row.Guardians {
        guardianErrors := c.validateGuardian(i+1, guardian)
        errors = append(errors, guardianErrors...)
    }

    // 5. Birthday validation (if provided)
    if row.Birthday != "" {
        if _, err := time.Parse("2006-01-02", row.Birthday); err != nil {
            errors = append(errors, ValidationError{
                Field:    "birthday",
                Message:  "Ungültiges Datumsformat. Bitte verwenden Sie JJJJ-MM-TT (z.B. 2015-08-15)",
                Code:     "invalid_date_format",
                Severity: ErrorSeverityError,
            })
        }
    }

    // 6. Privacy validation
    if row.DataRetentionDays < 1 || row.DataRetentionDays > 31 {
        errors = append(errors, ValidationError{
            Field:    "data_retention_days",
            Message:  "Aufbewahrungsdauer muss zwischen 1 und 31 Tagen liegen",
            Code:     "invalid_range",
            Severity: ErrorSeverityError,
        })
    }

    return errors
}

func (c *StudentImportConfig) validateGuardian(num int, guardian GuardianImportData) []ValidationError {
    errors := []ValidationError{}
    fieldPrefix := fmt.Sprintf("guardian_%d", num)

    // At least one contact method required
    if guardian.Email == "" && guardian.Phone == "" && guardian.MobilePhone == "" {
        errors = append(errors, ValidationError{
            Field:    fieldPrefix,
            Message:  fmt.Sprintf("Erziehungsberechtigter %d benötigt mindestens eine Kontaktmethode (Email, Telefon oder Mobil)", num),
            Code:     "guardian_contact_required",
            Severity: ErrorSeverityError,
        })
    }

    // Email format validation (if provided)
    if guardian.Email != "" && !isValidEmail(guardian.Email) {
        errors = append(errors, ValidationError{
            Field:    fmt.Sprintf("%s_email", fieldPrefix),
            Message:  fmt.Sprintf("Ungültiges Email-Format für Erziehungsberechtigten %d: %s", num, guardian.Email),
            Code:     "invalid_email",
            Severity: ErrorSeverityError,
        })
    }

    return errors
}

func (c *StudentImportConfig) Create(ctx context.Context, row StudentImportRow) (int64, error) {
    var studentID int64

    err := c.txHandler.RunInTx(ctx, func(txCtx context.Context, tx bun.Tx) error {
        // 1. Create Person
        birthday, _ := parseOptionalDate(row.Birthday)
        person := &users.Person{
            FirstName: strings.TrimSpace(row.FirstName),
            LastName:  strings.TrimSpace(row.LastName),
            Birthday:  birthday,
            TagID:     stringPtr(row.TagID),
        }

        personID, err := c.personRepo.Create(txCtx, person)
        if err != nil {
            return fmt.Errorf("create person: %w", err)
        }

        // 2. Create Student
        student := &users.Student{
            PersonID:        personID,
            SchoolClass:     strings.TrimSpace(row.SchoolClass),
            GroupID:         row.GroupID, // May be nil (no group)
            ExtraInfo:       stringPtr(row.ExtraInfo),
            SupervisorNotes: stringPtr(row.SupervisorNotes),
            HealthInfo:      stringPtr(row.HealthInfo),
            BusPermission:   row.BusPermission,
        }

        studentID, err = c.studentRepo.Create(txCtx, student)
        if err != nil {
            return fmt.Errorf("create student: %w", err)
        }

        // 3. Create/Link Multiple Guardians
        for i, guardianData := range row.Guardians {
            guardianID, err := c.createOrFindGuardian(txCtx, guardianData)
            if err != nil {
                return fmt.Errorf("guardian %d: %w", i+1, err)
            }

            // Create Student-Guardian Relationship
            relationship := &users.StudentGuardian{
                StudentID:          studentID,
                GuardianProfileID:  guardianID,
                RelationshipType:   guardianData.RelationshipType,
                IsPrimary:          guardianData.IsPrimary,
                IsEmergencyContact: guardianData.IsEmergencyContact,
                CanPickup:          guardianData.CanPickup,
            }

            _, err = c.relationRepo.Create(txCtx, relationship)
            if err != nil {
                return fmt.Errorf("create relationship %d: %w", i+1, err)
            }
        }

        // 4. Create Privacy Consent
        if row.PrivacyAccepted || row.DataRetentionDays > 0 {
            consent := &users.PrivacyConsent{
                StudentID:         studentID,
                Accepted:          row.PrivacyAccepted,
                DataRetentionDays: row.DataRetentionDays,
            }

            if row.PrivacyAccepted {
                now := time.Now()
                consent.AcceptedAt = &now
            }

            _, err = c.privacyRepo.Create(txCtx, consent)
            if err != nil {
                return fmt.Errorf("create privacy consent: %w", err)
            }
        }

        return nil
    })

    return studentID, err
}

// createOrFindGuardian deduplicates guardians by email
func (c *StudentImportConfig) createOrFindGuardian(ctx context.Context, data GuardianImportData) (int64, error) {
    // Deduplication strategy: Email is unique identifier
    if data.Email != "" {
        existing, err := c.guardianRepo.FindByEmail(ctx, data.Email)
        if err == nil && existing != nil {
            // Reuse existing guardian
            return existing.ID, nil
        }
    }

    // Create new guardian
    guardian := &users.GuardianProfile{
        FirstName:   strings.TrimSpace(data.FirstName),
        LastName:    strings.TrimSpace(data.LastName),
        Email:       stringPtr(data.Email),
        Phone:       stringPtr(data.Phone),
        MobilePhone: stringPtr(data.MobilePhone),
    }

    return c.guardianRepo.Create(ctx, guardian)
}

func (c *StudentImportConfig) EntityName() string {
    return "student"
}
```

### 4. **CSV Parser** (Auto-Detects Guardian Columns)

```go
// services/import/csv_parser.go
package importpkg

import (
    "encoding/csv"
    "fmt"
    "io"
    "strconv"
    "strings"
)

type CSVParser[T any] struct {
    columnMapping map[string]int // CSV column name → index (lowercase)
    rowMapper     func(values []string, mapping map[string]int) (T, error)
}

func NewStudentCSVParser() *CSVParser[StudentImportRow] {
    return &CSVParser[StudentImportRow]{
        rowMapper: mapStudentRow,
    }
}

func (p *CSVParser[T]) Parse(reader io.Reader) ([]T, error) {
    csvReader := csv.NewReader(reader)
    csvReader.FieldsPerRecord = -1 // Variable columns (support any number of guardians)
    csvReader.TrimLeadingSpace = true

    // Read header
    header, err := csvReader.Read()
    if err != nil {
        return nil, fmt.Errorf("read header: %w", err)
    }

    // Build column mapping (case-insensitive)
    p.columnMapping = make(map[string]int)
    for i, col := range header {
        key := strings.ToLower(strings.TrimSpace(col))
        p.columnMapping[key] = i
    }

    // Read data rows
    var rows []T
    for rowNum := 2; ; rowNum++ { // Start at 2 (1=header)
        values, err := csvReader.Read()
        if err == io.EOF {
            break
        }
        if err != nil {
            return nil, fmt.Errorf("row %d: %w", rowNum, err)
        }

        row, err := p.rowMapper(values, p.columnMapping)
        if err != nil {
            return nil, fmt.Errorf("row %d: %w", rowNum, err)
        }

        rows = append(rows, row)
    }

    return rows, nil
}

// mapStudentRow maps CSV values to StudentImportRow
func mapStudentRow(values []string, mapping map[string]int) (StudentImportRow, error) {
    row := StudentImportRow{
        DataRetentionDays: 30, // Default
    }

    // Helper: Get column value safely
    getCol := func(colName string) string {
        idx, exists := mapping[colName]
        if !exists || idx < 0 || idx >= len(values) {
            return "" // Column doesn't exist or out of range
        }
        return strings.TrimSpace(values[idx])
    }

    // Parse boolean ("Ja"/"Nein")
    parseBool := func(val string) bool {
        return strings.ToLower(val) == "ja"
    }

    // Map student fields
    row.FirstName = getCol("vorname")
    row.LastName = getCol("nachname")
    row.SchoolClass = getCol("klasse")
    row.GroupName = getCol("gruppe") // Human-readable name (e.g., "Gruppe 1A")
    row.Birthday = getCol("geburtstag")
    row.HealthInfo = getCol("gesundheitsinfo")
    row.SupervisorNotes = getCol("betreuernotizen")
    row.ExtraInfo = getCol("zusatzinfo")
    row.BusPermission = parseBool(getCol("bus"))

    // Privacy consent
    row.PrivacyAccepted = parseBool(getCol("datenschutz"))
    if retentionStr := getCol("aufbewahrung(tage)"); retentionStr != "" {
        if retention, err := strconv.Atoi(retentionStr); err == nil {
            row.DataRetentionDays = retention
        }
    }

    // AUTO-DETECT GUARDIANS (Erz1, Erz2, Erz3, ...)
    guardianNum := 1
    for {
        emailKey := fmt.Sprintf("erz%d.email", guardianNum)
        phoneKey := fmt.Sprintf("erz%d.telefon", guardianNum)

        // Check if this guardian number exists in CSV
        _, hasEmail := mapping[emailKey]
        _, hasPhone := mapping[phoneKey]

        if !hasEmail && !hasPhone {
            break // No more guardians
        }

        guardian := GuardianImportData{
            FirstName:        getCol(fmt.Sprintf("erz%d.vorname", guardianNum)),
            LastName:         getCol(fmt.Sprintf("erz%d.nachname", guardianNum)),
            Email:            getCol(emailKey),
            Phone:            getCol(phoneKey),
            MobilePhone:      getCol(fmt.Sprintf("erz%d.mobil", guardianNum)),
            RelationshipType: getCol(fmt.Sprintf("erz%d.verhältnis", guardianNum)),
            IsPrimary:        parseBool(getCol(fmt.Sprintf("erz%d.primär", guardianNum))),
            IsEmergencyContact: parseBool(getCol(fmt.Sprintf("erz%d.notfall", guardianNum))),
            CanPickup:        parseBool(getCol(fmt.Sprintf("erz%d.abholung", guardianNum))),
        }

        // Only add if has contact info (skip empty guardians)
        if guardian.Email != "" || guardian.Phone != "" {
            row.Guardians = append(row.Guardians, guardian)
        }

        guardianNum++
    }

    return row, nil
}
```

### 5. **API Endpoints**

```go
// api/import/api.go
package importapi

type Resource struct {
    studentImportService *importpkg.ImportService[importpkg.StudentImportRow]
    teacherImportService *importpkg.ImportService[importpkg.TeacherImportRow] // Future
}

func (rs *Resource) Router() chi.Router {
    r := chi.NewRouter()
    r.Use(tokenAuth.Verifier())
    r.Use(jwt.Authenticator)

    // Student import
    r.With(authorize.RequiresPermission(permissions.UsersCreate)).
        Post("/students/preview", rs.previewStudentImport)
    r.With(authorize.RequiresPermission(permissions.UsersCreate)).
        Post("/students/import", rs.importStudents)
    r.With(authorize.RequiresPermission(permissions.UsersRead)).
        Get("/students/template", rs.downloadStudentTemplate)

    // Future: Teacher import
    // r.Post("/teachers/preview", rs.previewTeacherImport)
    // r.Post("/teachers/import", rs.importTeachers)

    return r
}

// GET /api/import/students/template - Download CSV template with examples
func (rs *Resource) downloadStudentTemplate(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "text/csv; charset=utf-8")
    w.Header().Set("Content-Disposition", "attachment; filename=schueler-import-vorlage.csv")

    csvWriter := csv.NewWriter(w)

    // Header row
    headers := []string{
        "Vorname", "Nachname", "Klasse", "Gruppe", "Geburtstag",
        "Erz1.Vorname", "Erz1.Nachname", "Erz1.Email", "Erz1.Telefon", "Erz1.Verhältnis", "Erz1.Primär",
        "Erz2.Vorname", "Erz2.Nachname", "Erz2.Email", "Erz2.Telefon", "Erz2.Verhältnis", "Erz2.Primär",
        "Gesundheitsinfo", "Betreuernotizen", "Datenschutz", "Aufbewahrung(Tage)", "Bus",
    }
    csvWriter.Write(headers)

    // Example rows with realistic data
    examples := [][]string{
        {
            "Max", "Mustermann", "1A", "Gruppe 1A", "2015-08-15",
            "Maria", "Müller", "maria.mueller@example.com", "0123-456789", "Mutter", "Ja",
            "Hans", "Müller", "hans.mueller@example.com", "0123-987654", "Vater", "Nein",
            "", "Sehr ruhiges Kind", "Ja", "30", "Nein",
        },
        {
            "Anna", "Schmidt", "2B", "Gruppe 2B", "2014-03-22",
            "Petra", "Schmidt", "petra.schmidt@example.com", "0234-567890", "Mutter", "Ja",
            "", "", "", "", "", "", // No second guardian (empty is OK!)
            "Allergie: Nüsse", "", "Ja", "15", "Ja",
        },
    }

    for _, row := range examples {
        csvWriter.Write(row)
    }

    csvWriter.Flush()
}

// POST /api/import/students/preview - Validate and preview import (dry-run)
func (rs *Resource) previewStudentImport(w http.ResponseWriter, r *http.Request) {
    // Security: File size limit
    const maxFileSize = 10 * 1024 * 1024 // 10MB
    if r.ContentLength > maxFileSize {
        api.RespondError(w, r, http.StatusRequestEntityTooLarge,
            "Datei zu groß (max 10MB)", "file_too_large")
        return
    }

    // Parse CSV file from multipart form
    file, header, err := r.FormFile("file")
    if err != nil {
        api.RespondError(w, r, http.StatusBadRequest, "Datei fehlt", "missing_file")
        return
    }
    defer file.Close()

    // Validate file type
    allowedTypes := []string{"text/csv", "application/vnd.ms-excel", "text/plain"}
    contentType := header.Header.Get("Content-Type")
    if !contains(allowedTypes, contentType) {
        api.RespondError(w, r, http.StatusBadRequest,
            "Ungültiger Dateityp (nur CSV erlaubt)", "invalid_file_type")
        return
    }

    // Parse CSV
    parser := importpkg.NewStudentCSVParser()
    rows, err := parser.Parse(file)
    if err != nil {
        api.RespondError(w, r, http.StatusBadRequest,
            fmt.Sprintf("CSV-Fehler: %s", err.Error()), "invalid_csv")
        return
    }

    // Run dry-run import (preview only, no database changes)
    ctx := r.Context()
    request := importpkg.ImportRequest[importpkg.StudentImportRow]{
        Rows:        rows,
        Mode:        importpkg.ImportModeUpsert,
        DryRun:      true,  // PREVIEW ONLY
        StopOnError: false, // Collect all errors
        UserID:      getUserID(ctx),
    }

    result, err := rs.studentImportService.Import(ctx, request)
    if err != nil {
        api.RespondError(w, r, http.StatusInternalServerError,
            fmt.Sprintf("Vorschau fehlgeschlagen: %s", err.Error()), "preview_failed")
        return
    }

    api.RespondSuccess(w, r, result)
}

// POST /api/import/students/import - Perform actual import
func (rs *Resource) importStudents(w http.ResponseWriter, r *http.Request) {
    // ... same file parsing as preview ...

    request := importpkg.ImportRequest[importpkg.StudentImportRow]{
        Rows:            rows,
        Mode:            importpkg.ImportModeUpsert,
        DryRun:          false, // ACTUAL IMPORT
        StopOnError:     false,
        UserID:          getUserID(ctx),
        SkipInvalidRows: true, // Import valid rows, skip errors
    }

    result, err := rs.studentImportService.Import(ctx, request)
    if err != nil {
        api.RespondError(w, r, http.StatusInternalServerError,
            fmt.Sprintf("Import fehlgeschlagen: %s", err.Error()), "import_failed")
        return
    }

    // Audit logging
    if result.CreatedCount > 0 || result.UpdatedCount > 0 {
        auditEntry := &audit.DataImport{
            EntityType:    "student",
            Filename:      header.Filename,
            TotalRows:     result.TotalRows,
            CreatedCount:  result.CreatedCount,
            UpdatedCount:  result.UpdatedCount,
            ErrorCount:    result.ErrorCount,
            ImportedBy:    getUserID(ctx),
            StartedAt:     result.StartedAt,
            CompletedAt:   &result.CompletedAt,
        }
        // Save audit entry...
    }

    api.RespondSuccess(w, r, result)
}
```

---

## Frontend Integration (User-Friendly UI)

### Enhanced Error Display with Suggestions

```typescript
// frontend/src/app/database/students/csv-import/page.tsx

interface CSVStudent {
    row: number;
    status: 'new' | 'existing' | 'error' | 'warning' | 'info';
    errors: ValidationError[];
    // ... other fields
}

interface ValidationError {
    field: string;
    message: string;
    code: string;
    severity: 'error' | 'warning' | 'info';
    suggestions?: string[];
    auto_fix?: {
        action: string;
        replacement: string;
        description: string;
    };
}

function ErrorDisplay({ student, onApplySuggestion }: Props) {
    return (
        <div className="space-y-2">
            {student.errors.map((err, idx) => (
                <div key={idx} className={`border-l-4 p-3 ${getSeverityClass(err.severity)}`}>
                    <p className="font-medium">{err.message}</p>

                    {/* Autocorrect suggestions */}
                    {err.suggestions && err.suggestions.length > 0 && (
                        <div className="mt-2 flex flex-wrap gap-2">
                            <span className="text-sm text-gray-600">Meinten Sie:</span>
                            {err.suggestions.map(suggestion => (
                                <button
                                    key={suggestion}
                                    onClick={() => onApplySuggestion(student.row, err.field, suggestion)}
                                    className="btn-sm btn-outline text-blue-600 hover:bg-blue-50"
                                >
                                    {suggestion}
                                </button>
                            ))}
                        </div>
                    )}

                    {/* Auto-fix button */}
                    {err.auto_fix && (
                        <button
                            onClick={() => onApplyAutoFix(student.row, err.field, err.auto_fix)}
                            className="mt-2 btn-sm btn-primary"
                        >
                            {err.auto_fix.description}
                        </button>
                    )}
                </div>
            ))}
        </div>
    );
}

function getSeverityClass(severity: string): string {
    switch (severity) {
        case 'error': return 'border-red-500 bg-red-50';
        case 'warning': return 'border-yellow-500 bg-yellow-50';
        case 'info': return 'border-blue-500 bg-blue-50';
        default: return 'border-gray-500 bg-gray-50';
    }
}
```

### Bulk Correction UI

```typescript
interface BulkAction {
    title: string;
    description: string;
    action: string;
    affected_rows: number[];
    field: string;
    old_value: string;
    new_value: string;
}

function BulkActionsPanel({ bulkActions, onApplyBulk }: Props) {
    return (
        <div className="bg-blue-50 border border-blue-200 rounded p-4 space-y-3">
            <h3 className="font-medium text-blue-900">Massenkorrektur verfügbar</h3>

            {bulkActions.map((action, idx) => (
                <div key={idx} className="bg-white rounded p-3 shadow-sm">
                    <p className="font-medium">{action.title}</p>
                    <p className="text-sm text-gray-600">{action.description}</p>
                    <p className="text-sm text-gray-500">
                        Betrifft {action.affected_rows.length} Zeilen
                    </p>
                    <button
                        onClick={() => onApplyBulk(action)}
                        className="mt-2 btn-sm btn-primary"
                    >
                        Alle ändern: "{action.old_value}" → "{action.new_value}"
                    </button>
                </div>
            ))}
        </div>
    );
}
```

---

## Database Schema Changes

### New Tables

```sql
-- Import history tracking
CREATE TABLE IF NOT EXISTS audit.data_imports (
    id BIGSERIAL PRIMARY KEY,
    entity_type TEXT NOT NULL,          -- 'student', 'teacher', 'room'
    filename TEXT NOT NULL,
    total_rows INT NOT NULL,
    created_count INT NOT NULL DEFAULT 0,
    updated_count INT NOT NULL DEFAULT 0,
    skipped_count INT NOT NULL DEFAULT 0,
    error_count INT NOT NULL DEFAULT 0,
    warning_count INT NOT NULL DEFAULT 0,
    dry_run BOOLEAN NOT NULL DEFAULT FALSE,
    imported_by BIGINT NOT NULL REFERENCES auth.accounts(id),
    started_at TIMESTAMPTZ NOT NULL,
    completed_at TIMESTAMPTZ,
    metadata JSONB,                     -- Store error details, bulk actions
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_data_imports_entity_type ON audit.data_imports(entity_type);
CREATE INDEX idx_data_imports_imported_by ON audit.data_imports(imported_by);
CREATE INDEX idx_data_imports_created_at ON audit.data_imports(created_at DESC);
```

### New Indexes for Performance

```sql
-- Guardian deduplication by email
CREATE INDEX IF NOT EXISTS idx_guardian_profiles_email
    ON users.guardian_profiles(LOWER(email))
    WHERE email IS NOT NULL;

-- Student duplicate detection by name + class
CREATE INDEX IF NOT EXISTS idx_students_name_class
    ON users.students(person_id, school_class);

-- Group name lookup (case-insensitive)
CREATE INDEX IF NOT EXISTS idx_groups_name_lower
    ON education.groups(LOWER(name));
```

### New Repository Methods

```go
// StudentRepository
func (r *StudentRepository) FindByNameAndClass(
    ctx context.Context,
    firstName, lastName, schoolClass string,
) ([]*users.Student, error)

// GuardianProfileRepository
func (r *GuardianProfileRepository) FindByEmail(
    ctx context.Context,
    email string,
) (*users.GuardianProfile, error)

// GroupRepository
func (r *GroupRepository) ListAll(
    ctx context.Context,
) ([]*education.Group, error) // For cache pre-loading

// DataImportRepository (new)
type DataImportRepository interface {
    Create(ctx context.Context, importRecord *audit.DataImport) (int64, error)
    List(ctx context.Context, options *base.QueryOptions) ([]*audit.DataImport, error)
    GetByID(ctx context.Context, id int64) (*audit.DataImport, error)
}
```

---

## Implementation Phases

### **Phase 1: Core Framework + Student Import** (Week 1-2)

**Tasks**:
1. Create generic `ImportService[T]` with `ImportConfig` interface
2. Implement `RelationshipResolver` with fuzzy matching (Levenshtein)
3. Implement `StudentImportConfig` with validation
4. Add CSV parser with auto-detection of guardian columns
5. Create API endpoints (`/preview`, `/import`, `/template`)
6. Add multipart file upload handling with security checks
7. Implement batch processing (100 rows/batch)
8. Add transaction safety (`RunInTx`)
9. Write unit tests for validation + fuzzy matching

**Deliverables**:
- Generic import framework (reusable for teachers, rooms, etc.)
- Working student import with preview
- Fuzzy matching for group names
- Multiple guardian support (Erz1, Erz2, Erz3, ...)
- Template download with examples

**Success Criteria**:
- Preview returns validation errors with suggestions
- No database changes in dry-run mode
- Fuzzy matching works: "Gruppe A" → suggests "Gruppe 1A"
- Multiple guardians imported correctly
- Guardian deduplication by email works

---

### **Phase 2: Frontend Integration + UX Polish** (Week 3)

**Tasks**:
1. Update frontend to call backend `/preview` endpoint
2. Replace simulated validation with real server responses
3. Implement suggestion buttons (click to auto-fix)
4. Add bulk correction UI
5. Improve error display with severity indicators
6. Add import progress indicator (for large files)
7. Handle backend error responses gracefully
8. Add success/failure notifications

**Deliverables**:
- Frontend uses real backend validation
- Interactive error fixing (click suggestions)
- Bulk actions panel
- Progress feedback
- Success/failure toasts

**Success Criteria**:
- No more `Math.random()` validation (all backend)
- Users can fix errors with 1 click
- Bulk corrections work ("Fix all 5 rows")
- Import completes with clear feedback

---

### **Phase 3: Teacher Import Extension** (Week 4)

**Tasks**:
1. Create `TeacherImportRow` struct
2. Implement `TeacherImportConfig`
3. Add teacher-specific validation (email required, PIN format)
4. Create teacher import endpoints
5. Add teacher CSV template download
6. Extend relationship resolver for rooms
7. Integration tests for teacher import

**Deliverables**:
- Teacher import working with same framework
- Room assignment with fuzzy matching
- Account creation with password/PIN
- Template download

**Success Criteria**:
- Import 50 teachers successfully
- Rooms resolved by name
- Accounts created with proper roles
- Same UX as student import

---

### **Phase 4: Extensions & History** (Future)

**Tasks**:
1. Room import
2. Group import
3. Import history page (show past imports)
4. Error report download (CSV with failed rows)
5. Excel `.xlsx` support (using `excelize` library)
6. Column mapping UI (drag-and-drop columns)
7. Performance optimization (streaming for 1000+ rows)

---

## Testing Strategy

### Unit Tests

```go
func TestRelationshipResolverFuzzyMatching(t *testing.T) {
    resolver := &RelationshipResolver{
        groupCache: map[string]*education.Group{
            "gruppe 1a": {ID: 1, Name: "Gruppe 1A"},
            "gruppe 2b": {ID: 2, Name: "Gruppe 2B"},
        },
    }

    tests := []struct {
        input       string
        wantID      *int64
        wantSuggestions bool
    }{
        {"Gruppe 1A", int64Ptr(1), false},      // Exact match
        {"gruppe 1a", int64Ptr(1), false},      // Case-insensitive
        {"Gruppe A", nil, true},                 // Fuzzy match → suggestions
        {"Gruppe 1B", nil, true},                // Close match
        {"Gruppe XYZ", nil, false},              // No match
    }

    for _, tt := range tests {
        t.Run(tt.input, func(t *testing.T) {
            id, errors := resolver.ResolveGroup(context.Background(), tt.input)

            if tt.wantID != nil {
                assert.Equal(t, *tt.wantID, *id)
                assert.Empty(t, errors)
            } else if tt.wantSuggestions {
                assert.Nil(t, id)
                assert.NotEmpty(t, errors)
                assert.NotEmpty(t, errors[0].Suggestions)
            } else {
                assert.Nil(t, id)
                assert.NotEmpty(t, errors)
                assert.Empty(t, errors[0].Suggestions)
            }
        })
    }
}

func TestStudentImportMultipleGuardians(t *testing.T) {
    row := StudentImportRow{
        FirstName:   "Max",
        LastName:    "Mustermann",
        SchoolClass: "1A",
        Guardians: []GuardianImportData{
            {Email: "maria@example.com", FirstName: "Maria", LastName: "Müller"},
            {Email: "hans@example.com", FirstName: "Hans", LastName: "Müller"},
            {Phone: "0123-456789", FirstName: "Oma", LastName: "Schmidt"}, // No email
        },
    }

    config := &StudentImportConfig{...}
    errors := config.Validate(context.Background(), &row)

    assert.Empty(t, errors) // All guardians valid
    assert.Len(t, row.Guardians, 3)
}
```

### Integration Tests

```go
func TestStudentImportWithFuzzyGroupMatching(t *testing.T) {
    db := setupTestDB(t)
    defer cleanupTestDB(db)

    // Create test group
    group := &education.Group{Name: "Gruppe 1A"}
    groupID, _ := groupRepo.Create(context.Background(), group)

    // Import student with typo in group name
    csvData := `Vorname,Nachname,Klasse,Gruppe
Max,Mustermann,1A,Gruppe A`

    parser := importpkg.NewStudentCSVParser()
    rows, _ := parser.Parse(strings.NewReader(csvData))

    config := &StudentImportConfig{...}
    config.PreloadReferenceData(context.Background())

    errors := config.Validate(context.Background(), &rows[0])

    // Should suggest "Gruppe 1A"
    assert.Len(t, errors, 1)
    assert.Equal(t, "group", errors[0].Field)
    assert.Contains(t, errors[0].Suggestions, "Gruppe 1A")
}
```

### Bruno API Tests

```javascript
// bruno/20-student-import.bru
meta {
  name: Student Import - Preview with Fuzzy Matching
  type: http
  seq: 1
}

post {
  url: {{baseUrl}}/api/import/students/preview
  body: multipartForm
  auth: bearer
}

auth:bearer {
  token: {{authToken}}
}

body:multipart-form {
  file: @file(./fixtures/students-with-typos.csv)
}

assert {
  res.status: eq 200
  res.body.data.error_count: gt 0
}

tests {
  test("should suggest similar group names", function() {
    const data = res.getBody().data;
    const errors = data.errors[0].errors;

    const groupError = errors.find(e => e.field === 'group');
    expect(groupError).to.exist;
    expect(groupError.suggestions).to.be.an('array');
    expect(groupError.suggestions.length).to.be.greaterThan(0);
  });
}
```

---

## Security & GDPR

### File Upload Security

```go
// Security checks
const maxFileSize = 10 * 1024 * 1024 // 10MB

// 1. File size limit
if r.ContentLength > maxFileSize {
    return api.RespondError(w, r, http.StatusRequestEntityTooLarge,
        "Datei zu groß (max 10MB)", "file_too_large")
}

// 2. File type validation
allowedTypes := []string{"text/csv", "application/vnd.ms-excel"}
if !contains(allowedTypes, header.Header.Get("Content-Type")) {
    return api.RespondError(w, r, http.StatusBadRequest,
        "Ungültiger Dateityp (nur CSV erlaubt)", "invalid_file_type")
}

// 3. CSV injection protection
func scanCSVInjection(value string) bool {
    // Detect formula injection (=, +, -, @, \t, \r)
    if len(value) > 0 && strings.ContainsAny(value[:1], "=+-@\t\r") {
        return true
    }
    return false
}

// 4. Secure temp file cleanup
defer func() {
    if err := os.Remove(tempFilePath); err != nil {
        logging.Logger.Warn("failed to delete temp file", "path", tempFilePath)
    }
}()
```

### GDPR Compliance

1. **Privacy Consent**: Import validates and creates consent records
2. **Data Retention**: Enforces 1-31 day retention policy per student
3. **Audit Trail**: Logs who imported what, when, and how many records
4. **Right to Erasure**: Imported students can still be deleted
5. **Data Minimization**: Only import necessary fields
6. **Access Control**: `UsersCreate` permission required

```go
// Audit log entry
auditEntry := &audit.DataImport{
    EntityType:   "student",
    Filename:     filename,
    TotalRows:    result.TotalRows,
    CreatedCount: result.CreatedCount,
    ImportedBy:   userID,
    StartedAt:    result.StartedAt,
    CompletedAt:  &result.CompletedAt,
    Metadata: map[string]interface{}{
        "error_count":    result.ErrorCount,
        "warning_count":  result.WarningCount,
        "dry_run":        request.DryRun,
    },
}
```

---

## Summary

### Architecture Highlights

- ✅ **User-friendly for non-IT users** - fuzzy matching, German messages, click-to-fix
- ✅ **Generic `ImportService[T]`** - reusable for students, teachers, rooms, groups
- ✅ **Multiple guardians** - auto-detects Erz1, Erz2, Erz3, ... (unlimited)
- ✅ **Empty field support** - optional fields gracefully handled
- ✅ **Smart relationship resolution** - human-readable names with fuzzy matching
- ✅ **Batch processing** - 100 rows/batch with transaction safety
- ✅ **Two-tier validation** - client (instant) + server (authoritative)
- ✅ **Guardian deduplication** - by email (smart merging)
- ✅ **Preview before commit** - dry-run with full validation
- ✅ **Comprehensive error collection** - all errors shown at once
- ✅ **Bulk corrections** - fix multiple rows with one click
- ✅ **GDPR-compliant** - audit logging, privacy consent, data retention

### Current Limitations (MVP - Phase 1)

- ⚠️ **Create-only mode**: Import will create new students only. Duplicate students (same first name, last name, and class) will be rejected with error: `"Schüler existiert bereits"`. Update mode will be added in Phase 2.
- ⚠️ **RFID cards not supported**: RFID card assignment must be done separately after import via device management interface
- ⚠️ **Bulk actions UI**: Bulk correction suggestions are generated but not yet implemented in frontend (Phase 2)

### User Experience Highlights

- ✅ Template download with realistic examples
- ✅ Drag-and-drop file upload
- ✅ Real-time validation with error highlighting
- ✅ Autocorrect suggestions ("Did you mean...?")
- ✅ Bulk fix UI ("Change all 5 rows at once")
- ✅ Inline row editing for complex fixes
- ✅ Progress feedback for large files
- ✅ Success/failure notifications
- ✅ Error/warning/info severity indicators
- ✅ Download error report CSV (future)

### Extensibility

Same framework supports:
- ✅ Students (Phase 1)
- ✅ Teachers (Phase 3)
- ✅ Rooms (Phase 4)
- ✅ Groups (Phase 4)
- ✅ Activities (future)
- ✅ Any entity with Go generics

---

## Next Steps

1. **Review and approve this architecture**
2. **Answer open questions** (duplicate handling, file size limits, Excel support timing)
3. **Begin Phase 1 implementation**:
   - Generic import framework
   - Relationship resolver with fuzzy matching
   - Student import with multiple guardians
   - CSV parser with auto-detection
   - API endpoints with template download
4. **Database migration**: Create `audit.data_imports` table + indexes
5. **Testing**: Unit tests for fuzzy matching + guardian parsing

**Estimated Timeline**: 3-4 weeks for full student import with polished UX

---

**Created**: 2025-11-17
**Status**: Ready for implementation approval
**Target Users**: School administrators (non-IT)
**Key Innovation**: Fuzzy matching + extensible guardian support + user-friendly error handling
