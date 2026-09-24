package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sort"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
)

// Kursanmeldung durch Eltern (#3075, ADR 0012). A course is an AG a family
// reaches through a care offering bound to it, either through the legacy
// ActivityGroupID on the offering or through a Regeltermin sourcing the
// offering (#2137). There is no second request path: the catalog is the
// offering catalog filtered on offerings with an AG, a course request is an
// offering change request adding the course to today's booking, and the
// existing review decides it. The waiting position is read, never stored.

func (s *OfferingChanges) courseRequestsEnabled(ctx context.Context) (bool, error) {
	if err := s.changesEnabled(ctx); err != nil {
		if errors.Is(err, careplan.ErrOfferingChangeDisabled) || errors.Is(err, careplan.ErrCareOfferingsDisabled) {
			return false, nil
		}
		return false, err
	}
	enabled, err := s.deps.Settings.CourseRequestsEnabled(ctx)
	if err != nil {
		return false, fmt.Errorf("course request: resolve enabled setting: %w", err)
	}
	return enabled, nil
}

// CourseCatalog lists the school's courses with the child's state.
func (s *OfferingChanges) CourseCatalog(ctx context.Context, studentID, accountID int64) (*careplan.CourseCatalog, error) {
	enabled, err := s.courseRequestsEnabled(ctx)
	if err != nil {
		return nil, err
	}
	if !enabled {
		return &careplan.CourseCatalog{DisabledReason: careplan.CourseRequestsReasonSchoolOff, Items: []careplan.CourseCatalogItem{}}, nil
	}
	catalog, err := s.Catalog(ctx, studentID)
	if err != nil {
		if errors.Is(err, careplan.ErrOfferingChangeNoEnrollment) {
			return &careplan.CourseCatalog{DisabledReason: careplan.CourseRequestsReasonNoEnrollment, Items: []careplan.CourseCatalogItem{}}, nil
		}
		return nil, err
	}
	view := &careplan.CourseCatalog{
		Enabled: true, PhaseName: catalog.PhaseName, EffectiveFrom: catalog.EarliestEffectiveFrom,
		Items: []careplan.CourseCatalogItem{},
	}
	courses, groupsByOffering, err := s.courseItems(ctx, catalog)
	if err != nil {
		return nil, err
	}
	if len(courses) == 0 {
		view.Enabled = false
		view.DisabledReason = careplan.CourseRequestsReasonNoCourses
		return view, nil
	}
	pending, isCourseRequest, err := s.pendingCourseRequest(ctx, studentID, catalog, courses)
	if err != nil {
		return nil, err
	}
	switch {
	case pending != nil && isCourseRequest:
		view.PendingRequestID = pending.ID
		view.PendingSubmittedBySelf = pending.SubmittedBy == accountID
	case pending != nil:
		view.OtherRequestPending = true
	}
	if err := s.applyCourseCapacity(ctx, catalog, courses, groupsByOffering, pending); err != nil {
		return nil, err
	}
	view.Items = courses
	return view, nil
}

// courseItems keeps the offerings that lead to an AG, and answers with the
// AGs each of them feeds. Both link shapes count: the legacy one on the
// offering and the one a Regeltermin declares itself (#2137).
func (s *OfferingChanges) courseItems(ctx context.Context, catalog *careplan.OfferingChangeCatalog) ([]careplan.CourseCatalogItem, map[int64][]ports.CourseGroup, error) {
	offeringIDs := make([]int64, 0, len(catalog.Items))
	for _, item := range catalog.Items {
		if item.IsActive {
			offeringIDs = append(offeringIDs, item.OfferingID)
		}
	}
	groups, err := s.courseGroups(ctx, catalog, offeringIDs)
	if err != nil {
		return nil, nil, err
	}
	return courseItemsFromGroups(catalog, groups), groups, nil
}

// courseItemsFromGroups maps an active offering that feeds at least one AG
// to a course, sorted by name. The course view has no weekday picker, so a
// parent-choice offering stays on the ordinary offering-change path.
func courseItemsFromGroups(catalog *careplan.OfferingChangeCatalog, groupsByOffering map[int64][]ports.CourseGroup) []careplan.CourseCatalogItem {
	items := make([]careplan.CourseCatalogItem, 0, len(catalog.Items))
	for _, item := range catalog.Items {
		groups := groupsByOffering[item.OfferingID]
		if !item.IsActive || len(groups) == 0 || item.DaysOfWeekMode == daysOfWeekModeParentChoice {
			continue
		}
		items = append(items, careplan.CourseCatalogItem{
			OfferingID: item.OfferingID, ActivityGroupID: groups[0].ID, Name: item.Name,
			Description: item.Description, AvailableDays: courseScheduledDays(groups), Booked: item.Selected,
		})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items
}

// courseScheduledDays derives the displayed weekdays from the course
// templates: one offering can feed several target-filtered templates.
func courseScheduledDays(groups []ports.CourseGroup) []string {
	dayByWeekday := [...]string{"", "mon", "tue", "wed", "thu", "fri", "sat", "sun"}
	seen := make(map[int]bool)
	for _, group := range groups {
		for _, weekday := range group.ScheduledWeekdays {
			if weekday > 0 && weekday < len(dayByWeekday) {
				seen[weekday] = true
			}
		}
	}
	days := make([]string, 0, len(seen))
	for weekday := 1; weekday < len(dayByWeekday); weekday++ {
		if seen[weekday] {
			days = append(days, dayByWeekday[weekday])
		}
	}
	return days
}

// courseGroups maps each offering to the AGs it feeds, in a stable order.
// Only activity groups count: a care Regeltermin fed by an offering is
// planning for that care and does not belong under Kurse.
func (s *OfferingChanges) courseGroups(ctx context.Context, catalog *careplan.OfferingChangeCatalog, offeringIDs []int64) (map[int64][]ports.CourseGroup, error) {
	groups := make(map[int64][]ports.CourseGroup, len(offeringIDs))
	if len(offeringIDs) == 0 {
		return groups, nil
	}
	wanted := make(map[int64]bool, len(offeringIDs))
	for _, offeringID := range offeringIDs {
		wanted[offeringID] = true
	}
	references := make([]ports.CourseOfferingReference, 0, len(offeringIDs))
	for _, item := range catalog.Items {
		if item.IsActive && wanted[item.OfferingID] {
			references = append(references, ports.CourseOfferingReference{OfferingID: item.OfferingID, ActivityGroupID: item.ActivityGroupID})
		}
	}
	projected, err := s.deps.Planning.CourseGroupsForOfferings(ctx, references, catalog.EarliestEffectiveFrom)
	if err != nil {
		return nil, fmt.Errorf("course request: list course groups: %w", err)
	}
	for offeringID, list := range projected {
		groups[offeringID] = activeTargetGroups(list, catalog)
	}
	return groups, nil
}

// activeTargetGroups keeps the distinct active groups that admit the
// catalog's target child, in id order.
func activeTargetGroups(list []ports.CourseGroup, catalog *careplan.OfferingChangeCatalog) []ports.CourseGroup {
	var groups []ports.CourseGroup
	seen := make(map[int64]bool, len(list))
	for _, group := range list {
		if seen[group.ID] || !group.Active || !courseGroupMatchesTarget(group, catalog) {
			continue
		}
		seen[group.ID] = true
		groups = append(groups, group)
	}
	slices.SortFunc(groups, func(a, b ports.CourseGroup) int { return int(a.ID - b.ID) })
	return groups
}

func courseGroupMatchesTarget(group ports.CourseGroup, catalog *careplan.OfferingChangeCatalog) bool {
	if len(group.SourceGradeLevels) > 0 {
		if catalog.TargetGradeLevel == nil || !slices.Contains(group.SourceGradeLevels, int(*catalog.TargetGradeLevel)) {
			return false
		}
	}
	if len(group.SourceSchoolClasses) == 0 {
		return true
	}
	wanted := strings.ToLower(strings.TrimSpace(catalog.TargetSchoolClass))
	if wanted == "" {
		return false
	}
	for _, schoolClass := range group.SourceSchoolClasses {
		if strings.ToLower(strings.TrimSpace(schoolClass)) == wanted {
			return true
		}
	}
	return false
}

func courseTargetCatalog(child *ports.OfferingChangeChild) *careplan.OfferingChangeCatalog {
	catalog := &careplan.OfferingChangeCatalog{}
	if child == nil {
		return catalog
	}
	catalog.TargetGradeLevel = child.TargetGradeLevel
	if child.TargetSchoolClass != nil {
		catalog.TargetSchoolClass = *child.TargetSchoolClass
	}
	return catalog
}

// markCourseDiffEntries flags direct additions of a course the child may
// actually attend. Both link shapes are resolved in one bounded batch.
func (s *OfferingChanges) markCourseDiffEntries(
	ctx context.Context,
	entries []careplan.OfferingChangeDiffEntry,
	offerings map[int64]*careplan.CareOffering,
	catalog *careplan.OfferingChangeCatalog,
	requested []careplan.OfferingChangeSelection,
) error {
	if len(entries) == 0 || catalog == nil {
		return nil
	}
	references, requestedIDs := courseDiffReferences(entries, offerings, requested)
	if len(references) == 0 {
		return nil
	}
	groups, err := s.deps.Planning.CourseGroupsForOfferings(ctx, references, catalog.EarliestEffectiveFrom)
	if err != nil {
		return fmt.Errorf("offering change: mark course diff lines: %w", err)
	}
	markCourseDiffEntriesForGroups(entries, groups, catalog, requestedIDs)
	return nil
}

func addsRequestedOffering(entry careplan.OfferingChangeDiffEntry, requestedIDs map[int64]bool) bool {
	return entry.OldState == "not_booked" && entry.NewState == "booked" && requestedIDs[entry.OfferingID]
}

func courseDiffReferences(
	entries []careplan.OfferingChangeDiffEntry,
	offerings map[int64]*careplan.CareOffering,
	requested []careplan.OfferingChangeSelection,
) ([]ports.CourseOfferingReference, map[int64]bool) {
	requestedIDs := make(map[int64]bool, len(requested))
	for _, selected := range requested {
		requestedIDs[selected.OfferingID] = true
	}
	references := make([]ports.CourseOfferingReference, 0, len(entries))
	for _, entry := range entries {
		if !addsRequestedOffering(entry, requestedIDs) {
			continue
		}
		if offering := offerings[entry.OfferingID]; offering != nil {
			references = append(references, ports.CourseOfferingReference{OfferingID: offering.ID, ActivityGroupID: offering.ActivityGroupID})
		}
	}
	return references, requestedIDs
}

func markCourseDiffEntriesForGroups(
	entries []careplan.OfferingChangeDiffEntry,
	groups map[int64][]ports.CourseGroup,
	catalog *careplan.OfferingChangeCatalog,
	requestedIDs map[int64]bool,
) {
	for i := range entries {
		entry := &entries[i]
		if !addsRequestedOffering(*entry, requestedIDs) {
			continue
		}
		entry.IsCourse = slices.ContainsFunc(groups[entry.OfferingID], func(group ports.CourseGroup) bool {
			return group.Active && courseGroupMatchesTarget(group, catalog)
		})
	}
}

// CreateCourseRequest asks the OGS for one course: an ordinary offering
// change request with the child's current booking plus that course.
func (s *OfferingChanges) CreateCourseRequest(ctx context.Context, input careplan.CreateCourseRequestInput) (*careplan.OfferingChangeRequest, error) {
	enabled, err := s.courseRequestsEnabled(ctx)
	if err != nil {
		return nil, err
	}
	if !enabled {
		return nil, careplan.ErrCourseRequestsDisabled
	}
	catalog, err := s.Catalog(ctx, input.StudentID)
	if err != nil {
		return nil, err
	}
	courses, _, err := s.courseItems(ctx, catalog)
	if err != nil {
		return nil, err
	}
	course, err := courseCatalogEntry(courses, input.OfferingID)
	if err != nil {
		return nil, err
	}
	created, err := s.SubmitOfferingChange(ctx, careplan.CreateOfferingChangeInput{
		StudentID: input.StudentID, AccountID: input.AccountID, Selections: courseSelectionsWith(catalog, course),
		EffectiveFrom: catalog.EarliestEffectiveFrom, Note: strings.TrimSpace(input.Note),
	})
	if err != nil {
		return nil, err
	}
	s.logger().Info("course request created",
		slog.Int64("student_id", input.StudentID),
		slog.Int64("care_offering_id", input.OfferingID),
		slog.String("effective_from", catalog.EarliestEffectiveFrom.String()),
	)
	return created, nil
}

func courseCatalogEntry(courses []careplan.CourseCatalogItem, offeringID int64) (*careplan.CourseCatalogItem, error) {
	for i := range courses {
		course := &courses[i]
		if course.OfferingID != offeringID {
			continue
		}
		if course.Booked {
			return nil, careplan.ErrCourseAlreadyBooked
		}
		return course, nil
	}
	return nil, careplan.ErrCourseNotFound
}

// courseSelectionsWith is the child's current booking plus the new course:
// the request payload is a complete desired selection, never a delta.
// Courses have fixed days.
func courseSelectionsWith(catalog *careplan.OfferingChangeCatalog, course *careplan.CourseCatalogItem) []careplan.OfferingChangeSelection {
	selections := make([]careplan.OfferingChangeSelection, 0, len(catalog.Items)+1)
	for _, item := range catalog.Items {
		if !item.Selected || item.Automatic {
			continue
		}
		current := careplan.OfferingChangeSelection{OfferingID: item.OfferingID}
		if item.DaysOfWeekMode == daysOfWeekModeParentChoice {
			current.SelectedDays = append([]string(nil), item.SelectedDays...)
		}
		selections = append(selections, current)
	}
	return append(selections, careplan.OfferingChangeSelection{OfferingID: course.OfferingID})
}

// WithdrawCourseRequest takes back the caller's own open course request. A
// foreign request stays reported as missing, never as forbidden.
func (s *OfferingChanges) WithdrawCourseRequest(ctx context.Context, requestID, accountID, studentID int64) error {
	enabled, err := s.courseRequestsEnabled(ctx)
	if err != nil {
		return err
	}
	if !enabled {
		return careplan.ErrCourseRequestsDisabled
	}
	row, err := s.deps.Rows.FindForUpdate(ctx, requestID)
	if err != nil {
		return err
	}
	if row.StudentID != studentID || row.SubmittedBy != accountID {
		return careplan.ErrOfferingChangeNotFound
	}
	if offeringChangeTerminal(row) {
		return careplan.ErrOfferingChangeNotPending
	}
	if err := s.assertOwnCourseRequest(ctx, row, studentID); err != nil {
		return err
	}
	if err := s.deps.Rows.Decide(ctx, requestID, careplan.OfferingChangeWithdrawn, nil, nil, false); err != nil {
		return err
	}
	s.logger().Info("course request withdrawn",
		slog.Int64("student_id", studentID),
		slog.Int64("request_id", requestID),
	)
	return nil
}

// assertOwnCourseRequest refuses to withdraw a request that changes care
// offerings too: that change is edited where it was made.
func (s *OfferingChanges) assertOwnCourseRequest(ctx context.Context, row careplan.OfferingChangeRequest, studentID int64) error {
	catalog, err := s.Catalog(ctx, studentID)
	if err != nil {
		return err
	}
	courses, _, err := s.courseItems(ctx, catalog)
	if err != nil {
		return err
	}
	added, err := addedCourseIDs(row, courses)
	if err != nil {
		return err
	}
	pure, err := isCourseOnlyRequest(row, catalog, added)
	if err != nil {
		return err
	}
	if !pure {
		return careplan.ErrCourseRequestNotOwn
	}
	return nil
}
