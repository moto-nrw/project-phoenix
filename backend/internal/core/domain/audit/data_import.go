package audit

import (
	"time"

	"github.com/uptrace/bun"
)

// DataImport tracks CSV/Excel import operations for GDPR compliance (Article 30)
// Records who imported what data, when, and the results
type DataImport struct {
	bun.BaseModel `bun:"table:audit.data_imports,alias:data_import"`

	ID           int64      `bun:"id,pk,autoincrement" json:"id"`
	EntityType   string     `bun:"entity_type,notnull" json:"entity_type"`               // student, teacher, room, etc.
	Filename     string     `bun:"filename,notnull" json:"filename"`                     // Original filename
	TotalRows    int        `bun:"total_rows,notnull" json:"total_rows"`                 // Total rows in file
	CreatedCount int        `bun:"created_count,notnull,default:0" json:"created_count"` // New records created
	UpdatedCount int        `bun:"updated_count,notnull,default:0" json:"updated_count"` // Existing records updated
	SkippedCount int        `bun:"skipped_count,notnull,default:0" json:"skipped_count"` // Rows skipped
	ErrorCount   int        `bun:"error_count,notnull,default:0" json:"error_count"`     // Rows with errors
	WarningCount int        `bun:"warning_count,notnull,default:0" json:"warning_count"` // Rows with warnings
	DryRun       bool       `bun:"dry_run,notnull,default:false" json:"dry_run"`         // Preview or actual import
	ImportedBy   int64      `bun:"imported_by,notnull" json:"imported_by"`               // Account ID
	StartedAt    time.Time  `bun:"started_at,notnull" json:"started_at"`                 // Import start time
	CompletedAt  *time.Time `bun:"completed_at" json:"completed_at"`                     // Import completion time
	Metadata     JSONBMap   `bun:"metadata,type:jsonb,default:'{}'" json:"metadata"`     // Error details, bulk actions, etc.
	CreatedAt    time.Time  `bun:"created_at,notnull,default:now()" json:"created_at"`
}

// JSONBMap is a simple map type for JSONB columns
type JSONBMap map[string]interface{}
