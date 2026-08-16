package students

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/render"
	"github.com/moto-nrw/project-phoenix/api/common"
	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
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

// CodeCompanionLockBusy marks the retriable 409 raised when a linked child's
// row is held by a concurrent edit. It exists because the student PUT answers
// 409 for two very different reasons, and only this one has nothing for the
// user to confirm — the client keys its "Ergänzen?" question off the ABSENCE of
// this code (and off the conflicts list the other 409 carries).
const CodeCompanionLockBusy = "companion_lock_busy"

// CodeCompanionWouldLoseDeparture marks the 400 raised when removing a link
// would leave the OTHER child with an accompanied departure plan and no way to
// say who it walks home with. The client keys the user-actionable German
// message off this code: the save paths reduce a failed student PUT to a
// generic "Fehler beim Speichern", which hides the one instruction that lets
// the user get out of the refusal (fix that child's Heimweg first).
const CodeCompanionWouldLoseDeparture = "companion_would_lose_departure"

// CodeCompanionsChanged marks the 409 raised when the submitted "läuft mit"
// list was built on a snapshot someone else has since replaced. Like the lock
// collision it is retriable and carries no conflicts list, but the retry needs
// a RELOAD first — so the client has to tell the two apart to show the right
// instruction.
const CodeCompanionsChanged = "companions_changed"

// companionPlanErrorRenderer maps the companion sentinels that a departure-plan
// write can raise to their wire response, and returns nil for every other error
// so the caller can keep classifying.
//
// StudentRepository.Update reconciles the "läuft mit" edges on behalf of every
// caller, so these are NOT specific to the student PUT: care-request approval
// and master-data review replace a departure plan too and hit exactly the same
// two conditions. Both are expected and user-actionable — routing them through
// a default branch would answer them with a 500. Every flow that can end up in
// StudentRepository.Update must consult this first (#1694).
func companionPlanErrorRenderer(err error) render.Renderer {
	switch {
	// Removing the day would leave the OTHER child with an accompanied
	// departure plan and no "mit wem" detail at all. Nothing was written; the
	// user has to give that child a note (or another link) first. The German
	// sentinel text goes straight to the UI, and the code lets the client tell
	// this expected refusal apart from any other 400 the write can produce.
	case errors.Is(err, usersService.ErrCompanionWouldLoseDeparture):
		return common.ErrorInvalidRequestWithCode(err, CodeCompanionWouldLoseDeparture)
	// A linked child was being edited elsewhere and this transaction could not
	// wait for its row without risking a deadlock. Nothing was written and the
	// same request succeeds once the other edit commits, so this is a retriable
	// conflict — never a 500. It carries a code because it shares its status
	// with the companion-plan question (which the client answers with
	// extend_companion_plans): without the code the client would ask the user
	// whether to widen another child's plan for what is really a lock collision.
	case errors.Is(err, usersService.ErrCompanionLockBusy):
		return common.ErrorConflictWithCode(err, CodeCompanionLockBusy)
	// The stored links no longer match the snapshot the submitted list replaces.
	// Nothing was written, and the same list must NOT simply be re-sent — it
	// would delete the change this refusal is protecting. A 409 with its own
	// code lets the client reload and let the user redo the edit on the current
	// state.
	case errors.Is(err, usersService.ErrCompanionsChanged):
		return common.ErrorConflictWithCode(err, CodeCompanionsChanged)
	}
	return nil
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

// UnmarshalJSON accepts the companion id as a JSON number OR as a quoted
// string.
//
// The frontend carries every backend int64 id as a string (CLAUDE.md type
// mapping) and would otherwise have to convert it back to a JS number to send
// it: an id beyond Number.MAX_SAFE_INTEGER then either rounds to a DIFFERENT
// child's id or fails the conversion outright, so the link could not be
// created, edited or removed at all. A quoted decimal is exact, so the string
// travels verbatim. Non-JS clients keep sending numbers.
func (e *CompanionEntry) UnmarshalJSON(data []byte) error {
	var raw struct {
		CompanionStudentID json.RawMessage `json:"companion_student_id"`
		Weekdays           []string        `json:"weekdays"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	e.Weekdays = raw.Weekdays
	e.CompanionStudentID = 0
	if len(raw.CompanionStudentID) == 0 || string(raw.CompanionStudentID) == "null" {
		return nil
	}
	if raw.CompanionStudentID[0] == '"' {
		var quoted string
		if err := json.Unmarshal(raw.CompanionStudentID, &quoted); err != nil {
			return err
		}
		quoted = strings.TrimSpace(quoted)
		if quoted == "" {
			// Same shape as an omitted id: Bind rejects it with the German
			// "Bitte ein Kind auswählen" sentinel instead of a parser message.
			return nil
		}
		id, err := strconv.ParseInt(quoted, 10, 64)
		if err != nil {
			return userModels.ErrCompanionStudentIDRequired
		}
		e.CompanionStudentID = id
		return nil
	}
	return json.Unmarshal(raw.CompanionStudentID, &e.CompanionStudentID)
}

// validateCompanionEntries bounds the submitted list at the request boundary.
//
// The cap is a service rule (ErrTooManyCompanions), but the service only sees
// the list AFTER lockCompanionRows has taken a FOR UPDATE lock on every
// submitted id. Without this check a group supervisor authorized to edit one
// child could name hundreds of other children of the school and lock all of
// them for the length of the transaction before earning the eventual 400 —
// blocking unrelated writes on rows they may not even read. Counting needs no
// database, so it belongs before the locks.
func validateCompanionEntries(entries *[]CompanionEntry) error {
	if entries == nil {
		return nil
	}
	if len(*entries) > usersService.MaxStudentCompanions {
		return usersService.ErrTooManyCompanions
	}
	return nil
}

// errCompanionsFingerprintRequired is returned when an HTTP caller submits a
// replacement list without saying what it replaces.
var errCompanionsFingerprintRequired = errors.New("companions_fingerprint is required when companions is present")

// validateCompanionsFingerprint requires the snapshot claim that makes the
// replacement semantics safe.
//
// ExpectedFingerprint is documented as optional in the service because system
// callers (trims, enrollment approval) derive the list from the stored state
// themselves and have nothing to compare against. For an HTTP caller that
// optionality is a hole: a nil fingerprint skips the comparison entirely, so
// two staff members editing the same child from the same snapshot both submit
// everything they saw and the second write deletes the first one's committed
// links — the exact lost update the fingerprint exists to prevent. Our own
// forms always send it; a stale build or a direct API client would not, and
// would silently get the unprotected path.
//
// The empty string is a valid claim, not a missing one: it is the fingerprint
// of an empty list, which is what a child with no Laufgemeinschaft yet loads.
// Only the absent key is refused, which is why this takes the pointer.
func validateCompanionsFingerprint(entries *[]CompanionEntry, fingerprint *string) error {
	if entries == nil {
		return nil
	}
	if fingerprint == nil {
		return errCompanionsFingerprintRequired
	}
	return nil
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

	links, err = rs.redactUnreadableCompanionNames(r, links)
	if err != nil {
		renderError(w, r, common.ErrorInternalServerWrap("failed to authorize departure companions", err))
		return
	}

	common.Respond(w, r, http.StatusOK, toCompanionResponses(links), "Departure companions retrieved")
}

// redactUnreadableCompanionNames strips the name of every linked child the
// caller may not read, leaving the id and the weekdays.
//
// Access to the SUBJECT is not access to the far end of its links. Since
// #2329 every verified staff member reads every child, so the redaction only
// still bites callers without full access (guest/guardian accounts) — for
// them the far end's first and last name are another child's personal data
// and must not ride along on this response.
//
// Redacting rather than dropping the entry keeps the link itself visible: the
// caller may see THAT this child walks home with someone (they may edit the
// list, and an entry that silently disappeared would be deleted on the next
// save), just not who. The response fields are `omitempty`, so a redacted
// companion arrives without name keys — the same shape the Kindersuche already
// handles for a companion it cannot resolve.
//
// The predicate is the one the student list uses for has_full_access
// (DetermineStudentAccess), resolved ONCE for the request, so N companions cost
// one extra student read and no extra permission lookups.
func (rs *Resource) redactUnreadableCompanionNames(r *http.Request, links []userModels.CompanionLink) ([]userModels.CompanionLink, error) {
	if len(links) == 0 {
		return links, nil
	}
	if err := rs.redactCompanionNames(r.Context(), rs.determineStudentAccess(r), links); err != nil {
		return nil, err
	}
	return links, nil
}

// redactCompanionNames is the shared redaction core behind
// redactUnreadableCompanionNames and the export enrichment: it strips the name
// of every linked child the caller may not read from ALL given link sets, in
// one bulk student read. The link slices are mutated in place.
func (rs *Resource) redactCompanionNames(ctx context.Context, accessCtx *studentAccessContext, linkSets ...[]userModels.CompanionLink) error {
	seen := make(map[int64]bool)
	var ids []int64
	for _, links := range linkSets {
		for _, link := range links {
			if !seen[link.CompanionStudentID] {
				seen[link.CompanionStudentID] = true
				ids = append(ids, link.CompanionStudentID)
			}
		}
	}
	if len(ids) == 0 {
		return nil
	}
	companions, err := rs.StudentService.GetByIDs(ctx, ids)
	if err != nil {
		return err
	}

	for _, links := range linkSets {
		for i := range links {
			// A companion that could not be loaded stays redacted: without its
			// group there is nothing to authorize against, and guessing in the
			// permissive direction would leak the very name this guards.
			if accessCtx.HasFullAccessToStudent(companions[links[i].CompanionStudentID]) {
				continue
			}
			links[i].FirstName = ""
			links[i].LastName = ""
		}
	}
	return nil
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
	// A request that cannot touch a link needs none of it. applyCompanionUpdate
	// still reconciles for such a request, but against a plan this request does
	// not write: the trim finds nothing to drop and no edge is created or
	// removed, so the only row it writes is the subject's own. Taking the graph
	// lock anyway would make a sick or excused toggle wait behind an unrelated
	// companion edit of a linked child — and the NOWAIT top-up pass could refuse
	// it outright with companion_lock_busy for a write that cannot touch a
	// companion at all. Every writer that creates or removes an edge touching
	// this child locks THIS child's row too (that is what LockCompanionGraph
	// guarantees), so the subject row lock this request takes anyway
	// (lockStudentForUpdate) already serializes us against them.
	if !req.touchesCompanions() {
		return nil
	}

	var submitted []int64
	if req.hasCompanionUpdate() {
		for _, entry := range *req.Companions {
			submitted = append(submitted, entry.CompanionStudentID)
		}
	}
	return rs.lockStudentCompanionGraph(ctx, student.ID, submitted)
}

// lockStudentCompanionGraph runs the shared companion lock protocol for this
// request's subject: the child, the submitted companions, and the far end of
// every stored edge, in one ascending-id pass. deleteStudent uses it too — its
// ON DELETE CASCADE removes every edge, which is a removal like any other.
//
// The protocol itself lives in services/users (LockCompanionGraph), because
// non-HTTP writers need the same order: the enrollment change-request approval
// writes SEVERAL children in one transaction and has to take their whole
// student set in this order, or its per-child locks invert against this one.
func (rs *Resource) lockStudentCompanionGraph(ctx context.Context, studentID int64, submitted []int64) error {
	return rs.StudentService.LockCompanionGraph(ctx, []int64{studentID}, submitted)
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
	//
	// The fingerprint travels into both: a plan-driven removal deletes links this
	// request never listed, so it may only proceed on a caller that says which
	// links it was looking at — otherwise a form holding a stale plan drops an
	// edge somebody else has just committed (see CheckCompanionTrim). The claim
	// is checked against the links stored under the row lock this request already
	// holds, so a mismatch is answered with a retriable 409 before anything is
	// written.
	if !req.hasCompanionUpdate() {
		return nil, rs.StudentService.CheckCompanionTrimForPlan(ctx, student.ID, accompaniedDays, req.CompanionsFingerprint)
	}
	if len(accompaniedDays) == 0 {
		// A NON-EMPTY list on a plan that allows "Anderes Kind" on no weekday is
		// contradictory input, not a removal. reconcileCompanions drops every
		// link for such a plan, so accepting it would answer 200 to a list that
		// was never written and never validated — weekday shape, self-links,
		// duplicates and unknown children are all checked inside
		// CheckCompanionConflicts, which this branch skips. Our own form clears
		// the list together with the last accompanied day, so only a stale or a
		// direct client gets here, and it has to be told rather than silently
		// obeyed. The EMPTY list stays allowed: it says the same thing the plan
		// does, and it is what the form sends when the user takes the mode away.
		if len(*req.Companions) > 0 {
			return nil, usersService.ErrCompanionDayNotAllowed
		}
		return nil, rs.StudentService.CheckCompanionTrimForPlan(ctx, student.ID, nil, req.CompanionsFingerprint)
	}

	conflicts, err := rs.StudentService.CheckCompanionConflicts(ctx, student.ID, usersService.CompanionUpdate{
		Links:                toCompanionLinks(*req.Companions),
		ExpectedFingerprint:  req.CompanionsFingerprint,
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
// departure plan that is about to be written and reports whether the write
// actually CHANGED the Laufgemeinschaft.
//
// It runs BEFORE the student write for two reasons: a link satisfies the
// accompanied-requires-a-note invariant that the model checks on write, and a
// conflict has to abort the transaction before anything lands.
//
// The reported flag decides whether the update announces
// student_companions_changed, so it must answer "did anything change", not "did
// the request carry plan fields". The Stammdaten forms resubmit the whole
// departure plan on every save, including a pure name or address edit; treating
// that as a companion change would mark every open "läuft mit" editor in the
// school stale and block its save for a change that never touched a link.
//
// `student` must already carry the plan resolved from this request;
// `authorizedExtensions` is what checkCompanionConflicts returned.
func (rs *Resource) applyCompanionUpdate(ctx context.Context, student *userModels.Student, req *UpdateStudentRequest, authorizedExtensions map[int64]map[string]bool) (bool, error) {
	// A request that cannot touch the links at all needs no before-state read:
	// the reconciliation below is a no-op trim against the child's unchanged
	// plan. Everything else is decided by comparing the stored links against
	// what this write leaves behind — both read under the subject's row lock,
	// which the caller already holds.
	if !req.touchesCompanions() {
		_, err := rs.reconcileCompanions(ctx, student, req, authorizedExtensions)
		return false, err
	}

	current, err := rs.StudentService.ListCompanions(ctx, student.ID)
	if err != nil {
		return false, err
	}

	links, err := rs.reconcileCompanions(ctx, student, req, authorizedExtensions)
	if err != nil {
		return false, err
	}

	// A widened companion plan counts too: it is that other child's departure
	// permission, shown on their card, changed by this request.
	changed := userModels.CompanionLinksFingerprint(current) != userModels.CompanionLinksFingerprint(links) ||
		len(authorizedExtensions) > 0
	return changed, nil
}

// reconcileCompanions performs the actual link write and returns the links the
// child is left with, so the caller can tell an effective change from a no-op.
func (rs *Resource) reconcileCompanions(ctx context.Context, student *userModels.Student, req *UpdateStudentRequest, authorizedExtensions map[int64]map[string]bool) ([]userModels.CompanionLink, error) {
	accompaniedDays := userModels.AccompaniedWeekdays(student.AllowedDepartureModes, student.DepartureDays)

	// The plan no longer allows leaving with another child anywhere: the links
	// lose their basis and go with it, exactly like the free-text note does.
	// Otherwise the Kindersuche would keep grouping a child whose Stammdaten say
	// "fährt Bus".
	if len(accompaniedDays) == 0 {
		if _, err := rs.StudentService.ReplaceCompanions(ctx, student.ID, usersService.CompanionUpdate{
			AccompaniedDays: accompaniedDays,
		}); err != nil {
			return nil, err
		}
		student.DepartureCompanionDays = nil
		return nil, nil
	}

	if !req.hasCompanionUpdate() {
		// The client changed the plan but not the list. Links on a weekday the
		// new plan no longer allows lose their basis and are dropped — keeping
		// them would let the Kindersuche group the children on a day the
		// Stammdaten forbid. What survives still answers "mit wem", so the note
		// stays optional for a child that has a Laufgemeinschaft left.
		//
		// That this request is entitled to drop them was settled by
		// checkCompanionConflicts, which verified the caller's baseline claim
		// against the stored links. No re-check here: the companion graph lock
		// taken before that check is held for the rest of the transaction, and
		// every writer that adds an edge touching this child takes this child's
		// row too, so the links cannot have moved in between.
		links, err := rs.StudentService.TrimCompanionsToDays(ctx, student.ID, accompaniedDays)
		if err != nil {
			return nil, err
		}
		student.DepartureCompanionDays = userModels.CompanionDaysFromLinks(links)
		return links, nil
	}

	links := toCompanionLinks(*req.Companions)
	conflicts, err := rs.StudentService.ReplaceCompanions(ctx, student.ID, usersService.CompanionUpdate{
		Links:                links,
		ExpectedFingerprint:  req.CompanionsFingerprint,
		AccompaniedDays:      accompaniedDays,
		ExtendCompanionPlans: req.ExtendCompanionPlans,
		AuthorizedExtensions: authorizedExtensions,
		ActorAccountID:       int64(jwt.ClaimsFromCtx(ctx).ID),
	})
	if err != nil {
		return nil, err
	}
	if len(conflicts) > 0 {
		return nil, &companionConflictError{Conflicts: conflicts}
	}

	student.DepartureCompanionDays = userModels.CompanionDaysFromLinks(links)
	return links, nil
}

// enrichWithCompanions fills CompanionStudentIDs for the given day with the
// child's whole Laufgemeinschaft (the connected component, see
// CompanionIDsForWeekday) — the page cannot derive it from direct links alone
// when a bridging child is filtered out.
//
// Only the ids travel, not the names: the Kindersuche already knows every child
// on the page, so it can resolve a companion that is visible and quietly ignore
// one that is filtered out. Shipping names here would leak children the caller
// filtered away.
//
// A failure is NOT swallowed: the caller asked for the grouping, and empty
// companion ids are indistinguishable from "this child walks alone". The
// Kindersuche would file every linked child under "Ohne Laufgemeinschaft" and
// present a transient query failure as a real departure arrangement, so the
// request fails instead.
func (rs *Resource) enrichWithCompanions(ctx context.Context, responses []StudentResponse, params *studentListParams, day time.Time) error {
	if !params.includeCompanions {
		return nil
	}

	studentIDs := collectFullAccessStudentIDs(responses)
	if len(studentIDs) == 0 {
		return nil
	}

	weekday := companionWeekdayForDate(day)
	byStudent, err := rs.StudentService.CompanionIDsForWeekday(ctx, studentIDs, weekday)
	if err != nil {
		rs.Logger.Error("failed to load departure companions for list",
			slog.String("error", err.Error()),
			slog.Int("weekday", weekday),
		)
		return err
	}

	for i := range responses {
		if companions, ok := byStudent[responses[i].ID]; ok && len(companions) > 0 {
			responses[i].CompanionStudentIDs = companions
		}
	}
	return nil
}

// enrichWithCompanionLinks fills DepartureCompanions with each child's own
// "läuft mit" links, for the offline lists that have to print WHO a child walks
// home with.
//
// Unlike enrichWithCompanions (ids of the whole Laufgemeinschaft, for the
// Kindersuche grouping) this carries NAMES, so it is restricted to the children
// the caller has full access to — the same gate the grouping uses. That gate
// covers only the SUBJECT of each link: for a caller without full access
// (guest/guardian, #2329) the far end's name must not ride into the exported
// document either — the same per-companion redaction getStudentCompanions
// applies runs here before any link is attached to a response.
//
// A failure is returned, not swallowed. The caller decides: an export whose
// columns include the departure plan MUST abort, because a child whose
// accompanied days are backed only by links legitimately has no free-text note,
// so departureExportCell would print a bare "Mit anderem Kind" and hand staff a
// paper pickup list without a single name. Exports that do not carry the
// departure column lose nothing and may continue.
func (rs *Resource) enrichWithCompanionLinks(ctx context.Context, responses []StudentResponse, accessCtx *studentAccessContext) error {
	studentIDs := collectFullAccessStudentIDs(responses)
	if len(studentIDs) == 0 {
		return nil
	}

	byStudent, err := rs.StudentService.ListCompanionsForStudents(ctx, studentIDs)
	if err != nil {
		rs.Logger.Error("failed to load departure companions for export",
			slog.String("error", err.Error()),
		)
		return err
	}

	// Redaction failing must NOT fall through to the unredacted names — the
	// column is optional, the far-end names are another child's personal data.
	linkSets := make([][]userModels.CompanionLink, 0, len(byStudent))
	for _, links := range byStudent {
		linkSets = append(linkSets, links)
	}
	if err := rs.redactCompanionNames(ctx, accessCtx, linkSets...); err != nil {
		rs.Logger.Error("failed to authorize departure companions for export",
			slog.String("error", err.Error()),
		)
		return err
	}

	for i := range responses {
		if links, ok := byStudent[responses[i].ID]; ok && len(links) > 0 {
			responses[i].DepartureCompanions = links
		}
	}
	return nil
}

// companionWeekdayForDate maps a date onto the stored 1..5 weekday.
//
// Saturday and Sunday fall back to Monday rather than resolving to "no
// weekday": OGS care does not run on the weekend, so a staff member opening
// the Kindersuche on a Sunday is looking ahead at the coming week. Returning
// nothing there would make the Laufgemeinschaft grouping look broken (every
// child in "Ohne Laufgemeinschaft") for a reason no one can see.
//
// The instant is moved to Berlin first: it comes from rs.Now, which is
// time.Now in production (a container clock that is normally UTC) and an
// arbitrary injected clock in tests. Reading Weekday() off that location would
// spend the late Berlin evening — already the next calendar day here — querying
// yesterday's links and grouping the children by the wrong day's
// Laufgemeinschaft.
func companionWeekdayForDate(day time.Time) int {
	switch day.In(timezone.Berlin).Weekday() {
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
