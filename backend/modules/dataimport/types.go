package dataimport

import (
	"fmt"
	"strings"
	"time"
)

// ImportMode defines how to handle existing records
type ImportMode string

const (
	ImportModeCreate ImportMode = "create" // Only create new (error on duplicate)
	ImportModeUpdate ImportMode = "update" // Only update existing (error on new)
	ImportModeUpsert ImportMode = "upsert" // Create new rows, update existing ones
)

// ParseImportMode maps the form value of the import UI to an ImportMode. An
// empty value keeps the historical default (create-only); anything unknown is
// rejected so a typo can never silently turn a preview into an update run.
func ParseImportMode(raw string) (ImportMode, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", string(ImportModeCreate):
		return ImportModeCreate, nil
	case string(ImportModeUpdate):
		return ImportModeUpdate, nil
	case string(ImportModeUpsert):
		return ImportModeUpsert, nil
	default:
		return "", fmt.Errorf("unbekannter Import-Modus '%s' (erlaubt: create, update, upsert)", raw)
	}
}

// ErrorSeverity defines error importance
type ErrorSeverity string

const (
	ErrorSeverityError   ErrorSeverity = "error"   // Blocking: must fix
	ErrorSeverityWarning ErrorSeverity = "warning" // Non-blocking: can proceed
	ErrorSeverityInfo    ErrorSeverity = "info"    // Informational only
)

// ImportRequest contains the raw import data
type ImportRequest[T any] struct {
	Rows            []T
	Mode            ImportMode // Create, Update, Upsert
	DryRun          bool       // Preview only
	StopOnError     bool       // Stop on first error (false = collect all)
	UserID          int64      // Who is importing
	SkipInvalidRows bool       // Skip invalid rows and continue
}

// ImportResult tracks import outcomes
type ImportResult[T any] struct {
	StartedAt    time.Time
	CompletedAt  time.Time
	TotalRows    int
	CreatedCount int
	UpdatedCount int
	SkippedCount int
	ErrorCount   int
	WarningCount int
	Errors       []ImportError[T]
	BulkActions  []BulkAction // Suggested bulk corrections
	DryRun       bool
}

// ImportError captures per-row failures
type ImportError[T any] struct {
	RowNumber int // CSV row number (1-indexed, excludes header)
	Data      T   // The row data that failed
	Errors    []ValidationError
	Timestamp time.Time
}

// ValidationError describes a specific field validation failure
type ValidationError struct {
	Field       string        `json:"field"`                  // e.g., "first_name", "group"
	Message     string        `json:"message"`                // German user-friendly message
	Code        string        `json:"code"`                   // Machine-readable code
	Severity    ErrorSeverity `json:"severity"`               // error, warning, info
	Suggestions []string      `json:"suggestions,omitempty"`  // Autocorrect options
	AutoFix     *AutoFix      `json:"auto_fix,omitempty"`     // Suggested fix
	ActualValue string        `json:"actual_value,omitempty"` // The actual value that caused the error (for bulk actions)
}

// AutoFix describes an automatic correction option
type AutoFix struct {
	Action      string `json:"action"`      // "replace", "create", "ignore"
	Replacement string `json:"replacement"` // New value to use
	Description string `json:"description"` // German explanation
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
