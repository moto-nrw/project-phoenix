package application

import (
	"context"
	"fmt"
	"log/slog"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

func (m *BookingMaterialization) ResyncOfferingSourcedTemplates(ctx context.Context, effectiveFrom calendar.Date) error {
	templates, err := m.tenantSourcedTemplates(ctx)
	if err != nil {
		return err
	}
	return m.resyncSourcedTemplateList(ctx, templates, effectiveFrom, true)
}

func (m *BookingMaterialization) tenantSourcedTemplates(ctx context.Context) ([]ports.SourcedTemplate, error) {
	templates, err := m.deps.Templates.TemplatesWithOfferingSource(ctx)
	if err != nil {
		return nil, fmt.Errorf("offering roster resync: list sourced templates: %w", err)
	}
	return templates, nil
}

// templatesSourcing lists the live templates sourcing one offering; the
// operation names the flow in the error.
func (m *BookingMaterialization) templatesSourcing(ctx context.Context, offeringID int64, operation string) ([]ports.SourcedTemplate, error) {
	templates, err := m.deps.Templates.TemplatesSourcedFrom(ctx, []int64{offeringID})
	if err != nil {
		return nil, fmt.Errorf("%s: list templates sourcing offering %d: %w", operation, offeringID, err)
	}
	return templates, nil
}

func (m *BookingMaterialization) ResyncTemplatesSourcedFromOffering(ctx context.Context, offeringID int64, effectiveFrom calendar.Date) error {
	templates, err := m.templatesSourcing(ctx, offeringID, "offering roster resync")
	if err != nil {
		return err
	}
	return m.resyncSourcedTemplateList(ctx, templates, effectiveFrom, false)
}

func (m *BookingMaterialization) DetachTemplatesSourcedFromOffering(ctx context.Context, offeringID int64, effectiveFrom calendar.Date) error {
	templates, err := m.templatesSourcing(ctx, offeringID, "offering roster detach")
	if err != nil {
		return err
	}
	for _, tmpl := range templates {
		if err := m.detachTemplateSource(ctx, tmpl, offeringID, effectiveFrom); err != nil {
			return err
		}
	}
	return nil
}

func (m *BookingMaterialization) detachTemplateSource(ctx context.Context, tmpl ports.SourcedTemplate, offeringID int64, effectiveFrom calendar.Date) error {
	remaining := removeOfferingID(tmpl.SourceCareOfferingIDs, offeringID)
	in := templateResync(tmpl, remaining, effectiveFrom)
	if len(remaining) > 0 {
		in.GradeLevels, in.SchoolClasses = tmpl.SourceGradeLevels, tmpl.SourceSchoolClasses
	}
	if err := m.ResyncTemplateOfferingRoster(ctx, in); err != nil {
		if len(remaining) == 0 || !m.catalog.deps.SourceRules.IsRejection(err) {
			return fmt.Errorf("offering roster detach: template %d: %w", tmpl.ID, err)
		}
		// A remaining source drifted invalid (e.g. it turned inactive or its
		// phase no longer fits the period pin). The delete must not be
		// blocked, and the remaining sources are valid data — only their
		// combination with the period pin failed — so they are kept and the
		// full union resync is skipped. But the DEPARTING offering's rows
		// cannot wait for a later repair: the delete behind this detach
		// cascades their request-child provenance away, after which no resync
		// can ever attribute them again. Retire them now with a drift-tolerant
		// resync scoped to the departing offering's children (see
		// retireDepartingSourcedRows).
		m.deps.Logger.Warn("offering roster detach: remaining sources invalid; keeping sources, retiring only the departing offering's contribution",
			slog.Int64("template_id", tmpl.ID),
			slog.Int64("care_offering_id", offeringID),
			slog.String("error", err.Error()),
		)
		if err := m.retireDepartingSourcedRows(ctx, in, offeringID); err != nil {
			return fmt.Errorf("offering roster detach: template %d: %w", tmpl.ID, err)
		}
	}
	if err := m.deps.Templates.UpdateTemplateOfferingSource(ctx, tmpl.ID, remaining, in.GradeLevels, in.SchoolClasses); err != nil {
		return fmt.Errorf("offering roster detach: rewrite source columns of template %d: %w", tmpl.ID, err)
	}
	return nil
}

// retireDepartingSourcedRows removes the departing offering's contribution
// from the template when the remaining sources are too drifted for the full
// union resync: the offering delete behind the detach is about to cascade
// the departing children's provenance away, so their rows must be retired
// NOW or they linger as unattributable orphans (#2147 round 11's pre-delete
// ordering). It runs the union resync against the REMAINING sources with
// drift-tolerant loading, scoped to the departing offering's children: a
// child only the departing offering fed loses their rows, and a shared
// child's rows are reshaped to exactly the remaining sources' link coverage
// — skipping shared children wholesale would leave the departing offering's
// exclusive weekday×date footprint (e.g. A's Di–Fr next to B's Mo) planned
// forever. Children of only the remaining sources are outside the scope and
// stay untouched.
func (m *BookingMaterialization) retireDepartingSourcedRows(ctx context.Context, remaining careplan.OfferingRosterResync, offeringID int64) error {
	departing, err := m.deps.Enrollment.ApprovedChildren(ctx, []int64{offeringID}, remaining.EffectiveFrom)
	if err != nil {
		return fmt.Errorf("list departing offering children: %w", err)
	}
	scope := make([]int64, 0, len(departing))
	seen := make(map[int64]bool, len(departing))
	for _, child := range departing {
		if seen[child.Link.RequestChildID] {
			continue
		}
		seen[child.Link.RequestChildID] = true
		scope = append(scope, child.Link.RequestChildID)
	}
	if len(scope) == 0 {
		return nil
	}
	slices.Sort(scope)
	remaining.ScopeRequestChildIDs = scope
	remaining.TolerateDriftedSources = true
	return m.ResyncTemplateOfferingRoster(ctx, remaining)
}

// removeOfferingID returns ids without the given offering, preserving order.
func removeOfferingID(ids []int64, offeringID int64) []int64 {
	remaining := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id != offeringID {
			remaining = append(remaining, id)
		}
	}
	if len(remaining) == 0 {
		return nil
	}
	return remaining
}

// resyncSourcedTemplateList reconciles each template's sourced roster.
// skipInvalid controls what a drifted-invalid source does: the tenant-wide
// resync lets a stored source that turned inactive contribute nothing while
// another stays valid (#5305) and skips any other invalid template with a
// warning (one broken template must not block a whole grade transition),
// while the offering-scoped resync behind phase/offering edits keeps every
// guard and propagates the refusal so the caller can reject the edit that
// would strand the template's rows (#2147 review).
func (m *BookingMaterialization) resyncSourcedTemplateList(ctx context.Context, templates []ports.SourcedTemplate, effectiveFrom calendar.Date, skipInvalid bool) error {
	for _, tmpl := range templates {
		if len(tmpl.SourceCareOfferingIDs) == 0 {
			continue
		}
		in := templateResync(tmpl, tmpl.SourceCareOfferingIDs, effectiveFrom)
		in.GradeLevels, in.SchoolClasses = tmpl.SourceGradeLevels, tmpl.SourceSchoolClasses
		in.SkipInactiveSources = skipInvalid
		err := m.ResyncTemplateOfferingRoster(ctx, in)
		switch {
		case err == nil:
		case !m.catalog.deps.SourceRules.IsRejection(err):
			return fmt.Errorf("offering roster resync: template %d: %w", tmpl.ID, err)
		case skipInvalid:
			m.logSkippedSourcedTemplate(tmpl.ID, tmpl.SourceCareOfferingIDs, "tenant-wide resync: source invalid", err)
		default:
			return fmt.Errorf("offering roster resync: template %d (%q) would lose its offering source: %w", tmpl.ID, tmpl.Name, err)
		}
	}
	return nil
}
