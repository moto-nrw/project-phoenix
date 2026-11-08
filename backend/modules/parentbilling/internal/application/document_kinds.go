package application

import (
	"context"
	"errors"
	"fmt"
	"strings"

	validation "github.com/go-ozzo/ozzo-validation"

	"github.com/moto-nrw/project-phoenix/modules/documentrendering/templates"
	"github.com/moto-nrw/project-phoenix/modules/parentbilling"
)

// The kind of document per service (#4688, ADR 0119 addendum 2026-11-08).
// The Abrechnung chooses it; the release of a run freezes it on every
// document it issues and refuses fee notices while the template of the fee
// notice has no legal remedy.

// DocumentKindStore keeps the kind of document of every service of a Träger
// in its Träger transaction.
type DocumentKindStore interface {
	// DocumentKinds reads the chosen kind of every service that has one,
	// with the time of the choice.
	DocumentKinds(ctx context.Context, organizationID int64) (map[string]parentbilling.ServiceDocumentKind, error)
	SetDocumentKind(ctx context.Context, organizationID int64, service, documentKind string) error
}

// documentKindServices are the services in the order the Abrechnung sees
// them.
var documentKindServices = []string{parentbilling.ServiceHoliday, parentbilling.ServiceCare}

// DocumentKinds serves parentbilling.BillingDocumentKinds.
type DocumentKinds struct {
	gate documentKindGate
}

var _ parentbilling.BillingDocumentKinds = (*DocumentKinds)(nil)

// NewDocumentKinds builds the choice of the kinds of document over the
// templates the root binds.
func NewDocumentKinds(store DocumentKindStore, templates parentbilling.BillingDocumentTemplates) *DocumentKinds {
	return &DocumentKinds{gate: newDocumentKindGate(store, templates)}
}

// ListDocumentKinds reads the kind of every service, its choices and
// whether the template of the fee notice has its legal remedy. It reads no
// personal data.
func (d *DocumentKinds) ListDocumentKinds(ctx context.Context, organizationID int64) (parentbilling.DocumentKindOverview, error) {
	if organizationID <= 0 {
		return parentbilling.DocumentKindOverview{}, errTraegerRequired
	}
	chosen, err := d.gate.store.DocumentKinds(ctx, organizationID)
	if err != nil {
		return parentbilling.DocumentKindOverview{}, fmt.Errorf("parent billing: read document kinds: %w", err)
	}
	overview := parentbilling.DocumentKindOverview{Services: make([]parentbilling.ServiceDocumentKind, 0, len(documentKindServices))}
	for _, service := range documentKindServices {
		current, ok := chosen[service]
		if !ok || !parentbilling.ValidDocumentKind(service, current.DocumentKind) {
			current = parentbilling.ServiceDocumentKind{DocumentKind: parentbilling.DefaultDocumentKind(service)}
		}
		current.Service, current.Choices = service, parentbilling.DocumentKindChoices(service)
		overview.Services = append(overview.Services, current)
	}
	template, err := d.gate.templates.DocumentTemplate(ctx, organizationID, parentbilling.DocumentKindFeeNotice)
	if err != nil {
		return parentbilling.DocumentKindOverview{}, fmt.Errorf("parent billing: read the template of the fee notice: %w", err)
	}
	overview.LegalRemedySet = !templates.LacksLegalRemedy(templates.KindFeeNotice, letterTemplate(template, templates.KindFeeNotice))
	overview.FeeNoticeTemplateVersion = template.Version
	return overview, nil
}

// SetDocumentKind chooses the kind a service issues from the next release
// on. Issued documents keep theirs. Choosing the fee notice is allowed
// before its legal remedy is written; the release refuses until then.
func (d *DocumentKinds) SetDocumentKind(ctx context.Context, organizationID int64, service, documentKind string) (parentbilling.DocumentKindOverview, error) {
	if organizationID <= 0 {
		return parentbilling.DocumentKindOverview{}, errTraegerRequired
	}
	service, documentKind = strings.TrimSpace(service), strings.TrimSpace(documentKind)
	switch {
	case parentbilling.DocumentKindChoices(service) == nil:
		return parentbilling.DocumentKindOverview{}, validation.Errors{"service": errors.New("unknown service")}
	case !parentbilling.ValidDocumentKind(service, documentKind):
		return parentbilling.DocumentKindOverview{}, validation.Errors{"document_kind": parentbilling.ErrDocumentKindInvalid}
	}
	if err := d.gate.store.SetDocumentKind(ctx, organizationID, service, documentKind); err != nil {
		return parentbilling.DocumentKindOverview{}, fmt.Errorf("parent billing: set document kind: %w", err)
	}
	return d.ListDocumentKinds(ctx, organizationID)
}

// documentKindGate reads the kind a service issues now and checks before a
// release that a fee notice has its legal remedy. The holiday and the
// monthly runs share it.
type documentKindGate struct {
	store     DocumentKindStore
	templates parentbilling.BillingDocumentTemplates
}

func newDocumentKindGate(store DocumentKindStore, templates parentbilling.BillingDocumentTemplates) documentKindGate {
	if store == nil || templates == nil {
		panic("parent billing: the document kind store and the document templates are required")
	}
	return documentKindGate{store: store, templates: templates}
}

// current is the kind the service issues now and whether a release would
// refuse it for a missing legal remedy.
func (g documentKindGate) current(ctx context.Context, organizationID int64, service string) (parentbilling.DocumentKindState, error) {
	state, _, err := g.reviewed(ctx, organizationID, service)
	return state, err
}

// reviewed is current with what the review of a run shows of the legal
// remedy: the version and the text of the template of the fee notice, empty
// for any other kind. The fingerprint of the review covers it (#4688), so a
// legal remedy changed between review and release makes the release refuse.
func (g documentKindGate) reviewed(ctx context.Context, organizationID int64, service string) (parentbilling.DocumentKindState, string, error) {
	chosen, err := g.store.DocumentKinds(ctx, organizationID)
	if err != nil {
		return parentbilling.DocumentKindState{}, "", fmt.Errorf("parent billing: read document kinds: %w", err)
	}
	kind := parentbilling.DefaultDocumentKind(service)
	if choice, ok := chosen[service]; ok && parentbilling.ValidDocumentKind(service, choice.DocumentKind) {
		kind = choice.DocumentKind
	}
	state := parentbilling.DocumentKindState{DocumentKind: kind}
	if kind != parentbilling.DocumentKindFeeNotice {
		return state, "", nil
	}
	template, err := g.templates.DocumentTemplate(ctx, organizationID, kind)
	if err != nil {
		return parentbilling.DocumentKindState{}, "", fmt.Errorf("parent billing: read the template of the fee notice: %w", err)
	}
	state.LegalRemedyMissing = templates.LacksLegalRemedy(templates.KindFeeNotice, letterTemplate(template, templates.KindFeeNotice))
	return state, fmt.Sprintf("%d:%s", template.Version, template.LegalRemedy), nil
}

// issuable is the kind a release of the service freezes on its documents;
// a fee notice without legal remedy is ErrLegalRemedyMissing.
func (g documentKindGate) issuable(ctx context.Context, organizationID int64, service string) (string, error) {
	state, err := g.current(ctx, organizationID, service)
	if err != nil {
		return "", err
	}
	if state.LegalRemedyMissing {
		return "", parentbilling.ErrLegalRemedyMissing
	}
	return state.DocumentKind, nil
}
