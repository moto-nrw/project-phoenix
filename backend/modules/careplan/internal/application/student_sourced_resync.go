package application

import (
	"context"
	"fmt"
	"slices"

	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// ResyncStudentSourcedRosters reconciles the offering-sourced rows of the
// given students, and only theirs, with the current source configuration from
// effectiveFrom on (#5235). A cancelled or replaced care exit puts the
// removed rows back verbatim; their period may no longer be the one the
// template's instances materialize under, so the care-exit workflow re-plans
// the children right after the restore.
//
// The scope per template is the students' request children that either hold
// one of its source offerings or already carry a row on it. Every write goes
// through the ordinary resync, so unchanged rows keep their id, legacy-fed
// and manual rows stay protected and only the students' future occurrences
// are reconciled. A stored source that turned inactive contributes nothing
// while another stays valid (#5305); a template whose source drifted invalid
// otherwise is skipped with a warning: the restore it follows is deliberately
// forgiving. It runs in the
// caller's tenant unit, whose holder took the tenant recurrence lock.
func (m *BookingMaterialization) ResyncStudentSourcedRosters(ctx context.Context, studentIDs []int64, effectiveFrom calendar.Date) error {
	if len(studentIDs) == 0 {
		return nil
	}
	if effectiveFrom.IsZero() {
		return m.catalog.deps.SourceRules.Reject("effective_from is required")
	}
	templates, err := m.tenantSourcedTemplates(ctx)
	if err != nil {
		return err
	}
	scopes, err := m.studentSourcedScopes(ctx, studentIDs, templates, effectiveFrom)
	if err != nil {
		return err
	}
	for _, tmpl := range templates {
		scope := scopes[tmpl.ID]
		if len(scope) == 0 || len(tmpl.SourceCareOfferingIDs) == 0 {
			continue
		}
		in := templateResync(tmpl, tmpl.SourceCareOfferingIDs, effectiveFrom)
		in.GradeLevels, in.SchoolClasses = tmpl.SourceGradeLevels, tmpl.SourceSchoolClasses
		in.ScopeRequestChildIDs = scope
		in.SkipInactiveSources = true
		if err := m.ResyncTemplateOfferingRoster(ctx, in); err != nil {
			if m.catalog.deps.SourceRules.IsRejection(err) {
				m.logSkippedSourcedTemplate(tmpl.ID, tmpl.SourceCareOfferingIDs, "student resync: source invalid", err)
				continue
			}
			return fmt.Errorf("student roster resync: template %d: %w", tmpl.ID, err)
		}
	}
	return nil
}

// studentSourcedScopes maps each sourced template to the students' request
// children it has to reconcile, sorted.
func (m *BookingMaterialization) studentSourcedScopes(
	ctx context.Context, studentIDs []int64, templates []ports.SourcedTemplate, effectiveFrom calendar.Date,
) (map[int64][]int64, error) {
	students := make(map[int64]bool, len(studentIDs))
	for _, id := range studentIDs {
		students[id] = true
	}
	scopes := templateChildScopes{}
	childrenByOffering, err := m.studentRequestChildrenByOffering(ctx, students, templates, effectiveFrom)
	if err != nil {
		return nil, err
	}
	for _, tmpl := range templates {
		for _, offeringID := range tmpl.SourceCareOfferingIDs {
			scopes.add(tmpl.ID, childrenByOffering[offeringID]...)
		}
	}
	if err := m.scopeRestoredRows(ctx, students, templates, scopes); err != nil {
		return nil, err
	}
	return scopes.sorted(), nil
}

// scopeRestoredRows adds the request children of the students' rows on
// sourced templates. A restored row can sit on a template whose sources no
// longer include the child's offering; scoping it in lets the resync retire
// it.
func (m *BookingMaterialization) scopeRestoredRows(
	ctx context.Context, students map[int64]bool, templates []ports.SourcedTemplate, scopes templateChildScopes,
) error {
	sourced := make(map[int64]bool, len(templates))
	for _, tmpl := range templates {
		sourced[tmpl.ID] = true
	}
	for _, studentID := range sortedStudentIDs(students) {
		rows, err := m.deps.Rosters.StudentEnrollments(ctx, studentID)
		if err != nil {
			return fmt.Errorf("student roster resync: load student enrollments: %w", err)
		}
		for _, row := range rows {
			if row.EnrollmentRequestChildID != nil && sourced[row.ActivityGroupID] {
				scopes.add(row.ActivityGroupID, *row.EnrollmentRequestChildID)
			}
		}
	}
	return nil
}

// templateChildScopes collects request children per template.
type templateChildScopes map[int64]map[int64]bool

func (s templateChildScopes) add(templateID int64, requestChildIDs ...int64) {
	if len(requestChildIDs) == 0 {
		return
	}
	if s[templateID] == nil {
		s[templateID] = make(map[int64]bool)
	}
	for _, id := range requestChildIDs {
		s[templateID][id] = true
	}
}

func (s templateChildScopes) sorted() map[int64][]int64 {
	result := make(map[int64][]int64, len(s))
	for templateID, children := range s {
		ids := make([]int64, 0, len(children))
		for childID := range children {
			ids = append(ids, childID)
		}
		slices.Sort(ids)
		result[templateID] = ids
	}
	return result
}

// studentRequestChildrenByOffering lists, per source offering, the request
// children through which the students hold it on or after effectiveFrom.
func (m *BookingMaterialization) studentRequestChildrenByOffering(
	ctx context.Context, students map[int64]bool, templates []ports.SourcedTemplate, effectiveFrom calendar.Date,
) (map[int64][]int64, error) {
	var offeringIDs []int64
	for _, tmpl := range templates {
		offeringIDs = append(offeringIDs, tmpl.SourceCareOfferingIDs...)
	}
	slices.Sort(offeringIDs)
	offeringIDs = slices.Compact(offeringIDs)
	result := make(map[int64][]int64)
	if len(offeringIDs) == 0 {
		return result, nil
	}
	children, err := m.deps.Enrollment.ApprovedChildren(ctx, offeringIDs, effectiveFrom)
	if err != nil {
		return nil, fmt.Errorf("student roster resync: list approved children: %w", err)
	}
	for _, child := range children {
		if students[child.StudentID] {
			result[child.Link.CareOfferingID] = append(result[child.Link.CareOfferingID], child.Link.RequestChildID)
		}
	}
	return result, nil
}

func sortedStudentIDs(students map[int64]bool) []int64 {
	ids := make([]int64, 0, len(students))
	for id := range students {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	return ids
}
