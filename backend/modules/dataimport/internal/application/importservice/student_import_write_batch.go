package importpkg

import (
	"context"
	"fmt"
	"strings"

	importModels "github.com/moto-ogs/project-phoenix/modules/dataimport"
	"github.com/moto-ogs/project-phoenix/modules/dataimport/ports"
)

// BatchWriteConfig prepares name matches inside this write attempt, not in
// validation or across transaction retries. Create mode never renames existing
// persons or changes existing memberships. Consuming each name after its first
// lookup makes later namesakes observe any child created by an earlier row.
// Update/upsert still resolve every row afresh, including RFID-driven renames.
//
// A failed savepoint statement of the protected read is returned: the
// transaction can no longer be trusted, so no row reads on it (E19).
func (c *StudentImportConfig) BatchWriteConfig(ctx context.Context, rows []importModels.StudentImportRow) (ports.ImportConfig[importModels.StudentImportRow], error) {
	if importModeFromContext(ctx) != importModels.ImportModeCreate || len(rows) <= 4 {
		return c, nil
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
		return c, nil
	}
	// Even an initially absent Unicode name may be created in a canonically
	// equivalent spelling by an earlier row. Do not approximate SQL equality.
	filter, _ := studentPreviewNames(rows)
	if !asciiPreviewNames(filter.Names, nil) {
		return c, nil
	}
	view, err := c.previewConfig(ctx, untagged)
	if err != nil {
		return nil, fmt.Errorf("%w: read name matches: %v", errSavepointLost, err)
	}
	prepared, ok := view.(*StudentImportConfig)
	if !ok || prepared == c {
		return c, nil
	}
	return studentCreateBatchConfig{StudentImportConfig: c, matches: prepared}, nil
}

type studentCreateBatchConfig struct {
	*StudentImportConfig
	matches *StudentImportConfig
}

func (c studentCreateBatchConfig) FindExisting(ctx context.Context, row importModels.StudentImportRow) (*int64, error) {
	persons := c.matches.Persons.(previewPersons)
	defer delete(persons.names,
		previewName(strings.TrimSpace(row.FirstName), strings.TrimSpace(row.LastName)))
	if row.TagID != "" {
		return c.StudentImportConfig.FindExisting(ctx, row)
	}
	return c.matches.FindExisting(ctx, row)
}
