package application

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"time"

	enrollmentNative "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// Renewal terms (#6456). A renewal opens a new care phase, and the Träger
// concludes a new contract for each child of it (ADR 0153 section 2). The
// acceptance of the earlier phase does not carry over: the rollover copies no
// evidence entry, and silence on an opt-out renewal accepts nothing (ADR 0088
// section 3). The parents accept the required blocks of the form in force for
// the new phase explicitly, and that acceptance is a new evidence entry with
// the form version, the checksums and the time.

// renewalForm is the form in force for the phase of a renewal with the blocks
// the parents must tick.
type renewalForm struct {
	schema      *enrollmentNative.FormSchema
	blocks      []enrollmentNative.LegalBlock
	fingerprint string
}

// id is the form version, 0 for the tenant-wide blocks.
func (f renewalForm) id() int64 {
	if f.schema == nil {
		return 0
	}
	return f.schema.ID
}

// requiredLegalBlocks keeps the blocks a form demands a tick for.
func requiredLegalBlocks(blocks []enrollmentNative.LegalBlock) []enrollmentNative.LegalBlock {
	required := make([]enrollmentNative.LegalBlock, 0, len(blocks))
	for _, block := range blocks {
		if block.Required {
			required = append(required, block)
		}
	}
	return required
}

// renewalBlocks are the required blocks of a form: the template's enabled
// blocks, otherwise the tenant-wide ones, as a submission resolves them.
func renewalBlocks(schema *enrollmentNative.FormSchema, tenant func() (enrollmentNative.LegalTexts, error)) ([]enrollmentNative.LegalBlock, error) {
	if schema != nil && len(schema.LegalBlocks) > 0 {
		if blocks := buildTemplateLegalBlocks(schema.LegalBlocks); len(blocks) > 0 {
			return requiredLegalBlocks(blocks), nil
		}
	}
	texts, err := tenant()
	if err != nil {
		return nil, err
	}
	return requiredLegalBlocks(texts.Blocks), nil
}

// awaitsRenewal reports whether a child of the request still waits for the
// school's decision on its renewal.
func awaitsRenewal(children []*enrollmentNative.RequestChild) bool {
	return slices.ContainsFunc(children, func(child *enrollmentNative.RequestChild) bool {
		switch child.Status {
		case enrollmentNative.ChildStatusPendingRenewal, enrollmentNative.ChildStatusAutoRenewed:
			return true
		case enrollmentNative.ChildStatusSubmitted, enrollmentNative.ChildStatusUnderReview, enrollmentNative.ChildStatusWaitlisted:
			return child.RolloverSourceChildID != nil
		default:
			return false
		}
	})
}

// renewalFormFor resolves the form in force for the phase now. A renewal pins
// the form of its phase at the rollover; a version the school published
// since is the one the parents accept.
func (s *Intake) renewalFormFor(ctx context.Context, phaseID int64) (renewalForm, error) {
	phase, err := s.intakePhase(ctx, phaseID)
	if err != nil {
		return renewalForm{}, fmt.Errorf("renewal terms: load phase: %w", err)
	}
	schema, err := s.resolveSubmissionSchema(ctx, phase)
	if err != nil {
		return renewalForm{}, fmt.Errorf("renewal terms: load form: %w", err)
	}
	blocks, err := renewalBlocks(schema, func() (enrollmentNative.LegalTexts, error) { return s.legalTexts(ctx) })
	if err != nil {
		return renewalForm{}, fmt.Errorf("renewal terms: resolve legal blocks: %w", err)
	}
	if s.deps.Fingerprint == nil {
		return renewalForm{}, fmt.Errorf("renewal terms: fingerprint not configured")
	}
	encoded, err := json.Marshal(blocks)
	if err != nil {
		return renewalForm{}, fmt.Errorf("renewal terms: encode legal blocks: %w", err)
	}
	return renewalForm{schema: schema, blocks: blocks, fingerprint: s.deps.Fingerprint(encoded)}, nil
}

// renewalTerms are the terms the status page asks the parents to accept;
// nil while no child waits for its renewal.
func (s *Intake) renewalTerms(ctx context.Context, req *enrollmentNative.Request, children []*enrollmentNative.RequestChild) (*enrollmentNative.RenewalTerms, error) {
	if !awaitsRenewal(children) {
		return nil, nil
	}
	form, err := s.renewalFormFor(ctx, req.PhaseID)
	if err != nil {
		return nil, err
	}
	return &enrollmentNative.RenewalTerms{
		FormSchemaID:     form.id(),
		TermsFingerprint: form.fingerprint,
		Blocks:           form.blocks,
		ConfirmedAt:      renewalConfirmedAt(req.LegalBlocksSnapshot, form, s.deps.Fingerprint),
	}, nil
}

// renewalConfirmedAt is the time of the latest evidence entry when it
// accepted every required block of this form version in its current wording;
// nil otherwise. Only the latest entry counts, as for the contract.
func renewalConfirmedAt(entries []enrollmentNative.LegalBlocksSnapshotEntry, form renewalForm, fingerprint Fingerprint) *time.Time {
	if len(entries) == 0 {
		return nil
	}
	entry := entries[len(entries)-1]
	if entry.FormSchemaID != form.id() || entry.DisplayedFormSchemaID != form.id() {
		return nil
	}
	var flags map[string]any
	if json.Unmarshal(entry.ConsentFlags, &flags) != nil {
		return nil
	}
	for _, block := range form.blocks {
		if accepted, ok := flags[block.Key].(bool); !ok || !accepted {
			return nil
		}
		if !slices.ContainsFunc(entry.Blocks, func(shown enrollmentNative.LegalBlockSnapshot) bool {
			return shown.Key == block.Key && shown.Label == block.Label && shown.Title == block.Title &&
				slices.Equal(shown.Attachments, block.Attachments) && sameLegalText(shown, block.Text, fingerprint)
		}) {
			return nil
		}
	}
	at := entry.SnapshotAt
	return &at
}

func sameLegalText(shown enrollmentNative.LegalBlockSnapshot, text string, fingerprint Fingerprint) bool {
	if fingerprint == nil {
		return shown.Text == text
	}
	return shown.TextSHA256 != "" && shown.TextSHA256 == fingerprint([]byte(text))
}

// acceptRenewalTerms validates the parents' answer against the form in force
// and appends the evidence entry to the locked request. The caller writes
// the request.
func (s *Intake) acceptRenewalTerms(ctx context.Context, req *enrollmentNative.Request, confirmation enrollmentNative.RenewalConfirmation) error {
	form, err := s.renewalFormFor(ctx, req.PhaseID)
	if err != nil {
		return err
	}
	// A page that showed another version proves no acceptance of the one
	// in force; the parents reload and accept that one.
	if confirmation.DisplayedFormSchemaID != form.id() {
		return enrollmentNative.InvalidInput(enrollmentNative.CodeRenewalTermsChanged, "form_schema_id", enrollmentNative.ErrRenewalTermsChanged)
	}
	if confirmation.TermsFingerprint != form.fingerprint {
		return enrollmentNative.InvalidInput(enrollmentNative.CodeRenewalTermsChanged, "terms_fingerprint", enrollmentNative.ErrRenewalTermsChanged)
	}
	flags := make(map[string]any, len(form.blocks))
	for _, block := range form.blocks {
		accepted, ok := formValue(confirmation.ConsentFlags, block.Key).(bool)
		if !ok || !accepted {
			return enrollmentNative.InvalidInput(enrollmentNative.CodeRenewalTermsRequired, "consent_flags."+block.Key,
				fmt.Errorf("%w: %s", enrollmentNative.ErrRenewalTermsRequired, block.Key))
		}
		flags[block.Key] = true
	}
	if renewalConfirmedAt(req.LegalBlocksSnapshot, form, s.deps.Fingerprint) != nil {
		return nil
	}
	entry, err := legalBlocksSnapshotEntry(form.blocks, flags, formsOf(form.schema, confirmation.DisplayedFormSchemaID), time.Now(), s.deps.Fingerprint)
	if err != nil {
		return fmt.Errorf("renewal terms: snapshot: %w", err)
	}
	req.LegalBlocksSnapshot = append(req.LegalBlocksSnapshot, entry)
	// The request keeps the answers of the blocks this confirmation did not
	// ask, such as an optional consent of a full form edit.
	merged := formMap(req.ConsentFlags)
	if merged == nil {
		merged = map[string]any{}
	}
	maps.Copy(merged, flags)
	req.ConsentFlags = formValues(merged)
	return nil
}
