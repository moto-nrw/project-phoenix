package application

import (
	"context"
	"fmt"

	enrollmentNative "github.com/moto-ogs/project-phoenix/modules/enrollment"
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

func (s *Intake) labelledRequest(ctx context.Context, requestID int64) (*enrollmentNative.Request, error) {
	req, err := s.deps.Requests.RequestByID(ctx, requestID, false)
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

func (s *Intake) requestFormLabels(ctx context.Context, req *enrollmentNative.Request) (*enrollmentNative.RequestFormLabels, error) {
	snapshot, hasSnapshot := latestLegalBlocksSnapshot(req)
	var schema *enrollmentNative.FormSchema
	if schemaID := formLabelsSchemaID(req, snapshot); schemaID != nil {
		loaded, err := s.intakeSchema(ctx, *schemaID)
		if err != nil {
			return nil, fmt.Errorf("form labels: load displayed schema: %w", err)
		}
		schema = loaded
	}
	labels := &enrollmentNative.RequestFormLabels{}
	if hasSnapshot {
		// The snapshot is immutable evidence of what the parent saw. In
		// particular, tenant-wide legal texts cannot be reconstructed after a
		// school changes them.
		labels.LegalBlocks = snapshotLegalBlocks(snapshot.Blocks)
	} else {
		blocks, err := s.resolveSubmissionLegalBlocks(ctx, schema)
		if err != nil {
			return nil, fmt.Errorf("form labels: resolve legal blocks: %w", err)
		}
		labels.LegalBlocks = blocks
	}
	if schema == nil {
		return labels, nil
	}
	labels.Fields = schema.Fields
	if hasSnapshot {
		return labels, nil
	}
	seen := make(map[string]bool, len(labels.LegalBlocks))
	for _, b := range labels.LegalBlocks {
		seen[b.Key] = true
	}
	for _, b := range schema.LegalBlocks {
		if !b.Enabled || seen[b.Key] {
			continue
		}
		seen[b.Key] = true
		labels.LegalBlocks = append(labels.LegalBlocks, enrollmentNative.LegalBlock{
			Key: b.Key, Title: b.Title, Translations: b.PublicTranslations(),
		})
	}
	return labels, nil
}

func latestLegalBlocksSnapshot(req *enrollmentNative.Request) (enrollmentNative.LegalBlocksSnapshotEntry, bool) {
	if req == nil || len(req.LegalBlocksSnapshot) == 0 {
		return enrollmentNative.LegalBlocksSnapshotEntry{}, false
	}
	return req.LegalBlocksSnapshot[len(req.LegalBlocksSnapshot)-1], true
}

// formLabelsSchemaID selects the form version the parent saw. Earlier
// snapshots did not record that version, so their request schema remains the
// best available fallback.
func formLabelsSchemaID(req *enrollmentNative.Request, snapshot enrollmentNative.LegalBlocksSnapshotEntry) *int64 {
	if snapshot.DisplayedFormSchemaID != 0 {
		id := snapshot.DisplayedFormSchemaID
		return &id
	}
	if req == nil {
		return nil
	}
	return req.SchemaID
}

func snapshotLegalBlocks(blocks []enrollmentNative.LegalBlockSnapshot) []enrollmentNative.LegalBlock {
	if len(blocks) == 0 {
		return nil
	}
	labels := make([]enrollmentNative.LegalBlock, 0, len(blocks))
	for _, block := range blocks {
		labels = append(labels, enrollmentNative.LegalBlock{
			Key: block.Key, Title: block.Title,
			Translations: snapshotTranslations(block.Translations),
		})
	}
	return labels
}

func snapshotTranslations(values map[string]map[string]string) enrollmentNative.Translations {
	if len(values) == 0 {
		return nil
	}
	translations := make(enrollmentNative.Translations, len(values))
	for locale, attributes := range values {
		translated := make(map[string]enrollmentNative.TranslatedText, len(attributes))
		for attribute, text := range attributes {
			translated[attribute] = enrollmentNative.TranslatedText{Text: text}
		}
		translations[locale] = translated
	}
	return translations
}
