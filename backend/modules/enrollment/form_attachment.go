package enrollment

import (
	"fmt"
	"strings"
)

// FormAttachment is a named PDF in an immutable form version. Replacing a
// document creates a new URL; saved versions retain their original reference.
type FormAttachment struct {
	Label       string `json:"label"`
	DocumentURL string `json:"document_url"`
}

func ValidateFormAttachments(attachments []FormAttachment) error {
	seen := make(map[string]bool, len(attachments))
	for i := range attachments {
		attachment := &attachments[i]
		attachment.Label = strings.TrimSpace(attachment.Label)
		attachment.DocumentURL = strings.TrimSpace(attachment.DocumentURL)
		if attachment.Label == "" || attachment.DocumentURL == "" {
			return fmt.Errorf("attachment %d requires a label and document URL", i)
		}
		if seen[attachment.DocumentURL] {
			return fmt.Errorf("attachment %d repeats a document URL", i)
		}
		seen[attachment.DocumentURL] = true
	}
	return nil
}

// PublicFormAttachments copies the version's references for anonymous readers.
func PublicFormAttachments(attachments []FormAttachment) []FormAttachment {
	if len(attachments) == 0 {
		return nil
	}
	result := append([]FormAttachment(nil), attachments...)
	for i := range result {
		result[i].DocumentURL = PublicEnrollmentLegalDocumentURL(result[i].DocumentURL)
	}
	return result
}
