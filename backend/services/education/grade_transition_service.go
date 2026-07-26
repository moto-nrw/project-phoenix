package education

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	activeModels "github.com/moto-nrw/project-phoenix/models/active"
	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

// ErrGraduatesCheckedIn is returned by Apply when one or more graduating
// students are still checked in. It is a client-recoverable condition (the
// admin must check the children out first), so handlers map it to 409 Conflict
// — never a 500 (#405).
var ErrGraduatesCheckedIn = errors.New("graduating students are still checked in")

// ErrNotLatestApplied is returned by Revert when the caller targets an applied
// transition that is not the most recently applied one. Reverts must unwind in
// strict reverse order — replaying an older transition's history would overwrite
// classes or statuses a newer transition (or a manual edit) has since written.
// It is a client-recoverable condition (refresh and revert the latest first), so
// handlers map it to 409 Conflict, never a 500 (#405).
var ErrNotLatestApplied = errors.New("only the most recently applied transition can be reverted")

// ErrPreviewStale is returned by Apply/ApplyChecked when the cohort the apply
// would act on is no longer the one it was asked to act on — another admin
// changed the mappings, moved a child between classes after the preview was
// rendered, or moved one INTO a mapped class after the cohort was locked (see
// ensureNoLateArrivals; that case is refused even without a fingerprint,
// because the alternative is a transition marked applied that skipped a child).
// Applying anyway would perform a destructive bulk change the admin never saw,
// or only part of the one they did, so the request is refused and the UI reloads
// the preview for a fresh confirmation. Client-recoverable, hence 409 Conflict,
// never a 500 (#405).
var ErrPreviewStale = errors.New("the preview is out of date: the affected classes or children have changed")

// ErrTransitionNotDraft is returned by Update/Delete/Apply when the targeted
// transition is no longer a draft — another admin applied (or reverted) it
// between the editor's load and this write. A normal concurrent-edit outcome,
// not a server fault: handlers map it to 409 Conflict so the UI refreshes the
// stale editor instead of showing a generic failure (#405 review).
var ErrTransitionNotDraft = errors.New("must be in draft status")

// ErrTransitionNotApplied is returned by Revert when the targeted transition
// is not in applied status — it is still a draft, or another admin already
// reverted it between the list's load and this request. Like the other stale-
// state outcomes above it is client-recoverable, so handlers map it to 409
// Conflict and the UI reloads the list instead of offering a retry that can
// never succeed (#405 review).
var ErrTransitionNotApplied = errors.New("must be in applied status")

// ErrInvalidTransitionData wraps client-correctable validation failures in a
// submitted transition or its mappings (bad academic-year format, empty
// from_class, from_class equal to to_class). Handlers map it to 400 Bad
// Request so the admin can fix the input, never a 500 (#405 review).
var ErrInvalidTransitionData = errors.New("invalid transition data")

// CreateTransitionRequest contains data for creating a new transition
type CreateTransitionRequest struct {
	AcademicYear string           `json:"academic_year"`
	Notes        *string          `json:"notes,omitempty"`
	Mappings     []MappingRequest `json:"mappings"`
	CreatedBy    int64            `json:"-"` // Set from JWT context
}

// UpdateTransitionRequest contains data for updating a transition
type UpdateTransitionRequest struct {
	AcademicYear *string          `json:"academic_year,omitempty"`
	Notes        *string          `json:"notes,omitempty"`
	Mappings     []MappingRequest `json:"mappings,omitempty"`
}

// MappingRequest represents a class-to-class mapping in a request
type MappingRequest struct {
	FromClass string  `json:"from_class"`
	ToClass   *string `json:"to_class"` // null = graduate/delete
}

// TransitionPreview contains information about what will happen when applied
type TransitionPreview struct {
	TransitionID    int64               `json:"transition_id,string"`
	AcademicYear    string              `json:"academic_year"`
	TotalStudents   int                 `json:"total_students"`
	ToPromote       int                 `json:"to_promote"`
	ToGraduate      int                 `json:"to_graduate"`
	ByMapping       []MappingPreview    `json:"by_mapping"`
	UnmappedClasses []UnmappedClassInfo `json:"unmapped_classes"`
	Warnings        []string            `json:"warnings"`
	// Fingerprint identifies the exact cohort this preview describes: the
	// mappings plus every affected child and the class they are in. Apply
	// accepts it back as expected_fingerprint and refuses (ErrPreviewStale) when
	// the data has changed since, so an admin can never confirm one set of
	// children and graduate another (#405 review).
	Fingerprint string `json:"fingerprint"`
}

// MappingPreview shows the impact of a single mapping
type MappingPreview struct {
	FromClass    string  `json:"from_class"`
	ToClass      *string `json:"to_class,omitempty"`
	StudentCount int     `json:"student_count"`
	Action       string  `json:"action"` // "promote" or "graduate"
}

// UnmappedClassInfo shows classes not included in the transition
type UnmappedClassInfo struct {
	ClassName    string `json:"class_name"`
	StudentCount int    `json:"student_count"`
}

// TransitionResult contains the result of applying or reverting a transition
type TransitionResult struct {
	TransitionID      int64    `json:"transition_id,string"`
	Status            string   `json:"status"`
	StudentsPromoted  int      `json:"students_promoted"`
	StudentsGraduated int      `json:"students_graduated"`
	CanRevert         bool     `json:"can_revert"`
	Warnings          []string `json:"warnings"`
}

// SuggestedMapping represents an auto-suggested class mapping
type SuggestedMapping struct {
	FromClass    string  `json:"from_class"`
	ToClass      *string `json:"to_class,omitempty"`
	StudentCount int     `json:"student_count"`
	IsGraduating bool    `json:"is_graduating"`
	// Ambiguous marks a class whose name does not match the grade pattern, so
	// the graduation guess is not confident. The editor must not preselect
	// Abgang for these — it defaults them to an explicit, non-destructive
	// choice so free-form or placeholder classes ("offen") aren't mass-graduated.
	Ambiguous bool `json:"ambiguous,omitempty"`
}

// Error message format constants
const errFmtTransitionNotFound = "transition not found: %w"

// RosterReconciler adjusts already-materialized FUTURE timetable rosters when a
// grade transition graduates or restores students. The materializer is
// insert-only, so a graduation applied after upcoming instances were
// materialized leaves the departed children on those rosters, and a revert
// cannot re-add a child to an instance materialized while they were an alumnus.
// Implemented by services/schedule.RosterReconciler; optional so unit tests that
// do not exercise timetable reconciliation can leave it nil (#405 review).
type RosterReconciler interface {
	// RemoveStudentsFromFutureRosters drops the graduated students from
	// already-materialized rosters, archiving every removed row under
	// transitionID so the revert can replay exactly those rows.
	RemoveStudentsFromFutureRosters(ctx context.Context, transitionID int64, studentIDs []int64) error
	// RestoreStudentsToFutureRosters replays the archived rows and additionally
	// fills instances inserted after baselineInstanceID (the marker the apply
	// recorded) from enrollments — those are the instances the materializer
	// skipped while the children were alumni. A nil marker skips that fill.
	RestoreStudentsToFutureRosters(ctx context.Context, transitionID int64, studentIDs []int64, baselineInstanceID *int64) error
	// CurrentRosterBaseline returns the marker to record while applying, so the
	// revert can tell pre-existing instances from ones materialized during the
	// alumnus window.
	CurrentRosterBaseline(ctx context.Context) (int64, error)
}

// GradeTransitionService manages grade transitions: CRUD, preview/apply,
// revert, and mapping utilities.
type GradeTransitionService struct {
	transitionRepo   education.GradeTransitionRepository
	studentRepo      users.StudentRepository
	personRepo       users.PersonRepository
	visitRepo        activeModels.VisitRepository
	attendanceRepo   activeModels.AttendanceRepository
	rosterReconciler RosterReconciler
	db               *bun.DB
}

// GradeTransitionServiceDependencies contains dependencies for the service
type GradeTransitionServiceDependencies struct {
	TransitionRepo education.GradeTransitionRepository
	StudentRepo    users.StudentRepository
	PersonRepo     users.PersonRepository
	// VisitRepo and AttendanceRepo guard graduation against stranding a
	// checked-in child (open visit or open attendance record). Optional so
	// tests that don't exercise the check can leave them nil.
	VisitRepo      activeModels.VisitRepository
	AttendanceRepo activeModels.AttendanceRepository
	// RosterReconciler reconciles already-materialized future timetable rosters
	// on apply (drop graduates) and revert (restore reactivated students).
	// Optional; nil disables reconciliation for tests that don't exercise it.
	RosterReconciler RosterReconciler
	DB               *bun.DB
}

// NewGradeTransitionService creates a new grade transition service
func NewGradeTransitionService(deps GradeTransitionServiceDependencies) *GradeTransitionService {
	return &GradeTransitionService{
		transitionRepo:   deps.TransitionRepo,
		studentRepo:      deps.StudentRepo,
		personRepo:       deps.PersonRepo,
		visitRepo:        deps.VisitRepo,
		attendanceRepo:   deps.AttendanceRepo,
		rosterReconciler: deps.RosterReconciler,
		db:               deps.DB,
	}
}

// Create creates a new grade transition with mappings
func (s *GradeTransitionService) Create(ctx context.Context, req CreateTransitionRequest) (*education.GradeTransition, error) {
	if req.AcademicYear == "" {
		return nil, fmt.Errorf("%w: academic_year is required", ErrInvalidTransitionData)
	}

	transition := &education.GradeTransition{
		AcademicYear: req.AcademicYear,
		Status:       education.TransitionStatusDraft,
		CreatedBy:    req.CreatedBy,
		Notes:        req.Notes,
	}

	if err := transition.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidTransitionData, err)
	}
	if err := validateMappingRequests(req.Mappings); err != nil {
		return nil, err
	}

	transition.SetTenantID(tenant.FromContext(ctx))

	if err := s.transitionRepo.Create(ctx, transition); err != nil {
		return nil, fmt.Errorf("failed to create transition: %w", err)
	}

	if err := s.createMappingsIfProvided(ctx, transition, req.Mappings); err != nil {
		return nil, err
	}

	return transition, nil
}

// createMappingsIfProvided creates mappings for a transition if any are provided.
func (s *GradeTransitionService) createMappingsIfProvided(
	ctx context.Context,
	transition *education.GradeTransition,
	reqMappings []MappingRequest,
) error {
	if len(reqMappings) == 0 {
		return nil
	}

	mappings, err := s.buildMappings(transition.ID, reqMappings)
	if err != nil {
		return err
	}

	tenantID := tenant.FromContext(ctx)
	for _, m := range mappings {
		m.SetTenantID(tenantID)
	}

	if err := s.transitionRepo.CreateMappings(ctx, mappings); err != nil {
		return fmt.Errorf("failed to create mappings: %w", err)
	}
	transition.Mappings = mappings
	return nil
}

// buildMappings validates and builds mapping entities from input.
func (s *GradeTransitionService) buildMappings(transitionID int64, inputs []MappingRequest) ([]*education.GradeTransitionMapping, error) {
	mappings := make([]*education.GradeTransitionMapping, 0, len(inputs))
	for _, m := range inputs {
		mapping := &education.GradeTransitionMapping{
			TransitionID: transitionID,
			FromClass:    m.FromClass,
			ToClass:      m.ToClass,
		}
		if err := mapping.Validate(); err != nil {
			return nil, fmt.Errorf("%w: invalid mapping for class %s: %w", ErrInvalidTransitionData, m.FromClass, err)
		}
		mappings = append(mappings, mapping)
	}
	return mappings, nil
}

// validateMappingRequests validates replacement mappings before any transition
// or mapping rows are changed. A request rejected as invalid must leave an
// existing draft intact, even when the caller's transaction commits 4xx errors.
func validateMappingRequests(inputs []MappingRequest) error {
	fromClasses := make(map[string]struct{}, len(inputs))
	for _, input := range inputs {
		mapping := &education.GradeTransitionMapping{
			TransitionID: 1, // Validation requires a positive ID; it is assigned on persistence.
			FromClass:    input.FromClass,
			ToClass:      input.ToClass,
		}
		if err := mapping.Validate(); err != nil {
			return fmt.Errorf("%w: invalid mapping for class %s: %w", ErrInvalidTransitionData, input.FromClass, err)
		}
		if _, duplicate := fromClasses[mapping.FromClass]; duplicate {
			return fmt.Errorf("%w: duplicate mapping for class %s", ErrInvalidTransitionData, mapping.FromClass)
		}
		fromClasses[mapping.FromClass] = struct{}{}
	}
	return nil
}

// Update updates a grade transition
func (s *GradeTransitionService) Update(ctx context.Context, id int64, req UpdateTransitionRequest) (*education.GradeTransition, error) {
	// Take the tenant transition gate BEFORE reading the row. An apply loads the
	// mappings, validates the admin's confirmed fingerprint against them, and
	// then mutates students, writes history and records the revert metadata from
	// that same set — none of which locks the mapping rows. Without this gate a
	// concurrent mapping replacement commits in that window, and the transition
	// ends up marked applied carrying zuordnungen that the history rows and the
	// classes students were actually moved to never came from, so a later revert
	// replays a different transition than the one that ran (#405 review).
	//
	// Only the transition gate is taken here (an update touches no
	// recurrence-derived state), so the acquisition order stays acyclic against
	// apply/revert, which take the recurrence gate first — see
	// lockRecurrenceThenTransitions. Whichever side loses the race sees the
	// other's committed status: a draft edit behind an apply is refused by
	// CanModify, an apply behind an edit re-reads the new mappings.
	if err := s.transitionRepo.LockTenantTransitions(ctx); err != nil {
		return nil, err
	}

	transition, err := s.transitionRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf(errFmtTransitionNotFound, err)
	}

	if !transition.CanModify() {
		return nil, fmt.Errorf("cannot modify transition: %w", ErrTransitionNotDraft)
	}
	if err := validateMappingRequests(req.Mappings); err != nil {
		return nil, err
	}

	applyTransitionUpdates(transition, req)

	if err := transition.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidTransitionData, err)
	}

	if err := s.transitionRepo.Update(ctx, transition); err != nil {
		return nil, fmt.Errorf("failed to update transition: %w", err)
	}

	if err := s.replaceMappingsIfProvided(ctx, transition, req.Mappings); err != nil {
		return nil, err
	}

	return s.transitionRepo.FindByIDWithMappings(ctx, id)
}

// applyTransitionUpdates applies request fields to the transition.
func applyTransitionUpdates(transition *education.GradeTransition, req UpdateTransitionRequest) {
	if req.AcademicYear != nil {
		transition.AcademicYear = *req.AcademicYear
	}
	if req.Notes != nil {
		transition.Notes = req.Notes
	}
}

// replaceMappingsIfProvided deletes existing mappings and creates new ones if provided.
func (s *GradeTransitionService) replaceMappingsIfProvided(
	ctx context.Context,
	transition *education.GradeTransition,
	reqMappings []MappingRequest,
) error {
	if reqMappings == nil {
		return nil
	}

	if err := s.transitionRepo.DeleteMappings(ctx, transition.ID); err != nil {
		return fmt.Errorf("failed to delete existing mappings: %w", err)
	}

	if len(reqMappings) == 0 {
		transition.Mappings = nil
		return nil
	}

	mappings, err := s.buildMappings(transition.ID, reqMappings)
	if err != nil {
		return err
	}

	tenantID := tenant.FromContext(ctx)
	for _, m := range mappings {
		m.SetTenantID(tenantID)
	}

	if err := s.transitionRepo.CreateMappings(ctx, mappings); err != nil {
		return fmt.Errorf("failed to create mappings: %w", err)
	}
	transition.Mappings = mappings
	return nil
}

// Delete deletes a draft grade transition
func (s *GradeTransitionService) Delete(ctx context.Context, id int64) error {
	// Same gate as Update, for the same reason: deleting the draft cascades its
	// mappings away, and an apply that already read them must not find the row
	// gone underneath it (#405 review).
	if err := s.transitionRepo.LockTenantTransitions(ctx); err != nil {
		return err
	}

	transition, err := s.transitionRepo.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf(errFmtTransitionNotFound, err)
	}

	if !transition.CanModify() {
		return fmt.Errorf("cannot delete transition: %w", ErrTransitionNotDraft)
	}

	return s.transitionRepo.Delete(ctx, id)
}

// GetByID retrieves a grade transition by ID with mappings
func (s *GradeTransitionService) GetByID(ctx context.Context, id int64) (*education.GradeTransition, error) {
	return s.transitionRepo.FindByIDWithMappings(ctx, id)
}

// List retrieves grade transitions with pagination. Mappings are hydrated so
// every listed draft renders its zuordnungen and a correct can_apply, and the
// editor opens pre-filled — the bare list query returns no mappings otherwise.
func (s *GradeTransitionService) List(ctx context.Context, options *base.QueryOptions) ([]*education.GradeTransition, int, error) {
	transitions, total, err := s.transitionRepo.List(ctx, options)
	if err != nil {
		return nil, 0, err
	}

	if len(transitions) > 0 {
		ids := make([]int64, 0, len(transitions))
		for _, t := range transitions {
			ids = append(ids, t.ID)
		}
		byTransition, err := s.transitionRepo.GetMappingsByTransitionIDs(ctx, ids)
		if err != nil {
			return nil, 0, err
		}
		for _, t := range transitions {
			t.Mappings = byTransition[t.ID]
		}
	}

	return transitions, total, nil
}

// Preview returns what will happen when the transition is applied.
//
// The displayed counts and the fingerprint that binds the admin's confirmation
// MUST describe the same cohort, so both are derived from ONE query. Under READ
// COMMITTED a second query sees a newer snapshot: counting per class and then
// re-reading the cohort for the fingerprint could show "24 children graduate"
// while fingerprinting the 25 that Apply then recomputes, accepts and graduates
// — the confirmation would cover children the modal never displayed, which is
// exactly what the fingerprint exists to prevent (#405 review).
func (s *GradeTransitionService) Preview(ctx context.Context, id int64) (*TransitionPreview, error) {
	transition, err := s.transitionRepo.FindByIDWithMappings(ctx, id)
	if err != nil {
		return nil, fmt.Errorf(errFmtTransitionNotFound, err)
	}

	preview := &TransitionPreview{
		TransitionID: id,
		AcademicYear: transition.AcademicYear,
		ByMapping:    make([]MappingPreview, 0),
		Warnings:     make([]string, 0),
	}

	cohort, err := s.transitionRepo.GetStudentsByClasses(ctx, mappedClassNames(transition.Mappings))
	if err != nil {
		return nil, fmt.Errorf("failed to resolve preview cohort: %w", err)
	}

	mappedClasses := s.buildMappingPreviews(preview, transition.Mappings, countByClass(cohort))

	if err := s.findUnmappedClasses(ctx, preview, mappedClasses); err != nil {
		return nil, err
	}

	preview.Fingerprint = fingerprintOf(transition.Mappings, cohort)

	s.addPreviewWarnings(preview)

	return preview, nil
}

// mappedClassNames lists the distinct source classes a transition touches — the
// classes whose children the preview counts and the fingerprint covers.
func mappedClassNames(mappings []*education.GradeTransitionMapping) []string {
	seen := make(map[string]bool, len(mappings))
	classes := make([]string, 0, len(mappings))
	for _, m := range mappings {
		if seen[m.FromClass] {
			continue
		}
		seen[m.FromClass] = true
		classes = append(classes, m.FromClass)
	}
	return classes
}

// countByClass tallies a resolved cohort per school class, so the per-mapping
// counts a preview displays come from the very rows it fingerprints.
func countByClass(cohort []*education.StudentClassInfo) map[string]int {
	counts := make(map[string]int)
	for _, st := range cohort {
		counts[st.SchoolClass]++
	}
	return counts
}

// ensureFingerprintMatches compares the caller's previewed cohort against the
// one resolved under the locks. An empty expectation means the caller has no
// preview to be stale (CLI, direct API use) and skips the check.
func ensureFingerprintMatches(
	expected string,
	mappings []*education.GradeTransitionMapping,
	promotions, graduates []*education.StudentClassInfo,
) error {
	if expected == "" {
		return nil
	}
	current := make([]*education.StudentClassInfo, 0, len(promotions)+len(graduates))
	current = append(current, promotions...)
	current = append(current, graduates...)

	if fingerprintOf(mappings, current) != expected {
		return ErrPreviewStale
	}
	return nil
}

// fingerprintOf builds the stable digest both Preview and Apply compute. The
// inputs are sorted, so neither query's row order can change the result.
//
// The mapping action is encoded as its own field instead of being inferred from
// a "graduate" target sentinel: a school may legitimately name a class
// "graduate", and with a sentinel that promotion serialized exactly like the
// graduation of the same source class. Editing one into the other between
// preview and apply then left the digest unchanged, so the stale-preview guard
// waved through a bulk action the admin never confirmed — the destructive
// direction of the pair. Every free-text field is quoted (strconv.Quote escapes
// any embedded quote or backslash), so no class name can spoof the separators
// either (#405 review).
func fingerprintOf(mappings []*education.GradeTransitionMapping, students []*education.StudentClassInfo) string {
	parts := make([]string, 0, len(mappings)+len(students))
	for _, m := range mappings {
		if m.ToClass == nil {
			parts = append(parts, fmt.Sprintf("m|graduate|%s", strconv.Quote(m.FromClass)))
			continue
		}
		parts = append(parts, fmt.Sprintf("m|promote|%s|%s",
			strconv.Quote(m.FromClass), strconv.Quote(*m.ToClass)))
	}
	for _, st := range students {
		parts = append(parts, fmt.Sprintf("s|%d|%s", st.StudentID, strconv.Quote(st.SchoolClass)))
	}
	sort.Strings(parts)

	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return hex.EncodeToString(sum[:])
}

// buildMappingPreviews populates preview with mapping details and returns mapped
// class names. Counts come from the caller's single cohort snapshot rather than
// a per-class query, so they cannot disagree with the fingerprint (#405 review).
func (s *GradeTransitionService) buildMappingPreviews(
	preview *TransitionPreview,
	mappings []*education.GradeTransitionMapping,
	countsByClass map[string]int,
) map[string]bool {
	mappedClasses := make(map[string]bool)

	for _, mapping := range mappings {
		count := countsByClass[mapping.FromClass]

		mp := s.createMappingPreview(mapping, count)
		if mapping.IsGraduating() {
			preview.ToGraduate += count
		} else {
			preview.ToPromote += count
		}

		preview.ByMapping = append(preview.ByMapping, mp)
		preview.TotalStudents += count
		mappedClasses[mapping.FromClass] = true
	}

	return mappedClasses
}

// createMappingPreview creates a MappingPreview from a mapping and student count.
func (s *GradeTransitionService) createMappingPreview(mapping *education.GradeTransitionMapping, count int) MappingPreview {
	action := "promote"
	if mapping.IsGraduating() {
		action = "graduate"
	}
	return MappingPreview{
		FromClass:    mapping.FromClass,
		ToClass:      mapping.ToClass,
		StudentCount: count,
		Action:       action,
	}
}

// findUnmappedClasses finds classes with students that are not in the transition.
func (s *GradeTransitionService) findUnmappedClasses(
	ctx context.Context,
	preview *TransitionPreview,
	mappedClasses map[string]bool,
) error {
	allClasses, err := s.transitionRepo.GetDistinctClasses(ctx)
	if err != nil {
		return fmt.Errorf("failed to get distinct classes: %w", err)
	}

	for _, className := range allClasses {
		if mappedClasses[className] {
			continue
		}
		count, err := s.transitionRepo.GetStudentCountByClass(ctx, className)
		if err != nil {
			return fmt.Errorf("failed to count students in unmapped class %s: %w", className, err)
		}
		if count == 0 {
			continue
		}
		preview.UnmappedClasses = append(preview.UnmappedClasses, UnmappedClassInfo{
			ClassName:    className,
			StudentCount: count,
		})
	}
	return nil
}

// addPreviewWarnings adds warning messages to the preview.
func (s *GradeTransitionService) addPreviewWarnings(preview *TransitionPreview) {
	if len(preview.UnmappedClasses) > 0 {
		preview.Warnings = append(preview.Warnings,
			fmt.Sprintf("%d classes with students are not included in this transition",
				len(preview.UnmappedClasses)))
	}
	if preview.ToGraduate > 0 {
		preview.Warnings = append(preview.Warnings,
			fmt.Sprintf("%d students will be marked as alumni and hidden from the app (graduates)",
				preview.ToGraduate))
	}
}

// Apply executes the grade transition without a staleness check. Kept for
// callers that have no preview in hand (CLI/tests); the HTTP layer uses
// ApplyChecked so the admin's confirmation is bound to what they were shown.
func (s *GradeTransitionService) Apply(ctx context.Context, id int64, accountID int64) (*TransitionResult, error) {
	return s.ApplyChecked(ctx, id, accountID, "")
}

// ApplyChecked executes the grade transition, refusing with ErrPreviewStale when
// expectedFingerprint is set and no longer matches the cohort resolved under the
// locks. An empty fingerprint skips the check (see Apply).
func (s *GradeTransitionService) ApplyChecked(
	ctx context.Context, id int64, accountID int64, expectedFingerprint string,
) (*TransitionResult, error) {
	// Serialize against a concurrent apply/revert for this tenant BEFORE reading
	// anything: the class snapshots and student locks taken below must not be
	// invalidated by an interleaved revert that rewrites the same classes and
	// statuses (#405 review). The same lock is taken by the timetable
	// materializer, so a materialization pass can neither insert an upcoming
	// roster row for a child this apply is about to graduate (from an enrollment
	// it read while they were still active) nor race the reconciliation below.
	// Both locks release at COMMIT/ROLLBACK.
	if err := s.lockRecurrenceThenTransitions(ctx); err != nil {
		return nil, err
	}

	transition, err := s.transitionRepo.FindByIDWithMappings(ctx, id)
	if err != nil {
		return nil, fmt.Errorf(errFmtTransitionNotFound, err)
	}

	if err := validateCanApply(transition); err != nil {
		return nil, err
	}

	result := &TransitionResult{
		TransitionID: id,
		Warnings:     make([]string, 0),
	}

	if err := s.executeApply(ctx, transition, accountID, expectedFingerprint, result); err != nil {
		return nil, err
	}

	finalizeApplyResult(result)
	return result, nil
}

// lockRecurrenceThenTransitions takes the three tenant-wide gates an
// apply/revert needs, in the project-wide order: the student class-writes gate
// FIRST, then the recurrence gate, then the grade transition gate.
//
// The class-writes gate comes first because it is the one an ordinary request
// can also hold: a create-student request takes it (shared, inside the student
// repository) and may afterwards touch recurrence-derived state, so an apply
// holding the recurrence gate and then asking for the class-writes gate would
// close the cycle. Taking it up front keeps the order acyclic in both
// directions.
//
// Order matters because both operations mutate recurrence-derived roster state.
// A re-plan (services/schedule.ReplanWeek) holds the recurrence gate, deletes
// planned instances — locking their instance_students rows — and only then asks
// for the transition gate. If an apply held the transition gate and then waited
// on one of those very rows during its archive pass, PostgreSQL would detect a
// deadlock and abort one administrator's request. Taking the recurrence gate up
// front makes the acquisition order acyclic (#405 review).
//
// Both gates are taken through the repository, which owns the shared key
// strings (models/schedule.TenantRecurrenceLockKey and
// education.TenantTransitionsLockKey) — services/schedule cannot be imported
// here without an import cycle.
func (s *GradeTransitionService) lockRecurrenceThenTransitions(ctx context.Context) error {
	// Shuts the one window the row locks below cannot reach: a child CREATED in
	// a mapped class, or moved in from an unmapped one, has no row to lock, so
	// ensureNoLateArrivals can only catch arrivals that committed before its
	// re-read. Holding this gate for the rest of the transaction means an
	// arrival either lands before the re-read (and is refused with
	// ErrPreviewStale, so the admin re-previews the complete cohort) or waits
	// until the transition has committed — never in between, where it would be
	// skipped by writes that only touch the locked ids while the transition
	// still reports success (#405 review).
	//
	// Nil only in pure unit tests that wire no student repository; those take no
	// row locks either, so there is no window to close.
	if s.studentRepo != nil {
		if err := s.studentRepo.LockStudentClassWrites(ctx); err != nil {
			return err
		}
	}
	if err := s.transitionRepo.LockTenantRecurrenceWrites(ctx); err != nil {
		return err
	}
	return s.transitionRepo.LockTenantTransitions(ctx)
}

// validateCanApply checks if a transition can be applied. Every refusal wraps
// ErrTransitionNotDraft: they are all the same stale-state outcome (the draft
// the admin loaded has since been applied, reverted, or emptied), which the
// handler must answer with 409, not 500 (#405 review).
func validateCanApply(transition *education.GradeTransition) error {
	if transition.CanApply() {
		return nil
	}
	if transition.IsApplied() {
		return fmt.Errorf("%w: transition has already been applied", ErrTransitionNotDraft)
	}
	if transition.IsReverted() {
		return fmt.Errorf("%w: transition has been reverted", ErrTransitionNotDraft)
	}
	return fmt.Errorf("cannot apply transition: %w with mappings", ErrTransitionNotDraft)
}

// executeApply performs the actual apply within a transaction.
func (s *GradeTransitionService) executeApply(
	ctx context.Context,
	transition *education.GradeTransition,
	accountID int64,
	expectedFingerprint string,
	result *TransitionResult,
) error {
	promoteClasses, graduateClasses := categorizeMappings(transition.Mappings)

	// Resolve BOTH definitive cohorts ONCE and hold a FOR UPDATE lock on every
	// row. The check-in guard, the history snapshot, and the class/status writes
	// all operate on exactly these locked sets, so the rows checked, recorded,
	// and mutated are identical — a student inserted into or moved between
	// classes after this point is never silently transitioned without those
	// guards, and never promoted without a history row to revert (#405 review).
	promotions, graduates, err := s.lockTransitionCohorts(ctx, promoteClasses, graduateClasses)
	if err != nil {
		return err
	}

	// The admin confirmed a specific preview. Refuse if the locked cohort is no
	// longer the one that preview described — another admin editing mappings or
	// moving a child between classes in the meantime must not turn a reviewed
	// confirmation into a different, destructive outcome (#405 review).
	if err := ensureFingerprintMatches(expectedFingerprint, transition.Mappings, promotions, graduates); err != nil {
		return err
	}

	// A graduating child who is still checked in would be stranded: the row
	// flips to alumnus (open visit/attendance left dangling, still visible in
	// the active room), yet the kiosk rejects every alumnus checkout. Refuse
	// the whole apply until they are checked out.
	if err := s.ensureGraduatesNotCheckedIn(ctx, graduates); err != nil {
		return err
	}

	// Free the physical bracelets BEFORE the history write, so the ledger row
	// carries the tag that was actually released. A graduate keeps their person
	// row (graduation is a soft delete) and would otherwise keep holding the tag
	// while every staff-facing route — the kiosk's dedicated unassign included —
	// 404s on them, leaving a tag the kiosk can see but nobody can release
	// (#405 review).
	releasedTags, err := s.releaseGraduateTags(ctx, graduates)
	if err != nil {
		return err
	}

	if err := s.recordTransitionHistory(ctx, transition.ID, transition.Mappings, promotions, graduates, releasedTags); err != nil {
		return err
	}

	// Graduate BEFORE promoting. Promotions can move students into a class
	// that is graduated in the same transition (e.g. "3a -> 4a" alongside
	// "4a -> graduate"). Both cohorts were resolved from the same locked
	// snapshot, so a promoted-in child is not part of the graduate set; keeping
	// the original order also means the graduation writes never see a row the
	// promotion has already moved (#405 review).
	if err := s.applyGraduations(ctx, graduates, result); err != nil {
		return err
	}

	if err := s.applyPromotions(ctx, transition.Mappings, promotions, result); err != nil {
		return err
	}

	// Drop the departed children from already-materialized rosters so they stop
	// counting on today's and future timetables and staffing ratios, archiving
	// each removed row so the revert can replay it (#405 review).
	if err := s.reconcileGraduatedRosters(ctx, transition.ID, graduates); err != nil {
		return err
	}

	// Record which instances already existed, AFTER the archive pass and while
	// the locks still exclude every materializer. Everything inserted above this
	// marker is built during the alumnus window and is the revert's to refill.
	if err := s.recordRosterBaseline(ctx, transition); err != nil {
		return err
	}

	return s.markTransitionApplied(ctx, transition, accountID)
}

// lockTransitionCohorts resolves the promoting and graduating cohorts, takes a
// FOR UPDATE lock on each surviving row in ONE ascending-id pass (the
// deadlock-safe order every other student-row locker uses), and re-validates
// every row UNDER its lock.
//
// Re-validation is the point: between the snapshot query and the lock, a
// concurrent class edit or lifecycle change can commit. The locked row is
// therefore the authority for both the class that decides which cohort the child
// belongs to and the status recorded as from_status — a child moved out of every
// mapped class in that window is dropped from the transition entirely, and one
// already turned alumnus is skipped. Without this, apply could graduate a child
// who had just been moved out of the graduating class and record an obsolete
// from_status for a later revert to restore (#405 review).
//
// With no student repository wired (pure unit tests) the snapshot is used
// unlocked.
func (s *GradeTransitionService) lockTransitionCohorts(
	ctx context.Context,
	promoteClasses, graduateClasses []string,
) (promotions, graduates []*education.StudentClassInfo, err error) {
	classes := make([]string, 0, len(promoteClasses)+len(graduateClasses))
	classes = append(classes, promoteClasses...)
	classes = append(classes, graduateClasses...)
	if len(classes) == 0 {
		return nil, nil, nil
	}

	students, err := s.transitionRepo.GetStudentsByClasses(ctx, classes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get transition students: %w", err)
	}
	if len(students) == 0 {
		return nil, nil, nil
	}

	sort.Slice(students, func(i, j int) bool { return students[i].StudentID < students[j].StudentID })

	graduateSet := classSet(graduateClasses)
	promoteSet := classSet(promoteClasses)

	locked := make(map[int64]bool, len(students))

	// The kiosk/web check-in write path also takes a FOR UPDATE lock on the
	// student row (services/active.ensureStudentCheckinAllowed), so the two
	// transactions serialize on the shared row instead of racing: either a
	// concurrent check-in commits first and we observe its open record in the
	// check-in guard (and refuse), or we win the lock and the check-in re-reads
	// the alumnus status afterwards (and refuses).
	for _, student := range students {
		locked[student.StudentID] = true

		current, err := s.lockAndRefreshStudent(ctx, student)
		if err != nil {
			return nil, nil, err
		}
		if current == nil {
			continue // row vanished, or is already an alumnus — nothing to do
		}

		switch {
		case graduateSet[current.SchoolClass]:
			graduates = append(graduates, current)
		case promoteSet[current.SchoolClass]:
			promotions = append(promotions, current)
		default:
			// Moved out of every mapped class since the snapshot — this
			// transition has no claim on the child anymore.
		}
	}

	if err := s.ensureNoLateArrivals(ctx, classes, locked); err != nil {
		return nil, nil, err
	}

	return promotions, graduates, nil
}

// ensureNoLateArrivals refuses the apply when a child entered one of the mapped
// classes after the cohort snapshot was taken.
//
// The row locks above bound what a concurrent writer can CHANGE, but they cannot
// bound what it can ADD: a child created in a mapped class, moved in from an
// unmapped one, or reactivated by a revert lands in a row this pass never
// locked. Every write below then deliberately touches only the locked ids — so
// that late arrival is silently left behind in a class the transition has just
// emptied, while the transition is marked applied. That is a half-done bulk
// change presented as a finished one, and the revert has no history row to undo
// it with.
//
// Including them instead is not an option: they are exactly the children the
// admin's preview never showed, and graduating a child nobody confirmed is the
// destructive direction of this pair. So the apply refuses and the admin
// re-previews — the same answer, and the same 409, the fingerprint guard gives
// for every other cohort drift. Applying twice is safe (the second run sees the
// complete cohort), applying half of it is not (#405 review).
func (s *GradeTransitionService) ensureNoLateArrivals(
	ctx context.Context,
	classes []string,
	locked map[int64]bool,
) error {
	// Nothing was locked (no student repository wired — pure unit tests), so
	// there is no lock-window to re-check against.
	if s.studentRepo == nil {
		return nil
	}

	// Re-read UNDER the locks. In READ COMMITTED this statement sees every
	// concurrent commit that landed since the snapshot query — and the
	// class-writes gate the caller took before any of this (see
	// lockRecurrenceThenTransitions) means no further arrival can commit behind
	// it, so this re-read is the complete cohort for the rest of the apply.
	current, err := s.transitionRepo.GetStudentsByClasses(ctx, classes)
	if err != nil {
		return fmt.Errorf("failed to re-check transition students: %w", err)
	}
	for _, student := range current {
		if !locked[student.StudentID] {
			return ErrPreviewStale
		}
	}
	return nil
}

// lockAndRefreshStudent locks one student row and returns the cohort entry built
// from the LOCKED row's class and status. Returns (nil, nil) when the row
// vanished before it could be locked or is already an alumnus.
func (s *GradeTransitionService) lockAndRefreshStudent(
	ctx context.Context,
	snapshot *education.StudentClassInfo,
) (*education.StudentClassInfo, error) {
	if s.studentRepo == nil {
		return snapshot, nil
	}

	locked, err := s.studentRepo.FindByIDForUpdate(ctx, snapshot.StudentID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to lock transition student %d: %w", snapshot.StudentID, err)
	}
	if locked.Status == users.StudentStatusAlumnus {
		return nil, nil
	}

	// person_name stays from the snapshot: it is an audit-trail label, and the
	// locked row carries no person join.
	return &education.StudentClassInfo{
		StudentID:   snapshot.StudentID,
		PersonID:    locked.PersonID,
		PersonName:  snapshot.PersonName,
		SchoolClass: locked.SchoolClass,
		Status:      string(locked.Status),
	}, nil
}

// classSet builds a lookup set of class names.
func classSet(classes []string) map[string]bool {
	set := make(map[string]bool, len(classes))
	for _, c := range classes {
		set[c] = true
	}
	return set
}

// reconcileGraduatedRosters removes the graduated students from already-
// materialized rosters. No-op when no reconciler is wired.
func (s *GradeTransitionService) reconcileGraduatedRosters(
	ctx context.Context,
	transitionID int64,
	graduates []*education.StudentClassInfo,
) error {
	if s.rosterReconciler == nil || len(graduates) == 0 {
		return nil
	}
	if err := s.rosterReconciler.RemoveStudentsFromFutureRosters(ctx, transitionID, studentIDsOf(graduates)); err != nil {
		return fmt.Errorf("failed to reconcile graduated rosters: %w", err)
	}
	return nil
}

// recordRosterBaseline stamps the transition with the ordering marker its revert
// needs to separate instances that already existed from instances the
// materializer creates while the graduates are alumni. No-op when no reconciler
// is wired — the revert then skips the enrollment fill, matching an apply that
// archived nothing.
func (s *GradeTransitionService) recordRosterBaseline(
	ctx context.Context,
	transition *education.GradeTransition,
) error {
	if s.rosterReconciler == nil {
		return nil
	}
	baseline, err := s.rosterReconciler.CurrentRosterBaseline(ctx)
	if err != nil {
		return fmt.Errorf("failed to record roster baseline: %w", err)
	}
	transition.RosterBaselineInstanceID = &baseline
	return nil
}

// studentIDsOf projects the student IDs out of a StudentClassInfo slice.
func studentIDsOf(students []*education.StudentClassInfo) []int64 {
	ids := make([]int64, 0, len(students))
	for _, st := range students {
		ids = append(ids, st.StudentID)
	}
	return ids
}

// ensureGraduatesNotCheckedIn refuses the apply when any of the (already
// FOR UPDATE-locked) graduating students is currently checked in — an open visit
// (in a room) or an open attendance record for today. Such a child would become
// an alumnus with a dangling open record the kiosk can no longer close (it
// rejects every alumnus checkout). No-op when neither repository is wired (unit
// tests) or there is nothing to graduate. Because the caller already holds the
// FOR UPDATE lock on these rows, a concurrent check-in either committed before
// the lock (and is observed here) or blocks until this apply commits and then
// re-reads the alumnus status.
func (s *GradeTransitionService) ensureGraduatesNotCheckedIn(ctx context.Context, graduates []*education.StudentClassInfo) error {
	if len(graduates) == 0 || (s.visitRepo == nil && s.attendanceRepo == nil) {
		return nil
	}

	ids := studentIDsOf(graduates)

	checkedIn := make(map[int64]struct{})

	if s.visitRepo != nil {
		openVisits, err := s.visitRepo.GetCurrentByStudentIDs(ctx, ids)
		if err != nil {
			return fmt.Errorf("failed to check active visits: %w", err)
		}
		for id := range openVisits {
			checkedIn[id] = struct{}{}
		}
	}

	if s.attendanceRepo != nil {
		today, err := s.attendanceRepo.GetTodayByStudentIDs(ctx, ids)
		if err != nil {
			return fmt.Errorf("failed to check attendance: %w", err)
		}
		for id, att := range today {
			if att != nil && att.IsCheckedIn() {
				checkedIn[id] = struct{}{}
			}
		}
	}

	if len(checkedIn) > 0 {
		return fmt.Errorf("%w: %d student(s) must be checked out first", ErrGraduatesCheckedIn, len(checkedIn))
	}
	return nil
}

// categorizeMappings separates mappings into promote and graduate classes.
func categorizeMappings(mappings []*education.GradeTransitionMapping) (promote, graduate []string) {
	for _, mapping := range mappings {
		if mapping.IsGraduating() {
			graduate = append(graduate, mapping.FromClass)
		} else {
			promote = append(promote, mapping.FromClass)
		}
	}
	return
}

// recordTransitionHistory creates history records for all affected students.
// Both cohorts are the already FOR UPDATE-locked, re-validated sets threaded in
// from the caller, so the rows recorded in history are exactly the rows that get
// promoted and graduated — no student is transitioned without a history row to
// revert, and no history row describes a student the writes never touched (#405
// review).
func (s *GradeTransitionService) recordTransitionHistory(
	ctx context.Context,
	transitionID int64,
	mappings []*education.GradeTransitionMapping,
	promoted []*education.StudentClassInfo,
	graduates []*education.StudentClassInfo,
	releasedTags map[int64]string,
) error {
	// A student belongs to exactly one class, which is either a promote class or
	// a graduate class, so the two sets are disjoint — no dedup needed.
	students := make([]*education.StudentClassInfo, 0, len(promoted)+len(graduates))
	students = append(students, promoted...)
	students = append(students, graduates...)

	if len(students) == 0 {
		return nil
	}

	classMapping := buildClassMapping(mappings)
	historyRecords := buildHistoryRecords(transitionID, students, classMapping, releasedTags)

	tenantID := tenant.FromContext(ctx)
	for _, h := range historyRecords {
		h.SetTenantID(tenantID)
	}

	if err := s.transitionRepo.CreateHistoryBatch(ctx, historyRecords); err != nil {
		return fmt.Errorf("failed to create history: %w", err)
	}
	return nil
}

// buildClassMapping creates a map of from_class -> to_class.
func buildClassMapping(mappings []*education.GradeTransitionMapping) map[string]*string {
	classMapping := make(map[string]*string)
	for _, mapping := range mappings {
		classMapping[mapping.FromClass] = mapping.ToClass
	}
	return classMapping
}

// buildHistoryRecords creates history records from students and class mapping.
// releasedTags carries the RFID tag graduation took off each student (keyed by
// student id); a student absent from it held none.
func buildHistoryRecords(
	transitionID int64,
	students []*education.StudentClassInfo,
	classMapping map[string]*string,
	releasedTags map[int64]string,
) []*education.GradeTransitionHistory {
	records := make([]*education.GradeTransitionHistory, 0, len(students))
	for _, student := range students {
		toClass := classMapping[student.SchoolClass]
		action := education.ActionPromoted
		if toClass == nil {
			action = education.ActionGraduated
		}
		// Snapshot the pre-transition status so a revert restores graduates to
		// exactly what they were (pending / inactive) instead of activating them.
		fromStatus := student.Status
		record := &education.GradeTransitionHistory{
			TransitionID: transitionID,
			StudentID:    student.StudentID,
			PersonName:   student.PersonName,
			FromClass:    student.SchoolClass,
			ToClass:      toClass,
			Action:       action,
			FromStatus:   &fromStatus,
		}
		if tag, ok := releasedTags[student.StudentID]; ok && tag != "" {
			released := tag
			record.RFIDTag = &released
		}
		records = append(records, record)
	}
	return records
}

// applyPromotions moves exactly the FOR UPDATE-locked, history-recorded
// promotion cohort into its mapped class — never a class-wide update.
//
// A class-wide UPDATE re-evaluates membership at write time, so a child created
// or moved into a promoted class between the history snapshot and the write was
// promoted with no history row and could never be reverted, while the inverse
// race recorded history for a child the update no longer matched. Updating the
// locked ID set makes the recorded and mutated rows identical (#405 review).
//
// One guarded UPDATE per from-class keeps the mapping explicit and lets the
// repository re-assert the from-class in the WHERE.
func (s *GradeTransitionService) applyPromotions(
	ctx context.Context,
	mappings []*education.GradeTransitionMapping,
	promotions []*education.StudentClassInfo,
	result *TransitionResult,
) error {
	if len(promotions) == 0 {
		return nil
	}

	classMapping := buildClassMapping(mappings)

	byFromClass := make(map[string][]int64)
	for _, student := range promotions {
		toClass := classMapping[student.SchoolClass]
		if toClass == nil || *toClass == "" {
			// Defensive: a promotion cohort member always has a destination.
			continue
		}
		byFromClass[student.SchoolClass] = append(byFromClass[student.SchoolClass], student.StudentID)
	}

	// Deterministic statement order so concurrent applies of different
	// transitions cannot interleave their class updates in opposite orders.
	fromClasses := make([]string, 0, len(byFromClass))
	for fromClass := range byFromClass {
		fromClasses = append(fromClasses, fromClass)
	}
	sort.Strings(fromClasses)

	var promoted int64
	for _, fromClass := range fromClasses {
		n, err := s.transitionRepo.PromoteStudentsByIDs(
			ctx, byFromClass[fromClass], fromClass, *classMapping[fromClass],
		)
		if err != nil {
			return fmt.Errorf("failed to promote students: %w", err)
		}
		promoted += n
	}

	result.StudentsPromoted = int(promoted)
	return nil
}

// applyGraduations soft-deletes the graduating students by marking them alumnus.
// It mutates exactly the FOR UPDATE-locked rows the caller checked and recorded
// — never a blind class-wide update — so a student concurrently inserted into a
// graduating class cannot be graduated without the check-in guard and history
// (#405 review). Rows survive so a revert can restore them.
func (s *GradeTransitionService) applyGraduations(
	ctx context.Context,
	graduates []*education.StudentClassInfo,
	result *TransitionResult,
) error {
	if len(graduates) == 0 {
		return nil
	}

	graduated, err := s.transitionRepo.GraduateStudentsByIDs(ctx, studentIDsOf(graduates))
	if err != nil {
		return fmt.Errorf("failed to graduate students: %w", err)
	}
	result.StudentsGraduated = int(graduated)
	return nil
}

// releaseGraduateTags clears the RFID tag of every graduating student and
// returns what each was holding, so the history write can ledger it.
//
// The bracelet is physical: it is handed back when a child leaves and reissued
// to a current one. Leaving it bound to the alumnus creates a state the kiosk
// can observe but not resolve — GET /api/iot/rfid/{tagId} resolves the person
// unfiltered and shows the departed child's name, while DELETE
// /api/students/{id}/rfid goes through the shared alumnus gate and always 404s
// (#405 review).
//
// Runs inside the apply transaction on the already FOR UPDATE-locked cohort, so
// a failure here rolls the whole graduation back rather than half-releasing
// tags.
func (s *GradeTransitionService) releaseGraduateTags(
	ctx context.Context,
	graduates []*education.StudentClassInfo,
) (map[int64]string, error) {
	if len(graduates) == 0 {
		return nil, nil
	}

	released, err := s.transitionRepo.ReleaseStudentTagsByIDs(ctx, studentIDsOf(graduates))
	if err != nil {
		return nil, fmt.Errorf("failed to release graduate RFID tags: %w", err)
	}
	return released, nil
}

// markTransitionApplied updates the transition status to applied.
func (s *GradeTransitionService) markTransitionApplied(
	ctx context.Context,
	transition *education.GradeTransition,
	accountID int64,
) error {
	now := time.Now()
	transition.Status = education.TransitionStatusApplied
	transition.AppliedAt = &now
	transition.AppliedBy = &accountID

	if err := s.transitionRepo.Update(ctx, transition); err != nil {
		return fmt.Errorf("failed to update transition status: %w", err)
	}
	return nil
}

// finalizeApplyResult sets final result fields after successful apply.
func finalizeApplyResult(result *TransitionResult) {
	result.Status = education.TransitionStatusApplied
	result.CanRevert = true

	if result.StudentsGraduated > 0 {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("%d students were marked as alumni and hidden from the app (graduates)",
				result.StudentsGraduated))
	}
}

// Revert undoes an applied grade transition
func (s *GradeTransitionService) Revert(ctx context.Context, id int64, accountID int64) (*TransitionResult, error) {
	// Same tenant-wide lock Apply takes: apply and revert share this resource so
	// a revert of the current latest transition cannot interleave with an apply of
	// a newer draft (#405 review). It equally excludes a concurrent
	// materialization pass, which takes the same gate — otherwise the
	// materializer could insert rows between the reactivation and the roster
	// reconciliation below. Both locks release at COMMIT/ROLLBACK.
	if err := s.lockRecurrenceThenTransitions(ctx); err != nil {
		return nil, err
	}

	transition, err := s.transitionRepo.FindByIDWithMappings(ctx, id)
	if err != nil {
		return nil, fmt.Errorf(errFmtTransitionNotFound, err)
	}

	if err := validateCanRevert(transition); err != nil {
		return nil, err
	}

	// Enforce reverse-order reverts on the server, not just in the UI. Lock the
	// latest applied transition FOR UPDATE and refuse if it is not the one being
	// reverted: a stale admin page (or a direct call to the authorized endpoint)
	// could otherwise revert an older transition after a newer one has been
	// applied, replaying its history over the newer classes/statuses. The lock
	// also serializes two concurrent reverts of the same latest row (#405).
	latest, err := s.transitionRepo.LockLatestApplied(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve latest applied transition: %w", err)
	}
	if latest == nil || latest.ID != id {
		return nil, ErrNotLatestApplied
	}

	history, err := s.transitionRepo.GetHistory(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get transition history: %w", err)
	}

	result := &TransitionResult{
		TransitionID: id,
		Warnings:     make([]string, 0),
	}

	if err := s.executeRevert(ctx, transition, accountID, history, result); err != nil {
		return nil, err
	}

	result.Status = education.TransitionStatusReverted
	result.CanRevert = false

	return result, nil
}

// validateCanRevert checks if a transition can be reverted. Both refusals wrap
// ErrTransitionNotApplied — a draft was never applied, and an already-reverted
// transition has nothing left to undo. Either way the caller's list was stale
// and the handler answers 409 so the UI reloads it (#405 review).
func validateCanRevert(transition *education.GradeTransition) error {
	if transition.CanRevert() {
		return nil
	}
	if transition.IsDraft() {
		return fmt.Errorf("%w: transition has not been applied yet", ErrTransitionNotApplied)
	}
	return fmt.Errorf("%w: transition has already been reverted", ErrTransitionNotApplied)
}

// executeRevert performs the actual revert within a transaction.
func (s *GradeTransitionService) executeRevert(
	ctx context.Context,
	transition *education.GradeTransition,
	accountID int64,
	history []*education.GradeTransitionHistory,
	result *TransitionResult,
) error {
	graduated, err := s.revertPromotedStudents(ctx, history, result)
	if err != nil {
		return err
	}

	reactivated, err := s.revertGraduatedStudents(ctx, graduated, result)
	if err != nil {
		return err
	}

	// Re-add the restored children to already-materialized rosters they were
	// dropped from (replayed from the archive, with today's day statuses) or
	// never added to (materialized while alumni) (#405 review). Only the children
	// this revert restored AS ACTIVE — a graduate whose status was changed by
	// hand since the apply is deliberately left as-is, and one restored to the
	// pending or inactive state it held before the transition belongs off
	// actionable future rosters just like any other non-active child.
	if err := s.reconcileRestoredRosters(ctx, transition, reactivated); err != nil {
		return err
	}

	return s.markTransitionReverted(ctx, transition, accountID)
}

// reconcileRestoredRosters re-adds the reactivated students to the rosters this
// transition took them off, and fills the instances materialized after the apply
// from enrollments. No-op when no reconciler is wired.
//
// `ids` are the students this revert restored as active — never the whole
// graduated history. A child who is no longer an alumnus for another reason
// (manually set inactive, say) is skipped by the reactivation, and one restored
// to its recorded pending/inactive pre-transition status stays off the rosters
// for the same reason: neither is an active child (#405 review).
func (s *GradeTransitionService) reconcileRestoredRosters(
	ctx context.Context,
	transition *education.GradeTransition,
	ids []int64,
) error {
	if s.rosterReconciler == nil || len(ids) == 0 {
		return nil
	}

	// Everything inserted above the apply's marker is what the materializer built
	// while these children were alumni — only those instances may be filled from
	// enrollments; older ones are owned by the archive replay. The marker is an
	// instance id, not a timestamp: created_at carries the transaction START
	// time, so a materialization that waited on the transition lock backdates
	// rows it inserted after the apply and would be misfiled as pre-transition
	// (#405 review). A transition applied before the marker existed carries NULL
	// and fills nothing rather than reconstructing over hand-edited rosters.
	if err := s.rosterReconciler.RestoreStudentsToFutureRosters(
		ctx, transition.ID, ids, transition.RosterBaselineInstanceID,
	); err != nil {
		return fmt.Errorf("failed to reconcile restored rosters: %w", err)
	}
	return nil
}

// revertPromotedStudents reverts promoted students to their original classes.
// Returns the graduated students' history records so they can be restored to
// their recorded pre-transition status, and any error.
func (s *GradeTransitionService) revertPromotedStudents(
	ctx context.Context,
	history []*education.GradeTransitionHistory,
	result *TransitionResult,
) ([]*education.GradeTransitionHistory, error) {
	graduated := make([]*education.GradeTransitionHistory, 0)
	missingStudentCount := 0

	for _, h := range history {
		switch {
		case h.WasPromoted():
			reverted, err := s.revertStudentClass(ctx, h)
			if err != nil {
				// Real database error - propagate to trigger transaction rollback
				return nil, fmt.Errorf("failed to revert student %d: %w", h.StudentID, err)
			}
			if reverted {
				result.StudentsPromoted++
			} else {
				// Student was deleted, or moved to a different class since
				// promotion (manual edit or a later transition) - track for warning
				missingStudentCount++
			}
		case h.WasGraduated():
			graduated = append(graduated, h)
		}
	}

	if missingStudentCount > 0 {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("%d promoted students could not be reverted (deleted or class changed since promotion)",
				missingStudentCount))
	}

	return graduated, nil
}

// revertGraduatedStudents restores graduated (alumnus) students to the
// lifecycle status they held before the transition — an active child returns to
// active, a pending future enrollment returns to pending, an inactive leaver
// returns to inactive. History rows written before from_status existed fall
// back to active. Graduation is a soft delete, so the revert is complete — only
// students whose rows were deleted or manually changed since cannot be restored
// and are reported as a warning.
//
// Returns the ids restored AS ACTIVE, and only those: the caller feeds them to
// the roster reconciler, and a child restored to pending or inactive must not
// reappear on actionable future rosters — the roster removal is exactly what
// those lifecycle states mean (#405 review).
func (s *GradeTransitionService) revertGraduatedStudents(
	ctx context.Context,
	graduated []*education.GradeTransitionHistory,
	result *TransitionResult,
) ([]int64, error) {
	if len(graduated) == 0 {
		return nil, nil
	}

	// Group by the status to restore so each distinct status is one UPDATE.
	byStatus := make(map[string][]int64)
	for _, h := range graduated {
		status := string(users.StudentStatusActive)
		if h.FromStatus != nil && *h.FromStatus != "" {
			status = *h.FromStatus
		}
		byStatus[status] = append(byStatus[status], h.StudentID)
	}

	restored := make([]int64, 0, len(graduated))
	restoredActive := make([]int64, 0, len(graduated))
	for status, ids := range byStatus {
		reactivated, err := s.transitionRepo.ReactivateStudentsToStatus(ctx, ids, status)
		if err != nil {
			return nil, fmt.Errorf("failed to reactivate graduated students: %w", err)
		}
		restored = append(restored, reactivated...)
		if status == string(users.StudentStatusActive) {
			restoredActive = append(restoredActive, reactivated...)
		}
	}
	result.StudentsGraduated = len(restored)

	if notRestored := len(graduated) - len(restored); notRestored > 0 {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("%d graduated students could not be restored (deleted or status changed since graduation)",
				notRestored))
	}

	// Hand the bracelets back to exactly the children this revert reactivated —
	// including the pending/inactive ones, whose tag link the apply released too.
	if err := s.restoreGraduateTags(ctx, graduated, restored, result); err != nil {
		return nil, err
	}
	return restoredActive, nil
}

// restoreGraduateTags re-links the RFID tags the apply released, for the
// students this revert actually reactivated. A tag that has since been issued to
// another child is left where it is and reported as a warning: the bracelet is a
// physical object, the current holder is the one wearing it, and
// users.persons.tag_id is unique per tenant anyway (#405 review).
func (s *GradeTransitionService) restoreGraduateTags(
	ctx context.Context,
	graduated []*education.GradeTransitionHistory,
	restored []int64,
	result *TransitionResult,
) error {
	restoredSet := make(map[int64]struct{}, len(restored))
	for _, id := range restored {
		restoredSet[id] = struct{}{}
	}

	reissued := 0
	for _, h := range graduated {
		if h.RFIDTag == nil || *h.RFIDTag == "" {
			continue
		}
		if _, ok := restoredSet[h.StudentID]; !ok {
			continue
		}
		relinked, err := s.transitionRepo.RestoreStudentTag(ctx, h.StudentID, *h.RFIDTag)
		if err != nil {
			return fmt.Errorf("failed to restore RFID tag for student %d: %w", h.StudentID, err)
		}
		if !relinked {
			reissued++
		}
	}

	if reissued > 0 {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("%d RFID tags could not be re-linked (reassigned to another child since graduation)",
				reissued))
	}
	return nil
}

// revertStudentClass moves a promoted student back to their original class,
// but only while their current class still equals the class this transition
// assigned (h.ToClass). Returns (true, nil) if reverted, (false, nil) if the
// student no longer exists OR was moved to a different class since promotion
// (a manual correction or a later transition — either must be preserved), or
// (false, error) on a real database error.
func (s *GradeTransitionService) revertStudentClass(
	ctx context.Context,
	h *education.GradeTransitionHistory,
) (bool, error) {
	// A promoted history row always carries the destination class; guard against
	// a malformed row rather than reverting to an empty-string guard value.
	if h.ToClass == nil {
		return false, nil
	}

	// The repository joins the transition transaction via the context. The
	// to_class equality guard lives in the UPDATE's WHERE, so a since-moved child
	// yields 0 rows affected and is left untouched.
	rowsAffected, err := s.transitionRepo.RevertStudentClass(ctx, h.StudentID, h.FromClass, *h.ToClass)
	if err != nil {
		// Real database error - return it to trigger rollback
		return false, err
	}

	// No rows affected means the student was deleted or moved to another class
	// since promotion - not an error, just a skip the caller reports as a warning.
	return rowsAffected > 0, nil
}

// markTransitionReverted updates the transition status to reverted.
func (s *GradeTransitionService) markTransitionReverted(
	ctx context.Context,
	transition *education.GradeTransition,
	accountID int64,
) error {
	now := time.Now()
	transition.Status = education.TransitionStatusReverted
	transition.RevertedAt = &now
	transition.RevertedBy = &accountID

	if err := s.transitionRepo.Update(ctx, transition); err != nil {
		return fmt.Errorf("failed to update transition status: %w", err)
	}
	return nil
}

// GetDistinctClasses returns all distinct school classes
func (s *GradeTransitionService) GetDistinctClasses(ctx context.Context) ([]string, error) {
	return s.transitionRepo.GetDistinctClasses(ctx)
}

// SuggestMappings auto-suggests class mappings based on naming patterns
func (s *GradeTransitionService) SuggestMappings(ctx context.Context) ([]*SuggestedMapping, error) {
	classes, err := s.transitionRepo.GetDistinctClasses(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get classes: %w", err)
	}

	suggestions := make([]*SuggestedMapping, 0)

	// Pattern: optional prefix, grade number, optional letter(s) — matches
	// "1a", "10c", digit-only "1", and prefixed names like "Klasse 1a".
	// The prefix is preserved when incrementing.
	classPattern := regexp.MustCompile(`^(.*?)(\d+)([a-zA-Z]*)$`)

	for _, className := range classes {
		count, err := s.transitionRepo.GetStudentCountByClass(ctx, className)
		if err != nil || count == 0 {
			continue
		}

		matches := classPattern.FindStringSubmatch(className)
		if matches == nil {
			// No grade pattern — we cannot confidently tell "leaves the OGS"
			// from a free-form or placeholder class (e.g. "offen"). Mark it
			// ambiguous so the editor requires an explicit choice rather than
			// silently preselecting Abgang.
			suggestions = append(suggestions, &SuggestedMapping{
				FromClass:    className,
				ToClass:      nil,
				StudentCount: count,
				IsGraduating: true,
				Ambiguous:    true,
			})
			continue
		}

		// Parse grade number
		gradeNum := 0
		if _, err := fmt.Sscanf(matches[2], "%d", &gradeNum); err != nil {
			continue // Skip if parsing fails
		}
		prefix := matches[1]
		letterPart := matches[3]

		// Increment grade number, keeping prefix and letters
		nextGrade := gradeNum + 1
		nextClass := fmt.Sprintf("%s%d%s", prefix, nextGrade, letterPart)

		// Check if this is likely the highest grade (e.g., 4a for elementary)
		// For simplicity, assume grades 4+ might be graduating
		if gradeNum >= 4 {
			// Could be graduating - mark as such
			suggestions = append(suggestions, &SuggestedMapping{
				FromClass:    className,
				ToClass:      nil,
				StudentCount: count,
				IsGraduating: true,
			})
		} else {
			// Normal promotion
			suggestions = append(suggestions, &SuggestedMapping{
				FromClass:    className,
				ToClass:      &nextClass,
				StudentCount: count,
				IsGraduating: false,
			})
		}
	}

	// Sort by class name
	sort.Slice(suggestions, func(i, j int) bool {
		return suggestions[i].FromClass < suggestions[j].FromClass
	})

	return suggestions, nil
}

// GetHistory retrieves the history records for a transition
func (s *GradeTransitionService) GetHistory(ctx context.Context, transitionID int64) ([]*education.GradeTransitionHistory, error) {
	return s.transitionRepo.GetHistory(ctx, transitionID)
}

// Graduate lifecycle states reported alongside a transition's ledger. They
// describe what is left of a graduated child TODAY, which is what decides
// which actions the Abgänge view may offer.
const (
	// GraduateStateAlumnus: still soft-deleted. Revertable, and the only state
	// in which an endgültige Löschung is possible.
	GraduateStateAlumnus = "alumnus"
	// GraduateStateRestored: back in the roster — either this transition was
	// reverted, or the status was changed by hand since.
	GraduateStateRestored = "restored"
	// GraduateStatePurged: hard-deleted. Nothing left to restore or delete; the
	// ledger row survives only as the count of what this transition did.
	GraduateStatePurged = "purged"
)

// GetHistoryWithStudentStates returns a transition's ledger together with the
// CURRENT state of every student it names.
//
// The ledger alone cannot drive the Abgänge view: it is an append-only record
// of what the apply did, so a child who has since been reverted or hard-deleted
// still reads as "graduated" in it. The state map is resolved fresh on every
// call and is keyed by student id.
func (s *GradeTransitionService) GetHistoryWithStudentStates(
	ctx context.Context,
	transitionID int64,
) ([]*education.GradeTransitionHistory, map[int64]string, error) {
	// The ledger query alone cannot tell "transition without history" (a draft,
	// legitimately empty) apart from "transition does not exist" — both come
	// back as zero rows. Resolve the transition first so a nonexistent id is a
	// not-found error the handler turns into 404, not a 200 with an empty list
	// (#405 review).
	if _, err := s.transitionRepo.FindByID(ctx, transitionID); err != nil {
		return nil, nil, fmt.Errorf(errFmtTransitionNotFound, err)
	}

	history, err := s.transitionRepo.GetHistory(ctx, transitionID)
	if err != nil {
		return nil, nil, err
	}
	if len(history) == 0 {
		return history, map[int64]string{}, nil
	}

	ids := make([]int64, 0, len(history))
	seen := make(map[int64]struct{}, len(history))
	for _, h := range history {
		if _, dup := seen[h.StudentID]; dup {
			continue
		}
		seen[h.StudentID] = struct{}{}
		ids = append(ids, h.StudentID)
	}

	statuses, err := s.transitionRepo.FindStudentStatesByIDs(ctx, ids)
	if err != nil {
		return nil, nil, err
	}

	states := make(map[int64]string, len(ids))
	for _, id := range ids {
		status, exists := statuses[id]
		switch {
		case !exists:
			states[id] = GraduateStatePurged
		case status == string(users.StudentStatusAlumnus):
			states[id] = GraduateStateAlumnus
		default:
			states[id] = GraduateStateRestored
		}
	}
	return history, states, nil
}

// ErrGraduateStillPresent is returned when the ledger anonymization is asked to
// run for a student whose row still exists.
var ErrGraduateStillPresent = errors.New("student row still exists, refusing to anonymize its ledger")

// AnonymizePurgedGraduate strips the child's name from the transition ledger
// after the student and person rows have been hard-deleted.
//
// It verifies the row is really gone first, because that is the whole
// justification for losing the name: while the student still exists the ledger
// name is the only human-readable label a revert or an Abgänge list has, and
// blanking it there would break both for a child who is still restorable.
// Callers run this INSIDE the delete transaction, so the check sees the delete.
func (s *GradeTransitionService) AnonymizePurgedGraduate(ctx context.Context, studentID int64) error {
	statuses, err := s.transitionRepo.FindStudentStatesByIDs(ctx, []int64{studentID})
	if err != nil {
		return err
	}
	if _, stillThere := statuses[studentID]; stillThere {
		return ErrGraduateStillPresent
	}
	return s.transitionRepo.AnonymizeHistoryForStudent(ctx, studentID)
}
