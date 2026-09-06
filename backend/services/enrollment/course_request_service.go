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
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

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
		if errors.Is(err, ErrOfferingChangeDisabled) {
			return false, nil
		}
		return false, err
	}
	enabled, err := s.Settings.ResolveBool(ctx, configModel.KeyEnrollmentParentCourseRequestsEnabled)
	if err != nil {
		// Fail closed: a school that never switched this on must not start
		// collecting course requests because a lookup failed.
		return false, nil
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
	pending, isCourseRequest, err := s.pendingCourseRequest(ctx, studentID, courses)
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
) ([]CourseCatalogItem, map[int64][]*activitiesModels.Group, error) {
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
) (map[int64][]*activitiesModels.Group, error) {
	groups := make(map[int64][]*activitiesModels.Group, len(offeringIDs))
	if s.ActivityGroupRepo == nil || len(offeringIDs) == 0 {
		return groups, nil
	}
	// The legacy link points from the offering at one group, so it has to be
	// read per offering. Schools set up this way have a handful of them.
	for _, item := range catalog.Items {
		if !item.IsActive || item.ActivityGroupID == nil || *item.ActivityGroupID <= 0 {
			continue
		}
		group, err := s.ActivityGroupRepo.FindByID(ctx, *item.ActivityGroupID)
		if err != nil {
			return nil, fmt.Errorf("course request: load course %d: %w", *item.ActivityGroupID, err)
		}
		if isCourseGroup(group) {
			groups[item.OfferingID] = append(groups[item.OfferingID], group)
		}
	}
	templates, err := s.ActivityGroupRepo.FindTemplatesBySourceOfferings(ctx, offeringIDs)
	if err != nil {
		return nil, fmt.Errorf("course request: list templates fed by offerings: %w", err)
	}
	for _, template := range templates {
		if !isCourseGroup(template) {
			continue
		}
		for _, offeringID := range template.SourceCareOfferingIDs {
			if !slices.ContainsFunc(groups[offeringID], func(g *activitiesModels.Group) bool {
				return g.ID == template.ID
			}) {
				groups[offeringID] = append(groups[offeringID], template)
			}
		}
	}
	for offeringID := range groups {
		slices.SortFunc(groups[offeringID], func(a, b *activitiesModels.Group) int {
			return int(a.ID - b.ID)
		})
	}
	return groups, nil
}

// markCourseDiffEntries flags the diff lines that are about a Kurs. The entry
// builder already knows the legacy link on the offering; the link a Regeltermin
// declares itself (#2137) needs one lookup, which is why it happens here in one
// batch rather than per line.
func (s *offeringChangeRequestService) markCourseDiffEntries(
	ctx context.Context,
	entries []OfferingChangeDiffEntry,
) error {
	if s.ActivityGroupRepo == nil || len(entries) == 0 {
		return nil
	}
	offeringIDs := make([]int64, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsCourse {
			offeringIDs = append(offeringIDs, entry.OfferingID)
		}
	}
	if len(offeringIDs) == 0 {
		return nil
	}
	templates, err := s.ActivityGroupRepo.FindTemplatesBySourceOfferings(ctx, offeringIDs)
	if err != nil {
		return fmt.Errorf("offering change: mark course diff lines: %w", err)
	}
	courses := make(map[int64]bool, len(templates))
	for _, template := range templates {
		if !isCourseGroup(template) {
			continue
		}
		for _, offeringID := range template.SourceCareOfferingIDs {
			courses[offeringID] = true
		}
	}
	for i := range entries {
		if courses[entries[i].OfferingID] {
			entries[i].IsCourse = true
		}
	}
	return nil
}

func isCourseGroup(group *activitiesModels.Group) bool {
	return group != nil &&
		group.Type == activitiesModels.GroupTypeActivity &&
		group.ArchivedAt == nil
}

// courseGroupIDs is the id-only projection the catalog and the create path
// pass around.
func courseGroupIDs(groups map[int64][]*activitiesModels.Group) map[int64][]int64 {
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
	requested := make(map[int64]bool, len(added))
	for _, offeringID := range added {
		requested[offeringID] = true
	}
	for i := range courses {
		courses[i].Requested = requested[courses[i].OfferingID]
	}
	return pending, len(added) > 0, nil
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
	groupsByOffering map[int64][]*activitiesModels.Group,
	pending *enrollmentModels.OfferingChangeRequest,
) error {
	groupIDs := make([]int64, 0, len(courses))
	for _, course := range courses {
		for _, group := range groupsByOffering[course.OfferingID] {
			groupIDs = append(groupIDs, group.ID)
		}
	}
	taken, err := s.courseOccupancy(ctx, groupIDs, catalog.EarliestEffectiveFrom)
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
				group.ParticipantLimit(), taken[group.ID], capacity, free,
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
		position, posErr := s.courseWaitlistPosition(ctx, course.OfferingID, pending.CreatedAt)
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
) (map[int64]int, error) {
	if s.StudentEnrollmentRepo == nil {
		return map[int64]int{}, nil
	}
	counts, err := s.StudentEnrollmentRepo.CountActiveByGroupIDs(
		ctx, groupIDs, activitiesModels.Date(onDate.String()),
	)
	if err != nil {
		return nil, fmt.Errorf("course request: count course rosters: %w", err)
	}
	return counts, nil
}

// courseWaitlistPosition is the child's rank in the queue for one course: how
// many open requests for it were submitted no later than this one. The child's
// own request counts, so the family that asked first reads "Platz 1".
func (s *offeringChangeRequestService) courseWaitlistPosition(
	ctx context.Context,
	offeringID int64,
	submittedAt time.Time,
) (int, error) {
	rows, err := s.ChangeRepo.ListPendingForTenant(ctx, modelBase.RequestQueueFilters{})
	if err != nil {
		return 0, fmt.Errorf("course request: list pending for waitlist: %w", err)
	}
	position := 0
	for _, row := range rows {
		if row.CreatedAt.After(submittedAt) {
			continue
		}
		selections, decodeErr := selectionsFromPayload(row.Payload)
		if decodeErr != nil {
			// A payload we cannot read is not evidence of a competing course
			// request; it is a row the review queue will surface anyway.
			continue
		}
		for _, selection := range selections {
			if selection.OfferingID == offeringID {
				position++
				break
			}
		}
	}
	return max(position, 1), nil
}

// assertCourseCapacityAvailable refuses an approval that would put one more
// child into a full AG. It is a no-op for an offering without an AG, and for
// an AG without a Teilnehmergrenze.
func (s *offeringChangeRequestService) assertCourseCapacityAvailable(
	ctx context.Context,
	offering *enrollmentModels.CareOffering,
	effectiveFrom timezone.Date,
) error {
	if offering == nil || s.ActivityGroupRepo == nil || s.StudentEnrollmentRepo == nil {
		return nil
	}
	groups, err := s.offeringCourseGroups(ctx, offering)
	if err != nil {
		return err
	}
	if len(groups) == 0 {
		return nil
	}
	groupIDs := make([]int64, 0, len(groups))
	for _, group := range groups {
		groupIDs = append(groupIDs, group.ID)
	}
	taken, err := s.courseOccupancy(ctx, groupIDs, effectiveFrom)
	if err != nil {
		return err
	}
	for _, group := range groups {
		if group.HasParticipantLimit() && !group.HasAvailableSpots(taken[group.ID]) {
			return fmt.Errorf("%w: %s", ErrOfferingChangeCapacityFull, offering.Name)
		}
	}
	return nil
}

// offeringCourseGroups resolves both link shapes for a single offering.
func (s *offeringChangeRequestService) offeringCourseGroups(
	ctx context.Context,
	offering *enrollmentModels.CareOffering,
) ([]*activitiesModels.Group, error) {
	groups := make([]*activitiesModels.Group, 0, 2)
	if offering.ActivityGroupID != nil && *offering.ActivityGroupID > 0 {
		group, err := s.ActivityGroupRepo.FindByID(ctx, *offering.ActivityGroupID)
		if err != nil {
			return nil, fmt.Errorf("course request: load course %d: %w", *offering.ActivityGroupID, err)
		}
		if isCourseGroup(group) {
			groups = append(groups, group)
		}
	}
	templates, err := s.ActivityGroupRepo.FindTemplatesBySourceOffering(ctx, offering.ID)
	if err != nil {
		return nil, fmt.Errorf("course request: list templates fed by offering %d: %w", offering.ID, err)
	}
	for _, template := range templates {
		if !isCourseGroup(template) {
			continue
		}
		if !slices.ContainsFunc(groups, func(g *activitiesModels.Group) bool { return g.ID == template.ID }) {
			groups = append(groups, template)
		}
	}
	return groups, nil
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
	if len(added) == 0 {
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
