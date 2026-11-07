package enrollmenthttp

import (
	"context"

	enrollment "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

func (m *mockFormSchemaService) StudentExtraFieldSchemas(ctx context.Context, ids []int64) ([]enrollment.SchemaVersionResult, error) {
	results := make([]enrollment.SchemaVersionResult, 0, len(ids))
	for _, id := range ids {
		schema, err := m.SchemaVersion(ctx, id)
		results = append(results, enrollment.SchemaVersionResult{ID: id, Schema: schema, Err: err})
	}
	return results, nil
}
