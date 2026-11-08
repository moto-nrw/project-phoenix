package application

import (
	"context"
	"fmt"

	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// FormLabels loads the labels of a request of the school in context (#6142).
// A miss, including a request of another school, is ErrRequestNotFound.
func (s *Intake) FormLabels(ctx context.Context, requestID int64) (*enrollmentNative.RequestFormLabels, error) {
	req, err := s.labelledRequest(ctx, requestID)
	if err != nil {
		return nil, err
	}
	return s.requestFormLabels(ctx, req)
}

func (s *Intake) labelledRequest(ctx context.Context, requestID int64) (*Request, error) {
	req, err := decodedRequestByID(ctx, s.deps.Requests, requestID, false)
	if (err != nil && s.deps.Runtime.NotFound(err)) || (err == nil && req == nil) {
		return nil, enrollmentNative.ErrRequestNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("form labels: load request: %w", err)
	}
	return req, nil
}

// FormLabelsByStatusToken loads the labels of the request behind a valid
// status token. The token is the only authentication, like the status
// page's own reads.
func (s *Intake) FormLabelsByStatusToken(ctx context.Context, token string) (*enrollmentNative.RequestFormLabels, error) {
	req, err := s.requestByStatusToken(ctx, token, false)
	if err != nil {
		return nil, err
	}
	return s.requestFormLabels(s.tenantContext(ctx, req.TenantID), req)
}

func (s *Intake) requestFormLabels(ctx context.Context, req *Request) (*enrollmentNative.RequestFormLabels, error) {
	var schema *enrollmentNative.FormSchema
	if req.SchemaID != nil {
		loaded, err := s.intakeSchema(ctx, *req.SchemaID)
		if err != nil {
			return nil, fmt.Errorf("form labels: load pinned schema: %w", err)
		}
		schema = loaded
	}
	blocks, err := s.resolveSubmissionLegalBlocks(ctx, schema)
	if err != nil {
		return nil, fmt.Errorf("form labels: resolve legal blocks: %w", err)
	}
	labels := &enrollmentNative.RequestFormLabels{LegalBlocks: blocks}
	if schema == nil {
		return labels, nil
	}
	labels.Fields = schema.Fields
	seen := make(map[string]bool, len(blocks))
	for _, b := range blocks {
		seen[b.Key] = true
	}
	for _, b := range schema.LegalBlocks {
		if seen[b.Key] {
			continue
		}
		seen[b.Key] = true
		labels.LegalBlocks = append(labels.LegalBlocks, enrollmentNative.LegalBlock{
			Key: b.Key, Title: b.Title, Translations: b.PublicTranslations(),
		})
	}
	return labels, nil
}
