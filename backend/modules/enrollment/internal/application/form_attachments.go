package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

func (s *FormSchemas) normalizeAttachments(ctx context.Context, attachments []enrollment.FormAttachment) ([]enrollment.FormAttachment, error) {
	if len(attachments) == 0 {
		return nil, nil
	}
	var tenantID int64
	if s.runtime.TenantID != nil {
		tenantID = s.runtime.TenantID(ctx)
	}
	result := append([]enrollment.FormAttachment(nil), attachments...)
	for i := range result {
		documentURL, ok := enrollment.NormalizeEnrollmentFormLegalDocumentURL(result[i].DocumentURL, tenantID)
		if !ok {
			return nil, enrollment.InvalidInput(enrollment.CodeFormAttachmentInvalid, fmt.Sprintf("attachments.%d.document_url", i), fmt.Errorf("attachment %d has an invalid tenant PDF document URL", i))
		}
		result[i].DocumentURL = documentURL
	}
	return result, nil
}

func (s *FormSchemas) normalizeFieldAttachments(ctx context.Context, fields []enrollment.FormField) ([]enrollment.FormField, error) {
	if len(fields) == 0 {
		return fields, nil
	}
	result := append([]enrollment.FormField(nil), fields...)
	for i := range result {
		attachments, err := s.normalizeAttachments(ctx, result[i].Attachments)
		if err != nil {
			return nil, enrollment.NestInput(err, fmt.Sprintf("fields.%d", i), "field %d", i)
		}
		result[i].Attachments = attachments
	}
	return result, nil
}
