package education

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"time"

	"github.com/moto-nrw/project-phoenix/models/base"
	"github.com/moto-nrw/project-phoenix/models/education"
	"github.com/moto-nrw/project-phoenix/models/users"
	"github.com/uptrace/bun"
)

// GradeTransitionService defines operations for managing grade transitions
type GradeTransitionService interface {
	// Transition management
	Create(ctx context.Context, req CreateTransitionRequest) (*education.GradeTransition, error)
	Update(ctx context.Context, id int64, req UpdateTransitionRequest) (*education.GradeTransition, error)
	Delete(ctx context.Context, id int64) error
	GetByID(ctx context.Context, id int64) (*education.GradeTransition, error)
	List(ctx context.Context, options *base.QueryOptions) ([]*education.GradeTransition, int, error)

	// Preview & Apply
	Preview(ctx context.Context, id int64) (*TransitionPreview, error)
	Apply(ctx context.Context, id int64, accountID int64) (*TransitionResult, error)

	// Undo
	Revert(ctx context.Context, id int64, accountID int64) (*TransitionResult, error)

	// Utilities
	GetDistinctClasses(ctx context.Context) ([]string, error)
	SuggestMappings(ctx context.Context) ([]*SuggestedMapping, error)
	GetHistory(ctx context.Context, transitionID int64) ([]*education.GradeTransitionHistory, error)
}

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
	TransitionID    int64               `json:"transition_id"`
	AcademicYear    string              `json:"academic_year"`
	TotalStudents   int                 `json:"total_students"`
	ToPromote       int                 `json:"to_promote"`
	ToGraduate      int                 `json:"to_graduate"`
	ByMapping       []MappingPreview    `json:"by_mapping"`
	UnmappedClasses []UnmappedClassInfo `json:"unmapped_classes"`
	Warnings        []string            `json:"warnings"`
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
	TransitionID      int64    `json:"transition_id"`
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
}

// Error message format constants
const errFmtTransitionNotFound = "transition not found: %w"

// gradeTransitionService implements GradeTransitionService
type gradeTransitionService struct {
	transitionRepo education.GradeTransitionRepository
	studentRepo    users.StudentRepository
	personRepo     users.PersonRepository
	db             *bun.DB
	txHandler      *base.TxHandler
}

// GradeTransitionServiceDependencies contains dependencies for the service
type GradeTransitionServiceDependencies struct {
	TransitionRepo education.GradeTransitionRepository
	StudentRepo    users.StudentRepository
	PersonRepo     users.PersonRepository
	DB             *bun.DB
}

// NewGradeTransitionService creates a new grade transition service
func NewGradeTransitionService(deps GradeTransitionServiceDependencies) GradeTransitionService {
	return &gradeTransitionService{
		transitionRepo: deps.TransitionRepo,
		studentRepo:    deps.StudentRepo,
		personRepo:     deps.PersonRepo,
		db:             deps.DB,
		txHandler:      base.NewTxHandler(deps.DB),
	}
}

// Create creates a new grade transition with mappings
func (s *gradeTransitionService) Create(ctx context.Context, req CreateTransitionRequest) (*education.GradeTransition, error) {
	if req.AcademicYear == "" {
		return nil, errors.New("academic_year is required")
	}

	transition := &education.GradeTransition{
		AcademicYear: req.AcademicYear,
		Status:       education.TransitionStatusDraft,
		CreatedBy:    req.CreatedBy,
		Notes:        req.Notes,
	}

	if err := transition.Validate(); err != nil {
		return nil, err
	}

	// Execute in transaction
	err := s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		if err := s.transitionRepo.Create(ctx, transition); err != nil {
			return fmt.Errorf("failed to create transition: %w", err)
		}
		return s.createMappingsIfProvided(ctx, transition, req.Mappings)
	})

	if err != nil {
		return nil, err
	}

	return transition, nil
}

// createMappingsIfProvided creates mappings for a transition if any are provided.
func (s *gradeTransitionService) createMappingsIfProvided(
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

	if err := s.transitionRepo.CreateMappings(ctx, mappings); err != nil {
		return fmt.Errorf("failed to create mappings: %w", err)
	}
	transition.Mappings = mappings
	return nil
}

// buildMappings validates and builds mapping entities from input.
func (s *gradeTransitionService) buildMappings(transitionID int64, inputs []MappingRequest) ([]*education.GradeTransitionMapping, error) {
	mappings := make([]*education.GradeTransitionMapping, 0, len(inputs))
	for _, m := range inputs {
		mapping := &education.GradeTransitionMapping{
			TransitionID: transitionID,
			FromClass:    m.FromClass,
			ToClass:      m.ToClass,
		}
		if err := mapping.Validate(); err != nil {
			return nil, fmt.Errorf("invalid mapping for class %s: %w", m.FromClass, err)
		}
		mappings = append(mappings, mapping)
	}
	return mappings, nil
}

// Update updates a grade transition
func (s *gradeTransitionService) Update(ctx context.Context, id int64, req UpdateTransitionRequest) (*education.GradeTransition, error) {
	transition, err := s.transitionRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf(errFmtTransitionNotFound, err)
	}

	if !transition.CanModify() {
		return nil, errors.New("cannot modify transition: must be in draft status")
	}

	applyTransitionUpdates(transition, req)

	if err := transition.Validate(); err != nil {
		return nil, err
	}

	err = s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		if err := s.transitionRepo.Update(ctx, transition); err != nil {
			return fmt.Errorf("failed to update transition: %w", err)
		}
		return s.replaceMappingsIfProvided(ctx, transition, req.Mappings)
	})

	if err != nil {
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
func (s *gradeTransitionService) replaceMappingsIfProvided(
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

	if err := s.transitionRepo.CreateMappings(ctx, mappings); err != nil {
		return fmt.Errorf("failed to create mappings: %w", err)
	}
	transition.Mappings = mappings
	return nil
}

// Delete deletes a draft grade transition
func (s *gradeTransitionService) Delete(ctx context.Context, id int64) error {
	transition, err := s.transitionRepo.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf(errFmtTransitionNotFound, err)
	}

	if !transition.CanModify() {
		return errors.New("cannot delete transition: must be in draft status")
	}

	return s.transitionRepo.Delete(ctx, id)
}

// GetByID retrieves a grade transition by ID with mappings
func (s *gradeTransitionService) GetByID(ctx context.Context, id int64) (*education.GradeTransition, error) {
	return s.transitionRepo.FindByIDWithMappings(ctx, id)
}

// List retrieves grade transitions with pagination
func (s *gradeTransitionService) List(ctx context.Context, options *base.QueryOptions) ([]*education.GradeTransition, int, error) {
	return s.transitionRepo.List(ctx, options)
}

// Preview returns what will happen when the transition is applied
func (s *gradeTransitionService) Preview(ctx context.Context, id int64) (*TransitionPreview, error) {
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

	mappedClasses, err := s.buildMappingPreviews(ctx, preview, transition.Mappings)
	if err != nil {
		return nil, err
	}

	if err := s.findUnmappedClasses(ctx, preview, mappedClasses); err != nil {
		return nil, err
	}

	s.addPreviewWarnings(preview)

	return preview, nil
}

// buildMappingPreviews populates preview with mapping details and returns mapped class names.
func (s *gradeTransitionService) buildMappingPreviews(
	ctx context.Context,
	preview *TransitionPreview,
	mappings []*education.GradeTransitionMapping,
) (map[string]bool, error) {
	mappedClasses := make(map[string]bool)

	for _, mapping := range mappings {
		count, err := s.transitionRepo.GetStudentCountByClass(ctx, mapping.FromClass)
		if err != nil {
			return nil, fmt.Errorf("failed to count students in class %s: %w", mapping.FromClass, err)
		}

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

	return mappedClasses, nil
}

// createMappingPreview creates a MappingPreview from a mapping and student count.
func (s *gradeTransitionService) createMappingPreview(mapping *education.GradeTransitionMapping, count int) MappingPreview {
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
func (s *gradeTransitionService) findUnmappedClasses(
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
func (s *gradeTransitionService) addPreviewWarnings(preview *TransitionPreview) {
	if len(preview.UnmappedClasses) > 0 {
		preview.Warnings = append(preview.Warnings,
			fmt.Sprintf("%d classes with students are not included in this transition",
				len(preview.UnmappedClasses)))
	}
	if preview.ToGraduate > 0 {
		preview.Warnings = append(preview.Warnings,
			fmt.Sprintf("%d students will be permanently deleted (graduates)",
				preview.ToGraduate))
	}
}

// Apply executes the grade transition
func (s *gradeTransitionService) Apply(ctx context.Context, id int64, accountID int64) (*TransitionResult, error) {
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

	err = s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		return s.executeApply(ctx, transition, accountID, result)
	})

	if err != nil {
		return nil, err
	}

	finalizeApplyResult(result)
	return result, nil
}

// validateCanApply checks if a transition can be applied.
func validateCanApply(transition *education.GradeTransition) error {
	if transition.CanApply() {
		return nil
	}
	if transition.IsApplied() {
		return errors.New("transition has already been applied")
	}
	if transition.IsReverted() {
		return errors.New("transition has been reverted")
	}
	return errors.New("cannot apply transition: must be in draft status with mappings")
}

// executeApply performs the actual apply within a transaction.
func (s *gradeTransitionService) executeApply(
	ctx context.Context,
	transition *education.GradeTransition,
	accountID int64,
	result *TransitionResult,
) error {
	promoteClasses, graduateClasses := categorizeMappings(transition.Mappings)
	allClasses := append(promoteClasses, graduateClasses...)

	if err := s.recordTransitionHistory(ctx, transition.ID, transition.Mappings, allClasses); err != nil {
		return err
	}

	if err := s.applyPromotions(ctx, transition.ID, promoteClasses, result); err != nil {
		return err
	}

	if err := s.applyGraduations(ctx, graduateClasses, result); err != nil {
		return err
	}

	return s.markTransitionApplied(ctx, transition, accountID)
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
func (s *gradeTransitionService) recordTransitionHistory(
	ctx context.Context,
	transitionID int64,
	mappings []*education.GradeTransitionMapping,
	allClasses []string,
) error {
	students, err := s.transitionRepo.GetStudentsByClasses(ctx, allClasses)
	if err != nil {
		return fmt.Errorf("failed to get students: %w", err)
	}

	if len(students) == 0 {
		return nil
	}

	classMapping := buildClassMapping(mappings)
	historyRecords := buildHistoryRecords(transitionID, students, classMapping)

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
func buildHistoryRecords(
	transitionID int64,
	students []*education.StudentClassInfo,
	classMapping map[string]*string,
) []*education.GradeTransitionHistory {
	records := make([]*education.GradeTransitionHistory, 0, len(students))
	for _, student := range students {
		toClass := classMapping[student.SchoolClass]
		action := education.ActionPromoted
		if toClass == nil {
			action = education.ActionGraduated
		}
		records = append(records, &education.GradeTransitionHistory{
			TransitionID: transitionID,
			StudentID:    student.StudentID,
			PersonName:   student.PersonName,
			FromClass:    student.SchoolClass,
			ToClass:      toClass,
			Action:       action,
		})
	}
	return records
}

// applyPromotions updates student classes for promotions.
func (s *gradeTransitionService) applyPromotions(
	ctx context.Context,
	transitionID int64,
	promoteClasses []string,
	result *TransitionResult,
) error {
	if len(promoteClasses) == 0 {
		return nil
	}
	promoted, err := s.transitionRepo.UpdateStudentClasses(ctx, transitionID)
	if err != nil {
		return fmt.Errorf("failed to promote students: %w", err)
	}
	result.StudentsPromoted = int(promoted)
	return nil
}

// applyGraduations deletes graduating students.
func (s *gradeTransitionService) applyGraduations(
	ctx context.Context,
	graduateClasses []string,
	result *TransitionResult,
) error {
	if len(graduateClasses) == 0 {
		return nil
	}

	for _, className := range graduateClasses {
		count, err := s.transitionRepo.GetStudentCountByClass(ctx, className)
		if err != nil {
			return fmt.Errorf("failed to count graduating students in class %s: %w", className, err)
		}
		result.StudentsGraduated += count
	}

	if _, err := s.transitionRepo.DeleteStudentsByClasses(ctx, graduateClasses); err != nil {
		return fmt.Errorf("failed to delete graduating students: %w", err)
	}
	return nil
}

// markTransitionApplied updates the transition status to applied.
func (s *gradeTransitionService) markTransitionApplied(
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
			fmt.Sprintf("%d students were permanently deleted (graduates)",
				result.StudentsGraduated))
	}
}

// Revert undoes an applied grade transition
func (s *gradeTransitionService) Revert(ctx context.Context, id int64, accountID int64) (*TransitionResult, error) {
	transition, err := s.transitionRepo.FindByIDWithMappings(ctx, id)
	if err != nil {
		return nil, fmt.Errorf(errFmtTransitionNotFound, err)
	}

	if err := validateCanRevert(transition); err != nil {
		return nil, err
	}

	history, err := s.transitionRepo.GetHistory(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get transition history: %w", err)
	}

	result := &TransitionResult{
		TransitionID: id,
		Warnings:     make([]string, 0),
	}

	err = s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		return s.executeRevert(ctx, transition, accountID, history, result)
	})

	if err != nil {
		return nil, err
	}

	result.Status = education.TransitionStatusReverted
	result.CanRevert = false

	return result, nil
}

// validateCanRevert checks if a transition can be reverted.
func validateCanRevert(transition *education.GradeTransition) error {
	if transition.CanRevert() {
		return nil
	}
	if transition.IsDraft() {
		return errors.New("transition has not been applied yet")
	}
	return errors.New("transition has already been reverted")
}

// executeRevert performs the actual revert within a transaction.
func (s *gradeTransitionService) executeRevert(
	ctx context.Context,
	transition *education.GradeTransition,
	accountID int64,
	history []*education.GradeTransitionHistory,
	result *TransitionResult,
) error {
	graduatedCount, err := s.revertPromotedStudents(ctx, history, result)
	if err != nil {
		return err
	}

	if graduatedCount > 0 {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("%d graduated students cannot be restored (were permanently deleted)",
				graduatedCount))
	}

	return s.markTransitionReverted(ctx, transition, accountID)
}

// revertPromotedStudents reverts promoted students to their original classes.
// Returns the count of graduated students that cannot be restored and any error.
func (s *gradeTransitionService) revertPromotedStudents(
	ctx context.Context,
	history []*education.GradeTransitionHistory,
	result *TransitionResult,
) (int, error) {
	graduatedCount := 0
	missingStudentCount := 0

	for _, h := range history {
		switch {
		case h.WasPromoted():
			reverted, err := s.revertStudentClass(ctx, h)
			if err != nil {
				// Real database error - propagate to trigger transaction rollback
				return graduatedCount, fmt.Errorf("failed to revert student %d: %w", h.StudentID, err)
			}
			if reverted {
				result.StudentsPromoted++
			} else {
				// Student was deleted after promotion - track for warning
				missingStudentCount++
			}
		case h.WasGraduated():
			graduatedCount++
		}
	}

	if missingStudentCount > 0 {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("%d promoted students could not be reverted (may have been deleted since promotion)",
				missingStudentCount))
	}

	return graduatedCount, nil
}

// revertStudentClass updates a student back to their original class.
// Returns (true, nil) if successful, (false, nil) if student no longer exists,
// or (false, error) if a real database error occurred.
func (s *gradeTransitionService) revertStudentClass(
	ctx context.Context,
	h *education.GradeTransitionHistory,
) (bool, error) {
	// Use transaction from context if available
	var db bun.IDB = s.db
	if tx, ok := base.TxFromContext(ctx); ok && tx != nil {
		db = tx
	}

	result, err := db.NewUpdate().
		Model((*users.Student)(nil)).
		ModelTableExpr(`users.students AS "student"`).
		Set("school_class = ?", h.FromClass).
		Set("updated_at = NOW()").
		Where(`"student".id = ?`, h.StudentID).
		Exec(ctx)
	if err != nil {
		// Real database error - return it to trigger rollback
		return false, err
	}

	// Check if a row was actually updated
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return false, err
	}

	// No rows affected means student was deleted - not an error, just a skip
	return rowsAffected > 0, nil
}

// markTransitionReverted updates the transition status to reverted.
func (s *gradeTransitionService) markTransitionReverted(
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
func (s *gradeTransitionService) GetDistinctClasses(ctx context.Context) ([]string, error) {
	return s.transitionRepo.GetDistinctClasses(ctx)
}

// SuggestMappings auto-suggests class mappings based on naming patterns
func (s *gradeTransitionService) SuggestMappings(ctx context.Context) ([]*SuggestedMapping, error) {
	classes, err := s.transitionRepo.GetDistinctClasses(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get classes: %w", err)
	}

	suggestions := make([]*SuggestedMapping, 0)

	// Pattern: grade number followed by letter(s), e.g., "1a", "2b", "10c"
	classPattern := regexp.MustCompile(`^(\d+)([a-zA-Z]+)$`)

	for _, className := range classes {
		count, err := s.transitionRepo.GetStudentCountByClass(ctx, className)
		if err != nil || count == 0 {
			continue
		}

		matches := classPattern.FindStringSubmatch(className)
		if matches == nil {
			// No pattern match - suggest as graduating
			suggestions = append(suggestions, &SuggestedMapping{
				FromClass:    className,
				ToClass:      nil,
				StudentCount: count,
				IsGraduating: true,
			})
			continue
		}

		// Parse grade number
		gradeNum := 0
		if _, err := fmt.Sscanf(matches[1], "%d", &gradeNum); err != nil {
			continue // Skip if parsing fails
		}
		letterPart := matches[2]

		// Increment grade number
		nextGrade := gradeNum + 1
		nextClass := fmt.Sprintf("%d%s", nextGrade, letterPart)

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
func (s *gradeTransitionService) GetHistory(ctx context.Context, transitionID int64) ([]*education.GradeTransitionHistory, error) {
	return s.transitionRepo.GetHistory(ctx, transitionID)
}
