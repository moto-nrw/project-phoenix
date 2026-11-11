package application

import (
	"context"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// These batch commands deduplicate templates and children across every changed
// offering, so shared consumers are reconciled once against the final catalog.
func (m *BookingMaterialization) ResyncTemplatesSourcedFromOfferings(ctx context.Context, ids []int64, from calendar.Date) error {
	templates, err := m.templatesSourcing(ctx, ids, "offering batch")
	if err != nil {
		return err
	}
	if len(templates) == 0 {
		return nil
	}
	materialization, batch, err := m.sourcedRosterBatch(ctx, templates, from)
	if err != nil {
		return err
	}
	if err := materialization.resyncSourcedTemplateList(ctx, templates, from, false); err != nil {
		return err
	}
	return batch.flush(ctx)
}

func (m *BookingMaterialization) ReconcileOfferingPickupForOfferings(ctx context.Context, ids []int64) error {
	children, err := m.deps.Enrollment.ApprovedChildren(ctx, ids, m.todayDate())
	if err != nil {
		return err
	}
	students := []int64{}
	seen := map[int64]bool{}
	for _, child := range children {
		if !seen[child.StudentID] {
			students = append(students, child.StudentID)
			seen[child.StudentID] = true
		}
	}
	return m.ReconcileOfferingPickupForStudents(ctx, students)
}

func (m *BookingMaterialization) DetachTemplatesSourcedFromOfferings(ctx context.Context, ids []int64, from calendar.Date) error {
	templates, err := m.templatesSourcing(ctx, ids, "offering batch")
	if err != nil {
		return err
	}
	if len(templates) == 0 {
		return nil
	}
	materialization, batch, err := m.sourcedRosterBatch(ctx, templates, from)
	if err != nil {
		return err
	}
	removed := map[int64]bool{}
	for _, id := range ids {
		removed[id] = true
	}
	children, err := materialization.deps.Enrollment.ApprovedChildren(ctx, ids, from)
	if err != nil {
		return err
	}
	sources := []ports.TemplateSourceChange{}
	for _, tmpl := range templates {
		change, err := materialization.detachTraegerTemplate(ctx, tmpl, removed, children, from)
		if err != nil {
			return err
		}
		sources = append(sources, change)
	}
	if err := batch.flush(ctx); err != nil {
		return err
	}
	return m.deps.Templates.UpdateTemplateOfferingSources(ctx, sources)
}

func (m *BookingMaterialization) detachTraegerTemplate(ctx context.Context, tmpl ports.SourcedTemplate, removed map[int64]bool, children []ports.ApprovedOfferingChild, from calendar.Date) (ports.TemplateSourceChange, error) {
	remaining := []int64{}
	departing := map[int64]bool{}
	for _, id := range tmpl.SourceCareOfferingIDs {
		if removed[id] {
			departing[id] = true
		} else {
			remaining = append(remaining, id)
		}
	}
	in := templateResync(tmpl, remaining, from)
	if len(remaining) > 0 {
		in.GradeLevels, in.SchoolClasses = tmpl.SourceGradeLevels, tmpl.SourceSchoolClasses
	}
	if err := m.ResyncTemplateOfferingRoster(ctx, in); err != nil {
		if len(remaining) == 0 || !m.catalog.deps.SourceRules.IsRejection(err) {
			return ports.TemplateSourceChange{}, err
		}
		if err := m.retireTraegerSourceChildren(ctx, in, departing, children); err != nil {
			return ports.TemplateSourceChange{}, err
		}
	}
	return ports.TemplateSourceChange{TemplateID: tmpl.ID, OfferingIDs: remaining, GradeLevels: in.GradeLevels, SchoolClasses: in.SchoolClasses}, nil
}

func (m *BookingMaterialization) retireTraegerSourceChildren(ctx context.Context, in careplan.OfferingRosterResync, departing map[int64]bool, children []ports.ApprovedOfferingChild) error {
	scope := []int64{}
	seen := map[int64]bool{}
	for _, child := range children {
		if departing[child.Link.CareOfferingID] && !seen[child.Link.RequestChildID] {
			scope = append(scope, child.Link.RequestChildID)
			seen[child.Link.RequestChildID] = true
		}
	}
	if len(scope) == 0 {
		return nil
	}
	in.ScopeRequestChildIDs, in.TolerateDriftedSources = scope, true
	if err := m.ResyncTemplateOfferingRoster(ctx, in); err != nil {
		return fmt.Errorf("detach batch: %w", err)
	}
	return nil
}
