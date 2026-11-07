package enrollmenthttp

import (
	"context"

	enrollment "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// toStudentHistorySummaries batches only pinned schema display metadata.
// Phase, child and offering reads keep their per-request visibility and order.
func (rs *Resource) toStudentHistorySummaries(ctx context.Context, summaries []*RequestSummary) []AdminRequestSummary {
	out := make([]AdminRequestSummary, 0, len(summaries))
	var schemas map[int64]*enrollment.FormSchema
	loaded := false
	for _, summary := range summaries {
		if !loaded && rs.FormSchemaService != nil && summary != nil && summary.Request != nil && summary.Request.SchemaID != nil {
			// Keep the first schema read after any preceding rows' offering
			// guards. A malformed version must not hide another version.
			schemas = rs.studentHistorySchemas(ctx, summaries)
			loaded = true
		}
		out = append(out, rs.toStudentHistorySummary(ctx, summary, schemas))
	}
	return out
}

func (rs *Resource) studentHistorySchemas(ctx context.Context, summaries []*RequestSummary) map[int64]*enrollment.FormSchema {
	ids := make([]int64, 0, len(summaries))
	for _, summary := range summaries {
		if summary != nil && summary.Request != nil && summary.Request.SchemaID != nil {
			ids = append(ids, *summary.Request.SchemaID)
		}
	}
	results, err := rs.FormSchemaService.StudentExtraFieldSchemas(ctx, ids)
	if err != nil {
		return nil // Retry the original per-row reads if the batch itself failed.
	}
	schemas := make(map[int64]*enrollment.FormSchema, len(results))
	for _, result := range results {
		if result.Err == nil {
			schemas[result.ID] = result.Schema
		}
	}
	return schemas
}

func (rs *Resource) toStudentHistorySummary(ctx context.Context, summary *RequestSummary, schemas map[int64]*enrollment.FormSchema) AdminRequestSummary {
	if schemas == nil || summary == nil || summary.Request == nil {
		return rs.toAdminRequestDetailSummary(ctx, summary)
	}
	detail := toAdminRequestSummary(summary)
	detail.CustomData = summary.Request.CustomData
	detail.ConsentFlags = summary.Request.ConsentFlags
	if summary.Request.SchemaID != nil {
		if schema := schemas[*summary.Request.SchemaID]; schema != nil {
			detail.SchemaFields, detail.SchemaLegalBlocks = toAdminSchemaMetadata(schema)
		}
	}
	if rs.DecisionService != nil {
		childOfferings, err := rs.DecisionService.ListChildOfferings(ctx, summary.Request.ID)
		if err != nil {
			for i := range detail.Children {
				detail.Children[i].OfferingsUnavailable = true
			}
		} else {
			attachChildOfferings(detail.Children, summary.Children, childOfferings)
		}
	}
	return detail
}
