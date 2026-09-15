// Package imports defines Audit's import journal and resume receipt contract.
package imports

import "time"

// DataImport records the outcome of an import or one committed batch.
// Checkpoint metadata and the audit record are appended atomically.
type DataImport struct {
	TenantID                                                                      int64
	EntityType, Filename                                                          string
	TotalRows, CreatedCount, UpdatedCount, SkippedCount, ErrorCount, WarningCount int
	DryRun                                                                        bool
	ImportedBy                                                                    int64
	StartedAt                                                                     time.Time
	CompletedAt                                                                   *time.Time
	Checkpoint                                                                    *ImportCheckpoint
}

func (d *DataImport) GetTenantID() int64   { return d.TenantID }
func (d *DataImport) SetTenantID(id int64) { d.TenantID = id }
