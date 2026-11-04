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
		return enrollment.HolidayFormDocument{}, enrollment.InvalidInput(enrollment.CodeFormAttachmentInvalid, "document", errors.New("expected a PDF of at most 10 MB"))
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
	doc, found, err := h.forms.ReadHolidayFormDocument(ctx, org, period, id, true)
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
	if _, err := readHolidayDocument(ctx, h.forms, org, period, id, false); err != nil {
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

func readHolidayDocument(ctx context.Context, forms HolidayFormStore, org, period int64, id string, content bool) (enrollment.HolidayFormDocument, error) {
	parsed, err := strconv.ParseInt(id, 10, 64)
	if err != nil || parsed <= 0 || strconv.FormatInt(parsed, 10) != id {
		return enrollment.HolidayFormDocument{}, enrollment.ErrHolidayFormDocumentNotFound
	}
	doc, found, err := forms.ReadHolidayFormDocument(ctx, org, period, id, content)
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
				return readHolidayDocument(ctx, h.deps.Forms, link.OrganizationID, link.PeriodID, id, true)
			}
		}
	}
	return enrollment.HolidayFormDocument{}, enrollment.ErrHolidayFormDocumentNotFound
}

func holidayLegalInput(path string, err error) error {
	return enrollment.InvalidInput(enrollment.CodeFormAttachmentInvalid, path, err)
}

func (h *HolidayPeriods) validateHolidayLegalBlocks(ctx context.Context, org, period int64, blocks []enrollment.FormLegalBlock) error {
	if len(blocks) > 30 {
		return holidayLegalInput("legal_blocks", errors.New("at most 30 legal blocks"))
	}
	keys := map[string]bool{}
	for i := range blocks {
		block := &blocks[i]
		prefix := fmt.Sprintf("legal_blocks.%d", i)
		if err := validateHolidayLegalBlock(block); err != nil {
			return enrollment.NestInput(err, prefix, "legal block %d", i)
		}
		if keys[block.Key] {
			return holidayLegalInput(prefix+".key", errors.New("duplicate legal block key"))
		}
		keys[block.Key] = true
		if err := h.validateHolidayLegalDocuments(ctx, org, period, block.Attachments); err != nil {
			return enrollment.NestInput(err, prefix, "legal block %d", i)
		}
		block.SortOrder = i + 1
	}
	return nil
}

// Validate the common shape without changing the school contract: holiday
// notices may require acknowledgement and their PDFs use period document IDs.
func validateHolidayLegalBlock(block *enrollment.FormLegalBlock) error {
	checked, err := holidayLegalValidationCopy(*block)
	if err != nil {
		return err
	}
	if err := checked.Validate(); err != nil {
		var rejected *enrollment.InvalidInputError
		if errors.As(err, &rejected) {
			return err
		}
		return holidayLegalInput("", err)
	}
	checked.Kind = strings.TrimSpace(block.Kind)
	if strings.TrimSpace(block.DisplayMode) == enrollment.LegalBlockDisplayModePDF {
		checked.DisplayMode = enrollment.LegalBlockDisplayModePDF
	}
	for i := range checked.Attachments {
		checked.Attachments[i].DocumentURL = ""
	}
	*block = checked
	return nil
}

func holidayLegalValidationCopy(block enrollment.FormLegalBlock) (enrollment.FormLegalBlock, error) {
	block.Source = strings.TrimSpace(block.Source)
	if block.Source == "" {
		block.Source = enrollment.LegalBlockSourceCustom
	}
	if block.Source != enrollment.LegalBlockSourceCustom {
		return block, holidayLegalInput("source", errors.New("only custom holiday legal blocks"))
	}
	if block.DocumentURL != "" {
		return block, holidayLegalInput("document_url", errors.New("use a period document ID"))
	}
	if strings.TrimSpace(block.Kind) == enrollment.LegalBlockKindNotice && block.Required {
		block.Kind = enrollment.LegalBlockKindTerms
	}
	if strings.TrimSpace(block.DisplayMode) == enrollment.LegalBlockDisplayModePDF {
		if len(block.Attachments) == 0 {
			return block, holidayLegalInput("attachments", errors.New("PDF mode requires a document"))
		}
		block.DisplayMode = enrollment.LegalBlockDisplayModeText
	}
	block.Attachments = append([]enrollment.FormAttachment(nil), block.Attachments...)
	for j := range block.Attachments {
		if block.Attachments[j].DocumentURL != "" {
			return block, holidayLegalInput(fmt.Sprintf("attachments.%d.document_url", j), errors.New("use a period document ID"))
		}
		block.Attachments[j].DocumentURL = block.Attachments[j].DocumentID
	}
	return block, nil
}

func (h *HolidayPeriods) validateHolidayLegalDocuments(ctx context.Context, org, period int64, attachments []enrollment.FormAttachment) error {
	for j, attachment := range attachments {
		if _, err := readHolidayDocument(ctx, h.forms, org, period, attachment.DocumentID, false); err != nil {
			if errors.Is(err, enrollment.ErrHolidayFormDocumentNotFound) {
				return holidayLegalInput(fmt.Sprintf("attachments.%d.document_id", j), err)
			}
			return err
		}
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
