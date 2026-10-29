package application

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation"
	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// Save and delete hold the same period lock. Thus a draft cannot disappear
// between reference validation and version insertion. Stored versions never
// change, so every historic reference remains protected.
func (h *HolidayPeriods) UploadHolidayFormDocument(ctx context.Context, org, period, actor int64, filename string, content []byte) (enrollment.HolidayFormDocument, error) {
	if _, err := foundHolidayPeriod(h.store.FindHolidayPeriod(ctx, org, period, true)); err != nil {
		return enrollment.HolidayFormDocument{}, err
	}
	if len(content) > 10<<20 || !bytes.HasPrefix(content, []byte("%PDF-")) || !strings.EqualFold(filepath.Ext(filename), ".pdf") {
		return enrollment.HolidayFormDocument{}, enrollment.InvalidInput("enrollment.form_attachment_invalid", "document", errors.New("expected a PDF of at most 10 MB"))
	}
	return h.forms.InsertHolidayFormDocument(ctx, org, period, actor, filepath.Base(filename), content)
}

func (h *HolidayPeriods) HolidayFormDocument(ctx context.Context, org, period int64, id string) (enrollment.HolidayFormDocument, error) {
	if _, err := foundHolidayPeriod(h.store.FindHolidayPeriod(ctx, org, period, false)); err != nil {
		return enrollment.HolidayFormDocument{}, err
	}
	parsed, err := strconv.ParseInt(id, 10, 64)
	if err != nil || parsed <= 0 || strconv.FormatInt(parsed, 10) != id {
		return enrollment.HolidayFormDocument{}, enrollment.ErrHolidayFormDocumentNotFound
	}
	doc, found, err := h.forms.ReadHolidayFormDocument(ctx, org, period, id)
	if err != nil {
		return enrollment.HolidayFormDocument{}, err
	}
	if !found {
		return enrollment.HolidayFormDocument{}, enrollment.ErrHolidayFormDocumentNotFound
	}
	return doc, nil
}

func (h *HolidayPeriods) DeleteHolidayFormDocument(ctx context.Context, org, period int64, id string) error {
	if _, err := foundHolidayPeriod(h.store.FindHolidayPeriod(ctx, org, period, true)); err != nil {
		return err
	}
	if _, err := readHolidayDocument(ctx, h.forms, org, period, id); err != nil {
		return err
	}
	deleted, err := h.forms.DeleteHolidayFormDocument(ctx, org, period, id)
	if err != nil {
		return err
	}
	if !deleted {
		return enrollment.ErrHolidayFormDocumentReferenced
	}
	return nil
}

func readHolidayDocument(ctx context.Context, forms HolidayFormStore, org, period int64, id string) (enrollment.HolidayFormDocument, error) {
	parsed, err := strconv.ParseInt(id, 10, 64)
	if err != nil || parsed <= 0 || strconv.FormatInt(parsed, 10) != id {
		return enrollment.HolidayFormDocument{}, enrollment.ErrHolidayFormDocumentNotFound
	}
	doc, found, err := forms.ReadHolidayFormDocument(ctx, org, period, id)
	if err != nil {
		return enrollment.HolidayFormDocument{}, err
	}
	if !found {
		return enrollment.HolidayFormDocument{}, enrollment.ErrHolidayFormDocumentNotFound
	}
	return doc, nil
}

// A public link grants only the documents referenced by its current form,
// never drafts or a previous version. The Träger is resolved from the link.
func (h *HolidayRegistrations) PublicHolidayFormDocument(ctx context.Context, link enrollment.HolidayRegistrationLink, id string) (enrollment.HolidayFormDocument, error) {
	offer, err := h.HolidayRegistrationOffer(ctx, link)
	if err != nil {
		return enrollment.HolidayFormDocument{}, err
	}
	for _, block := range offer.Form.LegalBlocks {
		if !block.Enabled {
			continue
		}
		for _, attachment := range block.Attachments {
			if attachment.DocumentID == id {
				return readHolidayDocument(ctx, h.deps.Forms, link.OrganizationID, link.PeriodID, id)
			}
		}
	}
	return enrollment.HolidayFormDocument{}, enrollment.ErrHolidayFormDocumentNotFound
}

func (h *HolidayPeriods) validateHolidayLegalBlocks(ctx context.Context, org, period int64, blocks []enrollment.FormLegalBlock) error {
	invalid := func(path string, err error) error {
		return enrollment.InvalidInput("enrollment.form_attachment_invalid", path, err)
	}
	if len(blocks) > 30 {
		return invalid("legal_blocks", errors.New("at most 30 legal blocks"))
	}
	keys := map[string]bool{}
	for i := range blocks {
		b := &blocks[i]
		prefix := fmt.Sprintf("legal_blocks.%d", i)
		b.Source = strings.TrimSpace(b.Source)
		if b.Source == "" {
			b.Source = enrollment.LegalBlockSourceCustom
		}
		if b.Source != enrollment.LegalBlockSourceCustom {
			return invalid(prefix+".source", errors.New("only custom holiday legal blocks"))
		}
		// Holiday notices may require acknowledgement, unlike school notices.
		// PDF mode uses period-owned attachments, not the school AGB URL.
		kind, mode := strings.TrimSpace(b.Kind), strings.TrimSpace(b.DisplayMode)
		if kind == enrollment.LegalBlockKindNotice && b.Required {
			b.Kind = enrollment.LegalBlockKindTerms
		}
		if mode == enrollment.LegalBlockDisplayModePDF {
			if len(b.Attachments) == 0 {
				return invalid(prefix+".attachments", errors.New("PDF mode requires a document"))
			}
			b.DisplayMode = enrollment.LegalBlockDisplayModeText
		}
		attachments := append([]enrollment.FormAttachment(nil), b.Attachments...)
		for j := range b.Attachments {
			if b.Attachments[j].DocumentURL != "" {
				return invalid(fmt.Sprintf("%s.attachments.%d.document_url", prefix, j), errors.New("use a period document ID"))
			}
			b.Attachments[j].DocumentURL = b.Attachments[j].DocumentID
		}
		if err := b.Validate(); err != nil {
			var rejected *enrollment.InvalidInputError
			if errors.As(err, &rejected) {
				return enrollment.NestInput(err, prefix, "legal block %d", i)
			}
			return invalid(prefix, err)
		}
		for j := range b.Attachments {
			b.Attachments[j].DocumentURL = attachments[j].DocumentURL
		}
		b.Kind = kind
		if mode == enrollment.LegalBlockDisplayModePDF {
			b.DisplayMode = mode
		}
		if keys[b.Key] {
			return invalid(prefix+".key", errors.New("duplicate legal block key"))
		}
		keys[b.Key] = true
		if b.DocumentURL != "" {
			return invalid(prefix+".document_url", errors.New("use a period document ID"))
		}
		for j := range b.Attachments {
			a := &b.Attachments[j]
			if _, err := readHolidayDocument(ctx, h.forms, org, period, a.DocumentID); err != nil {
				if errors.Is(err, enrollment.ErrHolidayFormDocumentNotFound) {
					return invalid(fmt.Sprintf("%s.attachments.%d.document_id", prefix, j), err)
				}
				return err
			}
		}
		b.SortOrder = i + 1
	}
	return nil
}

func validateHolidayLegalAcceptances(form enrollment.HolidayPeriodForm, accepted map[string]bool) error {
	known := map[string]bool{}
	errs := validation.Errors{}
	for _, b := range form.LegalBlocks {
		if !b.Enabled {
			continue
		}
		known[b.Key] = true
		if b.Required && !accepted[b.Key] {
			errs[b.Key] = errors.New("acknowledgement is required")
		}
	}
	for key := range accepted {
		if !known[key] {
			errs[key] = errors.New("unknown legal block")
		}
	}
	if len(errs) > 0 {
		return validation.Errors{"legal_acceptances": errs}
	}
	return nil
}
