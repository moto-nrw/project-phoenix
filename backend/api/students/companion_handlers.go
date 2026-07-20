package students

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
	usersService "github.com/moto-nrw/project-phoenix/services/users"
)

// CompanionResponse is one child this child walks home with, plus the weekdays
// they walk together.
type CompanionResponse struct {
	CompanionStudentID int64    `json:"companion_student_id"`
	FirstName          string   `json:"first_name,omitempty"`
	LastName           string   `json:"last_name,omitempty"`
	Weekdays           []string `json:"weekdays"`
}

// CompanionConflictResponse is the 409 body: the companions whose own departure
// plan does not permit the requested days. The client turns it into the
// "Tom darf donnerstags noch nicht mit anderen Kindern gehen. Ergänzen?"
// confirmation and resends with extend_companion_plans.
type CompanionConflictResponse struct {
	Conflicts []usersService.CompanionConflict `json:"conflicts"`
	Message   string                           `json:"message"`
}

// Render satisfies render.Renderer.
func (resp *CompanionConflictResponse) Render(_ http.ResponseWriter, r *http.Request) error {
	render.Status(r, http.StatusConflict)
	return nil
}

// CompanionEntry is one submitted link.
type CompanionEntry struct {
	CompanionStudentID int64    `json:"companion_student_id"`
	Weekdays           []string `json:"weekdays"`
}

// getStudentCompanions returns the children this child walks home with.
//
// Read access follows the same scope rules as the rest of the child's data —
// a companion's NAME is another child's personal data, so this must never be
// looser than reading the child's own record.
func (rs *Resource) getStudentCompanions(w http.ResponseWriter, r *http.Request) {
	student, ok := rs.parseAndGetStudent(w, r)
	if !ok {
		return
	}
	if !rs.checkStudentReadAccess(r, student) {
		renderError(w, r, common.ErrorForbidden(errors.New("read access required to view departure companions")))
		return
	}

	links, err := rs.StudentService.ListCompanions(r.Context(), student.ID)
	if err != nil {
		renderError(w, r, common.ErrorInternalServerWrap("failed to load departure companions", err))
		return
	}

	common.Respond(w, r, http.StatusOK, toCompanionResponses(links), "Departure companions retrieved")
}

// lockCompanionRows takes the student row locks this request may need — the
// child being edited, every submitted companion, AND every currently linked
// companion — in one ascending-id pass.
//
// It has to run before the subject's own lock: an update that confirms an
// extension writes the companion row too, and locking the subject first would
// leave two requests editing children A and B, each linking the other, free to
// acquire A→B and B→A. PostgreSQL resolves that by aborting one with a deadlock
// error, which surfaces as a 500 on a perfectly legitimate edit.
//
// The CURRENT companions matter as much as the submitted ones: replacing the
// list (or narrowing the plan, which trims links) REMOVES edges, and each
// removal is judged by checkCompanionRemovals against the far child's other
// links. With existing links A-B and C-B, two concurrent requests clearing A's
// and C's lists would otherwise each observe B's other link, both pass the
// check, and both commit — leaving B with an accompanied plan and no "mit wem"
// detail at all. Locking B from both requests serializes them, and the second
// one re-validates against the first one's committed state.
func (rs *Resource) lockCompanionRows(ctx context.Context, student *userModels.Student, req *UpdateStudentRequest) error {
	var submitted []int64
	if req.hasCompanionUpdate() {
		for _, entry := range *req.Companions {
			submitted = append(submitted, entry.CompanionStudentID)
		}
	}
	return rs.lockStudentCompanionGraph(ctx, student.ID, submitted)
}

// lockStudentCompanionGraph is the shared lock protocol for every request that
// can add or remove "läuft mit" edges: it locks the subject, the submitted
// companions, and the far end of every stored edge in one ascending-id pass
// (via LockStudentsForUpdate). deleteStudent uses it too — its ON DELETE
// CASCADE removes every edge, which is a removal like any other.
//
// The stored-companion snapshot is read BEFORE the locks (there is no other
// order), so an edge committed between the snapshot and the lock pass could
// have a far end this pass never locked. One re-read under the subject's lock
// closes that: every writer that creates or removes an edge touching this
// child locks this child's row first (its subject or its submitted-companion
// set contains the id), so once we hold the subject no further edges can
// appear, and a single top-up pass over the late edges suffices.
func (rs *Resource) lockStudentCompanionGraph(ctx context.Context, studentID int64, submitted []int64) error {
	stored, err := rs.StudentService.ListCompanionIDs(ctx, studentID)
	if err != nil {
		return err
	}

	locked := make(map[int64]bool, len(submitted)+len(stored)+1)
	ids := make([]int64, 0, len(submitted)+len(stored)+1)
	add := func(id int64) {
		if id <= 0 || locked[id] {
			return
		}
		locked[id] = true
		ids = append(ids, id)
	}
	add(studentID)
	for _, id := range submitted {
		add(id)
	}
	for _, id := range stored {
		add(id)
	}
	if err := rs.StudentService.LockStudentsForUpdate(ctx, ids); err != nil {
		return err
	}

	fresh, err := rs.StudentService.ListCompanionIDs(ctx, studentID)
	if err != nil {
		return err
	}
	var late []int64
	for _, id := range fresh {
		if !locked[id] {
			late = append(late, id)
		}
	}
	if len(late) == 0 {
		return nil
	}
	return rs.StudentService.LockStudentsForUpdate(ctx, late)
}

// companionConflictError carries the conflicting companions out of the update
// transaction so the closure can abort (rolling back every write) while the
// handler still renders the 409 the client needs to ask its question.
type companionConflictError struct {
	Conflicts []usersService.CompanionConflict
}

func (e *companionConflictError) Error() string {
	return "companion departure plans do not allow the requested days"
}

// errCompanionExtendForbidden is returned when the caller may edit the child in
// front of them but not the companion whose departure plan the confirmation
// would widen.
var errCompanionExtendForbidden = errors.New("Für ein verknüpftes Kind fehlt die Berechtigung, den Heimweg zu ändern.") //nolint:staticcheck // ST1005: user-facing German message

// checkCompanionConflicts runs the COMPLETE companion validation before the
// update writes anything: link shape, count, the subject's own weekdays,
// companion existence, and the companions whose plan does not allow the
// requested days.
//
// Everything has to happen here, not in applyCompanionUpdate: this request runs
// inside the middleware's tenant transaction, which commits on every non-5xx
// response, so a 4xx raised after the first write would keep that write.
//
// It returns the companion ids whose departure plan the caller is allowed to
// widen; applyCompanionUpdate hands that set to the write path so a companion
// that never passed this check cannot be widened.
//
// `student` must already carry the plan resolved from this request.
func (rs *Resource) checkCompanionConflicts(ctx context.Context, student *userModels.Student, req *UpdateStudentRequest, userPermissions []string) (map[int64]map[string]bool, error) {
	accompaniedDays := userModels.AccompaniedWeekdays(student.AllowedDepartureModes, student.DepartureDays)

	// Removal-only paths. Both drop links without a submitted list to validate —
	// the trim keeps what the new plan still allows, an emptied plan drops
	// everything — and both can strand the child at the far end of a dropped
	// link. applyCompanionUpdate performs them AFTER the first writes of this
	// transaction, so the refusal has to be raised here.
	if !req.hasCompanionUpdate() {
		return nil, rs.StudentService.CheckCompanionTrim(ctx, student.ID, accompaniedDays)
	}
	if len(accompaniedDays) == 0 {
		return nil, rs.StudentService.CheckCompanionTrim(ctx, student.ID, nil)
	}

	conflicts, err := rs.StudentService.CheckCompanionConflicts(ctx, student.ID, usersService.CompanionUpdate{
		Links:                toCompanionLinks(*req.Companions),
		AccompaniedDays:      accompaniedDays,
		ExtendCompanionPlans: req.ExtendCompanionPlans,
	})
	if err != nil {
		return nil, err
	}
	if len(conflicts) == 0 {
		return map[int64]map[string]bool{}, nil
	}
	// Ask again whenever the current conflicts are not covered by what the user
	// actually confirmed. The confirmation was answered against an unlocked
	// snapshot, so a conflict that appeared (or grew a weekday) in the meantime
	// was never on screen — extending it would widen a child's departure
	// permission nobody agreed to. A 409 with the CURRENT set puts the question
	// back where it belongs; nothing is written either way.
	if !req.ExtendCompanionPlans || !companionConflictsConfirmed(conflicts, req.ConfirmedCompanionExtensions) {
		return nil, &companionConflictError{Conflicts: conflicts}
	}
	return rs.authorizeCompanionExtension(ctx, conflicts, userPermissions)
}

// companionConflictsConfirmed reports whether every conflicting weekday of every
// conflicting companion appears in the set the client confirmed. Extra confirmed
// entries are fine — they only mean a conflict resolved itself in the meantime.
func companionConflictsConfirmed(conflicts []usersService.CompanionConflict, confirmed []CompanionEntry) bool {
	byStudent := make(map[int64]map[string]bool, len(confirmed))
	for _, entry := range confirmed {
		days := byStudent[entry.CompanionStudentID]
		if days == nil {
			days = make(map[string]bool, len(entry.Weekdays))
			byStudent[entry.CompanionStudentID] = days
		}
		for _, day := range entry.Weekdays {
			days[day] = true
		}
	}

	for _, conflict := range conflicts {
		days := byStudent[conflict.StudentID]
		for _, day := range conflict.Weekdays {
			if !days[day] {
				return false
			}
		}
	}
	return true
}

// authorizeCompanionExtension gates the ONE companion write this endpoint can
// perform: widening a linked child's own allowed departure modes.
//
// Linking to a child in another group is the point of a Laufgemeinschaft, so
// creating the edge stays open to anyone who may edit the subject. Changing
// the OTHER child's departure permission is a different act — it must pass the
// same authorization that a direct update of that child would, or a group
// supervisor could edit the Heimweg of any child in the school by way of a
// crafted extend_companion_plans request.
//
// The rows it judges are the rows that get written: lockCompanionRows took a
// FOR UPDATE lock on every submitted companion before this validation ran, so
// no concurrent request can reassign a companion out of the caller's groups or
// narrow its plan between this pass and the write pass — the second validation
// inside ReplaceCompanions sees byte-identical rows and therefore the identical
// conflict set. The returned set is passed on as CompanionUpdate.
// AuthorizedExtensions so that assumption is enforced rather than trusted, down
// to the individual weekdays these conflicts name.
func (rs *Resource) authorizeCompanionExtension(ctx context.Context, conflicts []usersService.CompanionConflict, userPermissions []string) (map[int64]map[string]bool, error) {
	ids := make([]int64, 0, len(conflicts))
	for _, conflict := range conflicts {
		ids = append(ids, conflict.StudentID)
	}

	companions, err := rs.StudentService.GetByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}
	authorized := make(map[int64]map[string]bool, len(ids))
	for _, conflict := range conflicts {
		companion := companions[conflict.StudentID]
		if companion == nil {
			return nil, usersService.ErrCompanionNotFound
		}
		if ok, _ := canUpdateStudent(ctx, userPermissions, companion, rs.UserContextService); !ok {
			return nil, errCompanionExtendForbidden
		}
		days := authorized[conflict.StudentID]
		if days == nil {
			days = make(map[string]bool, len(conflict.Weekdays))
			authorized[conflict.StudentID] = days
		}
		for _, day := range conflict.Weekdays {
			days[day] = true
		}
	}
	return authorized, nil
}

// applyCompanionUpdate reconciles the child's "läuft mit" links with the
// departure plan that is about to be written.
//
// It runs BEFORE the student write for two reasons: a link satisfies the
// accompanied-requires-a-note invariant that the model checks on write, and a
// conflict has to abort the transaction before anything lands.
//
// `student` must already carry the plan resolved from this request;
// `authorizedExtensions` is what checkCompanionConflicts returned.
func (rs *Resource) applyCompanionUpdate(ctx context.Context, student *userModels.Student, req *UpdateStudentRequest, authorizedExtensions map[int64]map[string]bool) error {
	accompaniedDays := userModels.AccompaniedWeekdays(student.AllowedDepartureModes, student.DepartureDays)

	// The plan no longer allows leaving with another child anywhere: the links
	// lose their basis and go with it, exactly like the free-text note does.
	// Otherwise the Kindersuche would keep grouping a child whose Stammdaten say
	// "fährt Bus".
	if len(accompaniedDays) == 0 {
		if _, err := rs.StudentService.ReplaceCompanions(ctx, student.ID, usersService.CompanionUpdate{
			AccompaniedDays: accompaniedDays,
		}); err != nil {
			return err
		}
		student.DepartureCompanionDays = nil
		return nil
	}

	if !req.hasCompanionUpdate() {
		// The client changed the plan but not the list. Links on a weekday the
		// new plan no longer allows lose their basis and are dropped — keeping
		// them would let the Kindersuche group the children on a day the
		// Stammdaten forbid. What survives still answers "mit wem", so the note
		// stays optional for a child that has a Laufgemeinschaft left.
		links, err := rs.StudentService.TrimCompanionsToDays(ctx, student.ID, accompaniedDays)
		if err != nil {
			return err
		}
		student.DepartureCompanionDays = userModels.CompanionDaysFromLinks(links)
		return nil
	}

	links := toCompanionLinks(*req.Companions)
	conflicts, err := rs.StudentService.ReplaceCompanions(ctx, student.ID, usersService.CompanionUpdate{
		Links:                links,
		AccompaniedDays:      accompaniedDays,
		ExtendCompanionPlans: req.ExtendCompanionPlans,
		AuthorizedExtensions: authorizedExtensions,
	})
	if err != nil {
		return err
	}
	if len(conflicts) > 0 {
		return &companionConflictError{Conflicts: conflicts}
	}

	student.DepartureCompanionDays = userModels.CompanionDaysFromLinks(links)
	return nil
}

// enrichWithCompanions fills CompanionStudentIDs for the given day.
//
// Only the ids travel, not the names: the Kindersuche already knows every child
// on the page, so it can resolve a companion that is visible and quietly ignore
// one that is filtered out. Shipping names here would leak children the caller
// filtered away.
//
// A failure is logged and swallowed — losing the grouping must never fail the
// whole student list.
func (rs *Resource) enrichWithCompanions(ctx context.Context, responses []StudentResponse, params *studentListParams, day time.Time) {
	if !params.includeCompanions {
		return
	}

	studentIDs := collectFullAccessStudentIDs(responses)
	if len(studentIDs) == 0 {
		return
	}

	weekday := companionWeekdayForDate(day)
	byStudent, err := rs.StudentService.CompanionIDsForWeekday(ctx, studentIDs, weekday)
	if err != nil {
		rs.Logger.Error("failed to load departure companions for list",
			slog.String("error", err.Error()),
			slog.Int("weekday", weekday),
		)
		return
	}

	for i := range responses {
		if companions, ok := byStudent[responses[i].ID]; ok && len(companions) > 0 {
			responses[i].CompanionStudentIDs = companions
		}
	}
}

// companionWeekdayForDate maps a date onto the stored 1..5 weekday.
//
// Saturday and Sunday fall back to Monday rather than resolving to "no
// weekday": OGS care does not run on the weekend, so a staff member opening
// the Kindersuche on a Sunday is looking ahead at the coming week. Returning
// nothing there would make the Laufgemeinschaft grouping look broken (every
// child in "Ohne Laufgemeinschaft") for a reason no one can see.
func companionWeekdayForDate(day time.Time) int {
	switch day.Weekday() {
	case time.Tuesday:
		return 2
	case time.Wednesday:
		return 3
	case time.Thursday:
		return 4
	case time.Friday:
		return 5
	default:
		return 1
	}
}

// toCompanionLinks converts the wire entries into the service's link shape.
func toCompanionLinks(entries []CompanionEntry) []userModels.CompanionLink {
	links := make([]userModels.CompanionLink, 0, len(entries))
	for _, entry := range entries {
		links = append(links, userModels.CompanionLink{
			CompanionStudentID: entry.CompanionStudentID,
			Weekdays:           entry.Weekdays,
		})
	}
	return links
}

func toCompanionResponses(links []userModels.CompanionLink) []CompanionResponse {
	out := make([]CompanionResponse, 0, len(links))
	for _, link := range links {
		weekdays := link.Weekdays
		if weekdays == nil {
			weekdays = []string{}
		}
		out = append(out, CompanionResponse{
			CompanionStudentID: link.CompanionStudentID,
			FirstName:          link.FirstName,
			LastName:           link.LastName,
			Weekdays:           weekdays,
		})
	}
	return out
}
