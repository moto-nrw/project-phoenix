package importpkg

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/services/import/ports"
)

// findStudentByExternalID resolves the child another system knows by key,
// such as an IServ user (#1446). It comes first: the key survives a class
// change and a missing birthday, which name, class and birthday do not. Like
// the name match it skips graduates, on both sides of the care interval.
func (c *StudentImportConfig) findStudentByExternalID(ctx context.Context, key string) (*int64, error) {
	if key == "" {
		return nil, nil
	}
	records, err := c.Students.ListStudentDirectory(ctx, ports.StudentDirectoryFilter{
		ExternalIDs: []string{key}, CareStatus: ports.StudentCareStatusAll,
	})
	if err != nil || len(records) == 0 {
		return nil, err
	}
	return &records[0].ID, nil
}

// storeExternalID keeps the key of the row on the child, so the next import
// of the same export finds it again.
func (c *StudentImportConfig) storeExternalID(ctx context.Context, studentID int64, key string) error {
	if key == "" {
		return nil
	}
	err := c.Students.SetStudentExternalID(ctx, studentID, key)
	if errors.Is(err, ports.ErrStudentExternalIDTaken) {
		return errors.New("Diese iServ-Kennung gehört schon zu einem anderen Kind") //nolint:staticcheck // ST1005: user-facing German message
	}
	if err != nil {
		return fmt.Errorf("Kennung speichern: %w", err) //nolint:staticcheck // ST1005: user-facing German message
	}
	return nil
}
