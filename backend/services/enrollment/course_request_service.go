// Kursanmeldung durch Eltern (#3075, Leistungskatalog Schleswig-Holstein 4.3,
// ADR 0012).
//
// Ein Kurs ist eine AG, und eine AG erreichen Eltern über den Angebotsweg: ein
// Betreuungsangebot, das an einer AG hängt — entweder über die alte
// ActivityGroupID am Angebot oder darüber, dass ein Regeltermin das Angebot als
// Quelle führt (#2137). Deshalb gibt es hier KEINE zweite Antrags- und
// Entscheidungsstrecke. Alles unten baut eine Kursansicht auf der bestehenden
// Änderungsanfrage auf:
//
//   - der Katalog ist der Angebotskatalog, gefiltert auf Angebote mit AG,
//   - eine Kursanfrage ist eine Änderungsanfrage, deren Auswahl die heutige
//     Buchung plus den gewünschten Kurs ist,
//   - entschieden wird sie in der vorhandenen Freigabeansicht, und die
//     Freigabe schreibt wie bisher activities.student_enrollments.
//
// Der Wartelistenplatz wird gelesen, nicht gespeichert: er ergibt sich aus der
// freien Kapazität und der Reihenfolge der offenen Anfragen. Eine gespeicherte
// Position wäre schon falsch, sobald eine ältere Anfrage entschieden wird.
package enrollment

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sort"
	"strings"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentOwner "github.com/moto-nrw/project-phoenix/modules/enrollment"
)

// CourseProjectionReader is the consumer-owned port for the timetable facts
// that course requests need. Its production adapter reaches the tenant-safe
// timetable projection; it never exposes timetable repositories to enrollment.
type CourseProjectionReader interface {
	CourseGroupsForOfferings(context.Context, []enrollmentModels.CourseOfferingReference) (map[int64][]enrollmentModels.CourseGroup, error)
	LockCourseGroups(context.Context, []int64) ([]enrollmentModels.CourseGroup, error)
	CountActiveCourseEnrollments(context.Context, []int64, timezone.Date, int64) (map[int64]int, error)
}

// CourseOfferingReference and CourseGroup keep the timetable projection's
// enrollment read contract at the workflow boundary. Composition must use
// these aliases instead of reaching into the legacy enrollment model package.
type CourseOfferingReference = enrollmentModels.CourseOfferingReference
type CourseGroup = enrollmentModels.CourseGroup

func (s *offeringChangeRequestService) courseProjection() (CourseProjectionReader, error) {
	reader, ok := s.ImpactRepo.(CourseProjectionReader)
	if !ok {
		return nil, fmt.Errorf("course request: timetable projection is required")
	}
	return reader, nil
}

var (
	// ErrCourseRequestsDisabled means the school has parent course requests
	// switched off (or the offering-change machinery they run on).
	ErrCourseRequestsDisabled = errors.New("enrollment: parent course requests are disabled")
	// ErrCourseNotFound means the id is not a course the child may request:
	// unknown, not bound to an AG, or not part of the child's care period.
	ErrCourseNotFound = errors.New("enrollment: course not found")
	// ErrCourseAlreadyBooked means the child already holds that course.
	ErrCourseAlreadyBooked = errors.New("enrollment: course is already booked")
	// ErrCourseRequestNotOwn means the pending request is not a course request
	// the caller submitted, so it must not be withdrawn here.
	ErrCourseRequestNotOwn = errors.New("enrollment: not an own course request")
)

// Reasons a school has no course requests. Wire-stable identifiers; the German
// copy lives in the parents portal.
const (
	CourseRequestsReasonSchoolOff    = "school_disabled"
	CourseRequestsReasonNoEnrollment = "no_enrollment"
	CourseRequestsReasonNoCourses    = "no_courses"
	CourseRequestsReasonNoPermission = "no_permission"
)

// CourseCatalogItem is one course a family can see: what it is, whether the
// child is in it, and — when it is full — where the child stands.
type CourseCatalogItem struct {
	OfferingID      int64
	ActivityGroupID int64
	Name            string
	Description     string
	// AvailableDays are the canonical day keys the course runs on.
	AvailableDays []string
	// Capacity is the effective participant limit: the smaller of the AG's
	// Teilnehmergrenze and the offering's capacity. Nil means unlimited, and
	// then FreeSlots is nil too.
	Capacity  *int
	FreeSlots *int
	// Booked marks a course the child already attends.
	Booked bool
	// Requested marks a course the child's open request asks for.
	Requested bool
	// Waitlisted is true for a requested course with no free slot left.
	Waitlisted bool
	// WaitlistPosition is 1-based and only set together with Waitlisted.
	WaitlistPosition int
}

// CourseCatalog is everything the parents portal needs for the Kurse section.
type CourseCatalog struct {
	// Enabled is false when the school does not offer parent course requests;
	// DisabledReason then names why, and Items stays empty.
	Enabled        bool
	DisabledReason string
	PhaseName      string
	// EffectiveFrom is the date a new course request would take effect on —
	// the school's notice period, never earlier than the care period.
	EffectiveFrom timezone.Date
	// PendingRequestID is the child's open COURSE request, 0 when there is
	// none. PendingSubmittedBySelf says whether the reading guardian may
	// withdraw it.
	PendingRequestID       int64
	PendingSubmittedBySelf bool
	// OtherRequestPending marks an open request that changes care offerings
	// rather than courses. A child has at most one open request, so it blocks
	// a new course request — and the section has to say so instead of offering
	// a button that cannot work.
	OtherRequestPending bool
	Items               []CourseCatalogItem
}

// CreateCourseRequestInput is one parent asking for one course.
type CreateCourseRequestInput struct {
	StudentID  int64
	AccountID  int64
	OfferingID int64
	Note       string
}

// CourseRequestService is the parents-portal course surface. Every method runs
// inside a tenant transaction opened by its caller.
type CourseRequestService interface {
	// CourseCatalog lists the school's courses with the child's state.
	CourseCatalog(ctx context.Context, studentID, accountID int64) (*CourseCatalog, error)
	// CreateCourseRequest asks the OGS for one course. It stores an ordinary
	// offering change request: the child's current booking plus that course.
	CreateCourseRequest(ctx context.Context, input CreateCourseRequestInput) (*enrollmentModels.OfferingChangeRequest, error)
	// WithdrawCourseRequest takes back the caller's own open course request.
	WithdrawCourseRequest(ctx context.Context, requestID, accountID, studentID int64) error
}

func (s *offeringChangeRequestService) courseRequestsEnabled(ctx context.Context) (bool, error) {
	if s.Settings == nil {
		return false, nil
	}
	if err := s.changesEnabled(ctx); err != nil {
		if errors.Is(err, ErrOfferingChangeDisabled) || errors.Is(err, ErrCareOfferingsDisabled) {
			return false, nil
		}
		return false, err
	}
	enabled, err := s.Settings.ResolveBool(ctx, configModel.KeyEnrollmentParentCourseRequestsEnabled)
	if err != nil {
		return false, fmt.Errorf("course request: resolve enabled setting: %w", err)
	}
	return enabled, nil
}

func (s *offeringChangeRequestService) CourseCatalog(
	ctx context.Context,
	studentID, accountID int64,
) (*CourseCatalog, error) {
	enabled, err := s.courseRequestsEnabled(ctx)
	if err != nil {
		return nil, err
	}
	if !enabled {
		return &CourseCatalog{DisabledReason: CourseRequestsReasonSchoolOff, Items: []CourseCatalogItem{}}, nil
	}
	catalog, err := s.Catalog(ctx, studentID)
	if err != nil {
		if errors.Is(err, ErrOfferingChangeNoEnrollment) {
			return &CourseCatalog{DisabledReason: CourseRequestsReasonNoEnrollment, Items: []CourseCatalogItem{}}, nil
		}
		return nil, err
	}
	view := &CourseCatalog{
		Enabled:       true,
		PhaseName:     catalog.PhaseName,
		EffectiveFrom: catalog.EarliestEffectiveFrom,
		Items:         []CourseCatalogItem{},
	}
	courses, groupsByOffering, err := s.courseItems(ctx, catalog)
	if err != nil {
		return nil, err
	}
	if len(courses) == 0 {
		view.Enabled = false
		view.DisabledReason = CourseRequestsReasonNoCourses
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

// courseItems keeps the offerings that lead to an AG, and answers with the AGs
// each of them feeds. There are two link shapes, and both count:
//
//   - the legacy one on the offering (CareOffering.ActivityGroupID),
//   - the one a Regeltermin declares itself (source_care_offering_ids, #2137),
//     which is how new schools are set up and which can split one offering
//     across several filtered Regeltermine.
//
// Reading only the first would hide every course a school configured the
// current way.
func (s *offeringChangeRequestService) courseItems(
	ctx context.Context,
	catalog *OfferingChangeCatalog,
) ([]CourseCatalogItem, map[int64][]enrollmentModels.CourseGroup, error) {
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
	return courseItemsFromGroups(catalog, courseGroupIDs(groups)), groups, nil
}

// courseItemsFromGroups is the mapping step on its own: an active offering
// that feeds at least one AG becomes a course, sorted by name.
func courseItemsFromGroups(
	catalog *OfferingChangeCatalog,
	groupsByOffering map[int64][]int64,
) []CourseCatalogItem {
	items := make([]CourseCatalogItem, 0, len(catalog.Items))
	for _, item := range catalog.Items {
		groupIDs := groupsByOffering[item.OfferingID]
		if !item.IsActive || len(groupIDs) == 0 {
			continue
		}
		items = append(items, CourseCatalogItem{
			OfferingID:      item.OfferingID,
			ActivityGroupID: groupIDs[0],
			Name:            item.Name,
			Description:     item.Description,
			AvailableDays:   append([]string(nil), item.AvailableDays...),
			Booked:          item.Selected,
		})
	}
	sort.SliceStable(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items
}

// courseGroups maps each offering to the AGs it feeds, in a stable order.
//
// Only groups of type "activity" count. A Regeltermin of type "care" fed by an
// offering is planning for that care — the demo's Mittagessen is one — and
// listing it under Kurse would put lunch next to the football AG.
func (s *offeringChangeRequestService) courseGroups(
	ctx context.Context,
	catalog *OfferingChangeCatalog,
	offeringIDs []int64,
) (map[int64][]enrollmentModels.CourseGroup, error) {
	groups := make(map[int64][]enrollmentModels.CourseGroup, len(offeringIDs))
	if len(offeringIDs) == 0 {
		return groups, nil
	}
	projection, err := s.courseProjection()
	if err != nil {
		return nil, err
	}
	wanted := make(map[int64]bool, len(offeringIDs))
	for _, offeringID := range offeringIDs {
		wanted[offeringID] = true
	}
	references := make([]enrollmentModels.CourseOfferingReference, 0, len(offeringIDs))
	for _, item := range catalog.Items {
		if item.IsActive && wanted[item.OfferingID] {
			references = append(references, enrollmentModels.CourseOfferingReference{
				OfferingID: item.OfferingID, ActivityGroupID: item.ActivityGroupID,
			})
		}
	}
	projected, err := projection.CourseGroupsForOfferings(ctx, references)
	if err != nil {
		return nil, fmt.Errorf("course request: list course groups: %w", err)
	}
	for offeringID, list := range projected {
		seen := make(map[int64]bool, len(list))
		for _, group := range list {
			if seen[group.ID] || !group.Active || !courseGroupMatchesTarget(group, catalog) {
				continue
			}
			seen[group.ID] = true
			groups[offeringID] = append(groups[offeringID], group)
		}
		slices.SortFunc(groups[offeringID], func(a, b enrollmentModels.CourseGroup) int {
			return int(a.ID - b.ID)
		})
	}
	return groups, nil
}

func courseGroupMatchesTarget(group enrollmentModels.CourseGroup, catalog *OfferingChangeCatalog) bool {
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

func courseTargetCatalog(child *RequestChild) *OfferingChangeCatalog {
	catalog := &OfferingChangeCatalog{}
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
// actually attend. Both the legacy offering link and the Regeltermin source
// link are resolved in bounded batch queries and then checked with the
// catalog's course predicate.
func (s *offeringChangeRequestService) markCourseDiffEntries(
	ctx context.Context,
	entries []OfferingChangeDiffEntry,
	offerings map[int64]*enrollmentModels.CareOffering,
	catalog *OfferingChangeCatalog,
	requested []OfferingChangeSelection,
) error {
	if len(entries) == 0 || catalog == nil {
		return nil
	}
	projection, err := s.courseProjection()
	if err != nil {
		return err
	}
	requestedIDs := make(map[int64]bool, len(requested))
	for _, selection := range requested {
		requestedIDs[selection.OfferingID] = true
	}
	references := make([]enrollmentModels.CourseOfferingReference, 0, len(entries))
	for _, entry := range entries {
		if entry.OldState != "not_booked" || entry.NewState != "booked" || !requestedIDs[entry.OfferingID] {
			continue
		}
		offering := offerings[entry.OfferingID]
		if offering == nil {
			continue
		}
		references = append(references, enrollmentModels.CourseOfferingReference{
			OfferingID: offering.ID, ActivityGroupID: offering.ActivityGroupID,
		})
	}
	if len(references) == 0 {
		return nil
	}
	groups, err := projection.CourseGroupsForOfferings(ctx, references)
	if err != nil {
		return fmt.Errorf("offering change: mark course diff lines: %w", err)
	}
	markCourseDiffEntriesForGroups(entries, groups, catalog, requestedIDs)
	return nil
}

func markCourseDiffEntriesForGroups(
	entries []OfferingChangeDiffEntry,
	groups map[int64][]enrollmentModels.CourseGroup,
	catalog *OfferingChangeCatalog,
	requestedIDs map[int64]bool,
) {
	for i := range entries {
		entry := &entries[i]
		if entry.OldState != "not_booked" || entry.NewState != "booked" || !requestedIDs[entry.OfferingID] {
			continue
		}
		for _, group := range groups[entry.OfferingID] {
			if group.Active && courseGroupMatchesTarget(group, catalog) {
				entry.IsCourse = true
				break
			}
		}
	}
}

// courseGroupIDs is the id-only projection the catalog and the create path
// pass around.
func courseGroupIDs(groups map[int64][]enrollmentModels.CourseGroup) map[int64][]int64 {
	ids := make(map[int64][]int64, len(groups))
	for offeringID, list := range groups {
		for _, group := range list {
			ids[offeringID] = append(ids[offeringID], group.ID)
		}
	}
	return ids
}

// pendingCourseRequest returns the courses the child's open request asks for.
// A pending request that changes only care offerings is not a course request:
// it blocks a new one (one open request per child), but it is not withdrawable
// from the Kurse section.
func (s *offeringChangeRequestService) pendingCourseRequest(
	ctx context.Context,
	studentID int64,
	catalog *OfferingChangeCatalog,
	courses []CourseCatalogItem,
) (*enrollmentModels.OfferingChangeRequest, bool, error) {
	pending, err := s.ChangeRepo.GetPendingForStudent(ctx, studentID)
	if err != nil {
		return nil, false, fmt.Errorf("course request: get pending: %w", err)
	}
	if pending == nil {
		return nil, false, nil
	}
	added, err := addedCourseIDs(pending, courses)
	if err != nil {
		return nil, false, err
	}
	if len(added) == 0 {
		return pending, false, nil
	}
	pure, err := isCourseOnlyRequest(pending, catalog, added)
	if err != nil {
		return nil, false, err
	}
	if !pure {
		return pending, false, nil
	}
	requested := make(map[int64]bool, len(added))
	for _, offeringID := range added {
		requested[offeringID] = true
	}
	for i := range courses {
		courses[i].Requested = requested[courses[i].OfferingID]
	}
	return pending, len(added) > 0, nil
}

// isCourseOnlyRequest verifies that a complete selection changes nothing but
// adding courses. A course withdrawal acts on the whole offering-change row,
// so mixed care and course requests must stay in the care flow.
func isCourseOnlyRequest(
	request *enrollmentModels.OfferingChangeRequest,
	catalog *OfferingChangeCatalog,
	addedCourseIDs []int64,
) (bool, error) {
	if catalog == nil || len(addedCourseIDs) == 0 {
		return false, nil
	}
	selections, err := selectionsFromPayload(request.Payload)
	if err != nil {
		return false, err
	}
	selected := make(map[int64]OfferingChangeSelection, len(selections))
	for _, selection := range selections {
		selected[selection.OfferingID] = selection
	}
	added := make(map[int64]bool, len(addedCourseIDs))
	for _, offeringID := range addedCourseIDs {
		added[offeringID] = true
	}
	known := make(map[int64]bool, len(catalog.Items))
	for _, item := range catalog.Items {
		known[item.OfferingID] = true
		selection, requested := selected[item.OfferingID]
		if item.Automatic {
			continue
		}
		if !item.Selected && requested && added[item.OfferingID] {
			continue
		}
		if item.Selected != requested {
			return false, nil
		}
		if requested && item.DaysOfWeekMode == enrollmentModels.DaysOfWeekModeParentChoice &&
			!slices.Equal(canonicalDays(item.SelectedDays), canonicalDays(selection.SelectedDays)) {
			return false, nil
		}
	}
	for offeringID := range selected {
		if !known[offeringID] {
			return false, nil
		}
	}
	return true, nil
}

// addedCourseIDs is the set of courses a request adds on top of what the child
// already holds — the definition of "this is a course request".
func addedCourseIDs(
	request *enrollmentModels.OfferingChangeRequest,
	courses []CourseCatalogItem,
) ([]int64, error) {
	selections, err := selectionsFromPayload(request.Payload)
	if err != nil {
		return nil, err
	}
	selected := make(map[int64]bool, len(selections))
	for _, selection := range selections {
		selected[selection.OfferingID] = true
	}
	added := make([]int64, 0, len(courses))
	for _, course := range courses {
		if selected[course.OfferingID] && !course.Booked {
			added = append(added, course.OfferingID)
		}
	}
	return added, nil
}

// applyCourseCapacity fills the effective limit, the free slots and — for a
// requested course without a free slot — the waiting position.
func (s *offeringChangeRequestService) applyCourseCapacity(
	ctx context.Context,
	catalog *OfferingChangeCatalog,
	courses []CourseCatalogItem,
	groupsByOffering map[int64][]enrollmentModels.CourseGroup,
	pending *enrollmentModels.OfferingChangeRequest,
) error {
	groupIDs := make([]int64, 0, len(courses))
	for _, course := range courses {
		for _, group := range groupsByOffering[course.OfferingID] {
			groupIDs = append(groupIDs, group.ID)
		}
	}
	taken, err := s.courseOccupancy(ctx, groupIDs, catalog.EarliestEffectiveFrom, 0)
	if err != nil {
		return err
	}
	offeringFree := make(map[int64]*int, len(catalog.Items))
	offeringCapacity := make(map[int64]*int, len(catalog.Items))
	for _, item := range catalog.Items {
		offeringFree[item.OfferingID] = item.FreeSlots
		offeringCapacity[item.OfferingID] = item.Capacity
	}
	for i := range courses {
		course := &courses[i]
		capacity, free := offeringCapacity[course.OfferingID], offeringFree[course.OfferingID]
		// One offering can feed several Regeltermine (a Jahrgang split). The
		// child ends up in one of them, and we cannot know which before the
		// decision — so the tightest of them decides. Reporting the roomiest
		// would promise a seat the approval then refuses.
		for _, group := range groupsByOffering[course.OfferingID] {
			capacity, free = effectiveCourseCapacity(
				group.ParticipantLimit, taken[group.ID], capacity, free,
			)
		}
		course.Capacity, course.FreeSlots = capacity, free
		if !course.Requested || course.Booked || free == nil || *free > 0 {
			continue
		}
		course.Waitlisted = true
		if pending == nil {
			continue
		}
		position, posErr := s.courseWaitlistPosition(
			ctx, course.OfferingID, groupsByOffering[course.OfferingID], pending,
		)
		if posErr != nil {
			return posErr
		}
		course.WaitlistPosition = position
	}
	return nil
}

// effectiveCourseCapacity takes the stricter of the two limits a school can
// maintain. Both are optional; an unset limit never narrows the other one.
func effectiveCourseCapacity(
	groupLimit *int, groupTaken int,
	offeringCapacity, offeringFree *int,
) (capacity, free *int) {
	if groupLimit != nil {
		limit := *groupLimit
		capacity = &limit
		remaining := max(limit-groupTaken, 0)
		free = &remaining
	}
	if offeringCapacity == nil {
		return capacity, free
	}
	if capacity == nil || *offeringCapacity < *capacity {
		limit := *offeringCapacity
		capacity = &limit
	}
	if offeringFree != nil && (free == nil || *offeringFree < *free) {
		remaining := *offeringFree
		free = &remaining
	}
	return capacity, free
}

func (s *offeringChangeRequestService) courseOccupancy(
	ctx context.Context,
	groupIDs []int64,
	onDate timezone.Date,
	excludeStudentID int64,
) (map[int64]int, error) {
	projection, err := s.courseProjection()
	if err != nil {
		return nil, err
	}
	counts, err := projection.CountActiveCourseEnrollments(ctx, groupIDs, onDate, excludeStudentID)
	if err != nil {
		return nil, fmt.Errorf("course request: count course rosters: %w", err)
	}
	return counts, nil
}

// courseWaitlistPosition is the child's rank in the queue for one course: how
// many open requests for the same target group were submitted before this one.
// The child's own request counts, so the family that asked first reads
// "Platz 1".
func (s *offeringChangeRequestService) courseWaitlistPosition(
	ctx context.Context,
	offeringID int64,
	groups []enrollmentModels.CourseGroup,
	pending *enrollmentModels.OfferingChangeRequest,
) (int, error) {
	rows, err := s.ChangeRepo.ListPendingForTenant(ctx, enrollmentModels.OfferingChangeQueueFilters{})
	if err != nil {
		return 0, fmt.Errorf("course request: list pending for waitlist: %w", err)
	}
	dates := make(map[int64]timezone.Date, len(rows))
	childIDs := make([]int64, 0, len(rows))
	for _, row := range rows {
		if row != nil && row.ID != pending.ID && !courseRequestAfter(row, pending) && row.RequestChildID > 0 {
			dates[row.RequestChildID] = timezone.Date(row.EffectiveFrom)
			childIDs = append(childIDs, row.RequestChildID)
		}
	}
	slices.Sort(childIDs)
	childrenByID := make(map[int64]*RequestChild)
	if courseGroupsHaveTargets(groups) {
		children, childErr := offeringChildrenByID(ctx, s.Children, slices.Compact(childIDs))
		if childErr != nil {
			return 0, fmt.Errorf("course request: load waitlist children: %w", childErr)
		}
		childrenByID = make(map[int64]*RequestChild, len(children))
		for _, child := range children {
			if child != nil {
				childrenByID[child.ID] = child
			}
		}
	}
	current := make([]*RequestChildOffering, 0)
	if len(dates) > 0 {
		ownerDates := make(map[int64]enrollmentOwner.Date, len(dates))
		for childID, date := range dates {
			ownerDates[childID] = enrollmentOwner.Date(date)
		}
		values, currentErr := s.Children.RequestChildOfferingsAtDates(ctx, ownerDates)
		if currentErr != nil {
			return 0, fmt.Errorf("course request: load current offerings for waitlist: %w", currentErr)
		}
		current = legacyOfferingSelections(values)
	}
	bookedByChild := make(map[int64]map[int64]bool, len(dates))
	for _, link := range current {
		if link == nil {
			continue
		}
		if bookedByChild[link.RequestChildID] == nil {
			bookedByChild[link.RequestChildID] = make(map[int64]bool)
		}
		bookedByChild[link.RequestChildID][link.CareOfferingID] = true
	}
	return courseWaitlistPositionFromRows(rows, offeringID, groups, pending, childrenByID, bookedByChild), nil
}

func courseWaitlistPositionFromRows(
	rows []*enrollmentModels.OfferingChangeRequest,
	offeringID int64,
	groups []enrollmentModels.CourseGroup,
	pending *enrollmentModels.OfferingChangeRequest,
	childrenByID map[int64]*RequestChild,
	bookedByChild map[int64]map[int64]bool,
) int {
	position := 0
	for _, row := range rows {
		if row == nil || courseRequestAfter(row, pending) {
			continue
		}
		if !courseGroupsMatchTarget(groups, childrenByID[row.RequestChildID]) {
			continue
		}
		added, addedErr := courseWasAdded(row, offeringID, bookedByChild[row.RequestChildID])
		if addedErr != nil {
			// A payload we cannot read is not evidence of a competing course
			// request; it is a row the review queue will surface anyway.
			continue
		}
		if added {
			position++
		}
	}
	return max(position, 1)
}

func courseRequestAfter(row, pending *enrollmentModels.OfferingChangeRequest) bool {
	if row == nil || pending == nil {
		return true
	}
	return row.CreatedAt.After(pending.CreatedAt) ||
		(row.CreatedAt.Equal(pending.CreatedAt) && row.ID > pending.ID)
}

func courseGroupsMatchTarget(groups []enrollmentModels.CourseGroup, child *RequestChild) bool {
	if !courseGroupsHaveTargets(groups) {
		return true
	}
	if child == nil {
		return false
	}
	return slices.ContainsFunc(groups, func(group enrollmentModels.CourseGroup) bool {
		return group.Active && courseGroupMatchesTarget(group, courseTargetCatalog(child))
	})
}

func courseGroupsHaveTargets(groups []enrollmentModels.CourseGroup) bool {
	return slices.ContainsFunc(groups, func(group enrollmentModels.CourseGroup) bool {
		return len(group.SourceGradeLevels) > 0 || len(group.SourceSchoolClasses) > 0
	})
}

// courseWasAdded distinguishes a course request from another offering change
// whose complete desired selection merely retains an already booked course.
func courseWasAdded(
	row *enrollmentModels.OfferingChangeRequest,
	offeringID int64,
	booked map[int64]bool,
) (bool, error) {
	selections, err := selectionsFromPayload(row.Payload)
	if err != nil {
		return false, err
	}
	for _, selection := range selections {
		if selection.OfferingID == offeringID {
			return !booked[offeringID], nil
		}
	}
	return false, nil
}

// assertCourseCapacityAvailable refuses an approval that would put one more
// child into a full AG. It is a no-op for an offering without an AG, and for
// an AG without a Teilnehmergrenze.
func (s *offeringChangeRequestService) assertCourseCapacityAvailable(
	ctx context.Context,
	studentID int64,
	requestChildID int64,
	offering *enrollmentModels.CareOffering,
	effectiveFrom timezone.Date,
) error {
	if offering == nil {
		return nil
	}
	if _, err := s.courseProjection(); err != nil {
		return err
	}
	child, err := offeringChildByID(ctx, s.Children, requestChildID)
	if err != nil {
		return fmt.Errorf("course request: load request child: %w", err)
	}
	if child == nil {
		return fmt.Errorf("course request: request child %d not found", requestChildID)
	}
	groups, hadCourseTarget, err := s.offeringCourseGroups(ctx, offering, child.TargetGradeLevel, child.TargetSchoolClass)
	if err != nil {
		return err
	}
	if len(groups) == 0 {
		if hadCourseTarget {
			return fmt.Errorf("%w: %s", ErrOfferingChangeInvalid, offering.Name)
		}
		return nil
	}
	groups, err = s.lockCourseGroups(ctx, groups)
	if err != nil {
		return err
	}
	groupIDs := make([]int64, 0, len(groups))
	for _, group := range groups {
		groupIDs = append(groupIDs, group.ID)
	}
	taken, err := s.courseOccupancy(ctx, groupIDs, effectiveFrom, studentID)
	if err != nil {
		return err
	}
	for _, group := range groups {
		if group.ParticipantLimit != nil && taken[group.ID] >= *group.ParticipantLimit {
			return fmt.Errorf("%w: %s", ErrOfferingChangeCapacityFull, offering.Name)
		}
	}
	return nil
}

// lockCourseGroups serializes every approval that can consume the same AG
// capacity. Offering locks alone do not suffice: several offerings may feed a
// single group through their source templates.
func (s *offeringChangeRequestService) lockCourseGroups(
	ctx context.Context,
	groups []enrollmentModels.CourseGroup,
) ([]enrollmentModels.CourseGroup, error) {
	ids := make([]int64, 0, len(groups))
	for _, group := range groups {
		ids = append(ids, group.ID)
	}
	slices.Sort(ids)
	ids = slices.Compact(ids)
	projection, err := s.courseProjection()
	if err != nil {
		return nil, err
	}
	locked, err := projection.LockCourseGroups(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("course request: lock course capacity: %w", err)
	}
	return locked, nil
}

// offeringCourseGroups resolves both link shapes for a single offering.
func (s *offeringChangeRequestService) offeringCourseGroups(
	ctx context.Context,
	offering *enrollmentModels.CareOffering,
	gradeLevel *int16,
	schoolClass *string,
) ([]enrollmentModels.CourseGroup, bool, error) {
	projection, err := s.courseProjection()
	if err != nil {
		return nil, false, err
	}
	projected, err := projection.CourseGroupsForOfferings(ctx, []enrollmentModels.CourseOfferingReference{{
		OfferingID: offering.ID, ActivityGroupID: offering.ActivityGroupID,
	}})
	if err != nil {
		return nil, false, fmt.Errorf("course request: list course groups: %w", err)
	}
	catalog := &OfferingChangeCatalog{TargetGradeLevel: gradeLevel}
	if schoolClass != nil {
		catalog.TargetSchoolClass = *schoolClass
	}
	groups, hadCourseTarget := activeCourseGroupsForTarget(projected[offering.ID], catalog)
	return groups, hadCourseTarget, nil
}

func activeCourseGroupsForTarget(
	projected []enrollmentModels.CourseGroup,
	catalog *OfferingChangeCatalog,
) ([]enrollmentModels.CourseGroup, bool) {
	groups := make([]enrollmentModels.CourseGroup, 0, len(projected))
	hadCourseTarget := false
	seen := make(map[int64]bool, len(projected))
	for _, group := range projected {
		if !seen[group.ID] && courseGroupMatchesTarget(group, catalog) {
			seen[group.ID] = true
			hadCourseTarget = true
			if group.Active {
				groups = append(groups, group)
			}
		}
	}
	return groups, hadCourseTarget
}

func (s *offeringChangeRequestService) CreateCourseRequest(
	ctx context.Context,
	input CreateCourseRequestInput,
) (*enrollmentModels.OfferingChangeRequest, error) {
	enabled, err := s.courseRequestsEnabled(ctx)
	if err != nil {
		return nil, err
	}
	if !enabled {
		return nil, ErrCourseRequestsDisabled
	}
	catalog, err := s.Catalog(ctx, input.StudentID)
	if err != nil {
		return nil, err
	}
	_, groupsByOffering, err := s.courseItems(ctx, catalog)
	if err != nil {
		return nil, err
	}
	course, err := courseCatalogEntry(catalog, courseGroupIDs(groupsByOffering), input.OfferingID)
	if err != nil {
		return nil, err
	}
	selections := courseSelectionsWith(catalog, course)
	created, err := s.Create(ctx, CreateOfferingChangeInput{
		StudentID:     input.StudentID,
		AccountID:     input.AccountID,
		Selections:    selections,
		EffectiveFrom: catalog.EarliestEffectiveFrom,
		Note:          strings.TrimSpace(input.Note),
	})
	if err != nil {
		return nil, err
	}
	s.Logger.Info("course request created",
		slog.Int64("student_id", input.StudentID),
		slog.Int64("care_offering_id", input.OfferingID),
		slog.String("effective_from", catalog.EarliestEffectiveFrom.String()),
	)
	return created, nil
}

func courseCatalogEntry(
	catalog *OfferingChangeCatalog,
	groupsByOffering map[int64][]int64,
	offeringID int64,
) (*OfferingChangeCatalogItem, error) {
	for i := range catalog.Items {
		item := &catalog.Items[i]
		if item.OfferingID != offeringID {
			continue
		}
		if len(groupsByOffering[item.OfferingID]) == 0 || !item.IsActive {
			return nil, ErrCourseNotFound
		}
		if item.Selected {
			return nil, ErrCourseAlreadyBooked
		}
		return item, nil
	}
	return nil, ErrCourseNotFound
}

// courseSelectionsWith is the child's current booking plus the new course. The
// request payload is a complete desired selection, never a delta — the same
// contract the enrollment form and the change modal write.
func courseSelectionsWith(
	catalog *OfferingChangeCatalog,
	course *OfferingChangeCatalogItem,
) []OfferingChangeSelection {
	selections := make([]OfferingChangeSelection, 0, len(catalog.Items)+1)
	for _, item := range catalog.Items {
		if !item.Selected || item.Automatic {
			continue
		}
		current := OfferingChangeSelection{OfferingID: item.OfferingID}
		if item.DaysOfWeekMode == enrollmentModels.DaysOfWeekModeParentChoice {
			current.SelectedDays = append([]string(nil), item.SelectedDays...)
		}
		selections = append(selections, current)
	}
	// Die Tage werden nur mitgeschickt, wenn das Angebot eine Auswahl zulässt.
	// Bei einem Kurs legt die Schule sie fast immer selbst fest, und eine
	// mitgeschickte Auswahl weist der Server dann zurück.
	selection := OfferingChangeSelection{OfferingID: course.OfferingID}
	if course.DaysOfWeekMode == enrollmentModels.DaysOfWeekModeParentChoice {
		selection.SelectedDays = append([]string(nil), course.AvailableDays...)
	}
	return append(selections, selection)
}

func (s *offeringChangeRequestService) WithdrawCourseRequest(
	ctx context.Context,
	requestID, accountID, studentID int64,
) error {
	row, err := s.ChangeRepo.FindByIDForUpdate(ctx, requestID)
	if err != nil {
		return err
	}
	// A foreign request stays reported as missing, never as forbidden.
	if row.StudentID != studentID || row.SubmittedBy != accountID {
		return enrollmentModels.ErrOfferingChangeNotFound
	}
	if row.IsTerminal() {
		return enrollmentModels.ErrOfferingChangeNotPending
	}
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
		// Withdrawing here would silently drop a care-offering change the
		// family made somewhere else. That request is edited where it was made.
		return ErrCourseRequestNotOwn
	}
	if err := s.ChangeRepo.Decide(
		ctx, requestID, enrollmentModels.OfferingChangeStatusWithdrawn, nil, nil, false,
	); err != nil {
		return err
	}
	s.Logger.Info("course request withdrawn",
		slog.Int64("student_id", studentID),
		slog.Int64("request_id", requestID),
	)
	return nil
}
