package tenantexport

import (
	"encoding/json"
	"errors"
	"strings"
)

type exportLegalAttachment struct {
	DocumentURL string `json:"document_url"`
}

type exportLegalField struct {
	Attachments []exportLegalAttachment `json:"attachments"`
}

type exportLegalBlock struct {
	DocumentURL string                  `json:"document_url"`
	Attachments []exportLegalAttachment `json:"attachments"`
}

// Only typed document references authorize originals, never arbitrary form text.
func (r *exportRun) collectLegalOriginals(table string, fields map[string]json.RawMessage) error {
	switch table {
	case "enrollment.form_schemas":
		return r.collectFormLegalOriginals(fields)
	case "enrollment.requests":
		raw := fields["legal_blocks_snapshot"]
		if len(raw) == 0 {
			return nil
		}
		var snapshots []struct {
			Blocks []exportLegalField `json:"blocks"`
		}
		if err := json.Unmarshal(raw, &snapshots); err != nil {
			return exportTableError(table)
		}
		for _, snapshot := range snapshots {
			for _, block := range snapshot.Blocks {
				if err := r.collectLegalAttachments(block.Attachments); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (r *exportRun) collectFormLegalOriginals(fields map[string]json.RawMessage) error {
	if raw := fields["legal_blocks"]; len(raw) != 0 {
		var blocks []exportLegalBlock
		if err := json.Unmarshal(raw, &blocks); err != nil {
			return exportTableError("enrollment.form_schemas")
		}
		if err := r.collectLegalBlocks(blocks); err != nil {
			return err
		}
	}
	if raw := fields["fields"]; len(raw) != 0 {
		var formFields []exportLegalField
		if err := json.Unmarshal(raw, &formFields); err != nil {
			return exportTableError("enrollment.form_schemas")
		}
		for _, field := range formFields {
			if err := r.collectLegalAttachments(field.Attachments); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *exportRun) collectLegalBlocks(blocks []exportLegalBlock) error {
	for _, block := range blocks {
		if err := r.collectLegalURL(block.DocumentURL); err != nil {
			return err
		}
		if err := r.collectLegalAttachments(block.Attachments); err != nil {
			return err
		}
	}
	return nil
}

func (r *exportRun) collectLegalAttachments(attachments []exportLegalAttachment) error {
	for _, attachment := range attachments {
		if err := r.collectLegalURL(attachment.DocumentURL); err != nil {
			return err
		}
	}
	return nil
}

func (r *exportRun) collectLegalURL(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	// The enrollment public routes serve the same stored keys without a fetch.
	key, ok := strings.CutPrefix(value, "/uploads/")
	if !ok {
		key, ok = strings.CutPrefix(value, "/api/public/")
	}
	kind, _, _ := strings.Cut(key, "/")
	if !ok || (kind != "enrollment-form-legal-documents" && kind != "enrollment-legal-documents") ||
		strings.Count(key, "/") != 1 || strings.Contains(key, "..") ||
		!strings.HasSuffix(strings.ToLower(key), ".pdf") || !r.allowedObject(key, false) {
		return errors.New("tenant export: invalid or foreign legal document reference")
	}
	r.references[key] = true
	return nil
}
