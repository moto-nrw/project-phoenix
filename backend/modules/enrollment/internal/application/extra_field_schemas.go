package application

import (
	"context"
	"fmt"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
)

// StudentExtraFieldSchemas reads historical versions without changing when
// the renderer consumes each version's error.
func (s *FormSchemas) StudentExtraFieldSchemas(ctx context.Context, ids []int64) ([]enrollment.SchemaVersionResult, error) {
	valid := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id > 0 {
			valid = append(valid, id)
		}
	}
	results, err := s.records.StudentExtraFieldSchemas(ctx, valid)
	for i := range results {
		if s.notFound(results[i].Err) {
			results[i].Err = fmt.Errorf("%w: %w", enrollment.ErrFormSchemaNotFound, results[i].Err)
		}
		results[i].Schema = enrollment.CopyFormSchema(results[i].Schema)
	}
	for _, id := range ids {
		if id <= 0 {
			results = append(results, enrollment.SchemaVersionResult{ID: id, Err: fmt.Errorf("schema id must be positive")})
		}
	}
	return results, err
}
