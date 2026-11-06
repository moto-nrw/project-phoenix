package importpkg

import (
	"context"
	"strings"

	importModels "github.com/moto-nrw/project-phoenix/modules/dataimport"
	"github.com/moto-nrw/project-phoenix/modules/dataimport/ports"
)

// BatchWriteConfig prepares name matches inside this write attempt, not in
// validation or across transaction retries. Create mode never renames existing
// persons or changes existing memberships. Consuming each name after its first
// lookup makes later namesakes observe any child created by an earlier row.
// Update/upsert still resolve every row afresh, including RFID-driven renames.
func (c *StudentImportConfig) BatchWriteConfig(ctx context.Context, rows []importModels.StudentImportRow) ports.ImportConfig[importModels.StudentImportRow] {
	if importModeFromContext(ctx) != importModels.ImportModeCreate || len(rows) <= 4 {
		return c
	}
	untagged := make([]importModels.StudentImportRow, 0, len(rows))
	for _, row := range rows {
		if row.TagID == "" {
			untagged = append(untagged, row)
		}
	}
	_, names := studentPreviewNames(untagged)
	// A protected read takes at most four statements (persons, students,
	// SAVEPOINT, RELEASE). Each distinct name replaces at least one lookup.
	if len(names) <= 4 {
		return c
	}
	// Even an initially absent Unicode name may be created in a canonically
	// equivalent spelling by an earlier row. Do not approximate SQL equality.
	filter, _ := studentPreviewNames(rows)
	if !asciiPreviewNames(filter.Names, nil) {
		return c
	}
	prepared, ok := c.PreviewConfig(ctx, untagged).(*StudentImportConfig)
	if !ok || prepared == c {
		return c
	}
	return studentCreateBatchConfig{StudentImportConfig: c, matches: prepared}
}

type studentCreateBatchConfig struct {
	*StudentImportConfig
	matches *StudentImportConfig
}

func (c studentCreateBatchConfig) FindExisting(ctx context.Context, row importModels.StudentImportRow) (*int64, error) {
	defer delete(c.matches.Persons.(previewPersons).names,
		previewName(strings.TrimSpace(row.FirstName), strings.TrimSpace(row.LastName)))
	if row.TagID != "" {
		return c.StudentImportConfig.FindExisting(ctx, row)
	}
	return c.matches.FindExisting(ctx, row)
}
