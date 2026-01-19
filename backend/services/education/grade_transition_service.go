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
	TransitionID    int64                `json:"transition_id"`
	AcademicYear    string               `json:"academic_year"`
	TotalStudents   int                  `json:"total_students"`
	ToPromote       int                  `json:"to_promote"`
	ToGraduate      int                  `json:"to_graduate"`
	ByMapping       []MappingPreview     `json:"by_mapping"`
	UnmappedClasses []UnmappedClassInfo  `json:"unmapped_classes"`
	Warnings        []string             `json:"warnings"`
}

// MappingPreview shows the impact of a single mapping
type MappingPreview struct {
	FromClass    string `json:"from_class"`
	ToClass      *string `json:"to_class,omitempty"`
	StudentCount int    `json:"student_count"`
	Action       string `json:"action"` // "promote" or "graduate"
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
		// Create the transition
		if err := s.transitionRepo.Create(ctx, transition); err != nil {
			return fmt.Errorf("failed to create transition: %w", err)
		}

		// Create mappings if provided
		if len(req.Mappings) > 0 {
			mappings := make([]*education.GradeTransitionMapping, 0, len(req.Mappings))
			for _, m := range req.Mappings {
				mapping := &education.GradeTransitionMapping{
					TransitionID: transition.ID,
					FromClass:    m.FromClass,
					ToClass:      m.ToClass,
				}
				if err := mapping.Validate(); err != nil {
					return fmt.Errorf("invalid mapping for class %s: %w", m.FromClass, err)
				}
				mappings = append(mappings, mapping)
			}
			if err := s.transitionRepo.CreateMappings(ctx, mappings); err != nil {
				return fmt.Errorf("failed to create mappings: %w", err)
			}
			transition.Mappings = mappings
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return transition, nil
}

// Update updates a grade transition
func (s *gradeTransitionService) Update(ctx context.Context, id int64, req UpdateTransitionRequest) (*education.GradeTransition, error) {
	transition, err := s.transitionRepo.FindByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("transition not found: %w", err)
	}

	if !transition.CanModify() {
		return nil, errors.New("cannot modify transition: must be in draft status")
	}

	// Update fields
	if req.AcademicYear != nil {
		transition.AcademicYear = *req.AcademicYear
	}
	if req.Notes != nil {
		transition.Notes = req.Notes
	}

	if err := transition.Validate(); err != nil {
		return nil, err
	}

	// Execute in transaction
	err = s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		// Update the transition
		if err := s.transitionRepo.Update(ctx, transition); err != nil {
			return fmt.Errorf("failed to update transition: %w", err)
		}

		// Update mappings if provided
		if req.Mappings != nil {
			// Delete existing mappings
			if err := s.transitionRepo.DeleteMappings(ctx, id); err != nil {
				return fmt.Errorf("failed to delete existing mappings: %w", err)
			}

			// Create new mappings
			if len(req.Mappings) > 0 {
				mappings := make([]*education.GradeTransitionMapping, 0, len(req.Mappings))
				for _, m := range req.Mappings {
					mapping := &education.GradeTransitionMapping{
						TransitionID: id,
						FromClass:    m.FromClass,
						ToClass:      m.ToClass,
					}
					if err := mapping.Validate(); err != nil {
						return fmt.Errorf("invalid mapping for class %s: %w", m.FromClass, err)
					}
					mappings = append(mappings, mapping)
				}
				if err := s.transitionRepo.CreateMappings(ctx, mappings); err != nil {
					return fmt.Errorf("failed to create mappings: %w", err)
				}
				transition.Mappings = mappings
			}
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Reload with mappings
	return s.transitionRepo.FindByIDWithMappings(ctx, id)
}

// Delete deletes a draft grade transition
func (s *gradeTransitionService) Delete(ctx context.Context, id int64) error {
	transition, err := s.transitionRepo.FindByID(ctx, id)
	if err != nil {
		return fmt.Errorf("transition not found: %w", err)
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
		return nil, fmt.Errorf("transition not found: %w", err)
	}

	preview := &TransitionPreview{
		TransitionID:  id,
		AcademicYear:  transition.AcademicYear,
		ByMapping:     make([]MappingPreview, 0),
		Warnings:      make([]string, 0),
	}

	// Track which classes are covered by mappings
	mappedClasses := make(map[string]bool)

	// Get preview for each mapping
	for _, mapping := range transition.Mappings {
		count, err := s.transitionRepo.GetStudentCountByClass(ctx, mapping.FromClass)
		if err != nil {
			return nil, fmt.Errorf("failed to count students in class %s: %w", mapping.FromClass, err)
		}

		action := "promote"
		if mapping.IsGraduating() {
			action = "graduate"
			preview.ToGraduate += count
		} else {
			preview.ToPromote += count
		}

		preview.ByMapping = append(preview.ByMapping, MappingPreview{
			FromClass:    mapping.FromClass,
			ToClass:      mapping.ToClass,
			StudentCount: count,
			Action:       action,
		})

		preview.TotalStudents += count
		mappedClasses[mapping.FromClass] = true
	}

	// Find unmapped classes
	allClasses, err := s.transitionRepo.GetDistinctClasses(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get distinct classes: %w", err)
	}

	for _, className := range allClasses {
		if !mappedClasses[className] {
			count, err := s.transitionRepo.GetStudentCountByClass(ctx, className)
			if err != nil {
				continue // Skip on error
			}
			if count > 0 {
				preview.UnmappedClasses = append(preview.UnmappedClasses, UnmappedClassInfo{
					ClassName:    className,
					StudentCount: count,
				})
			}
		}
	}

	// Add warnings
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

	return preview, nil
}

// Apply executes the grade transition
func (s *gradeTransitionService) Apply(ctx context.Context, id int64, accountID int64) (*TransitionResult, error) {
	transition, err := s.transitionRepo.FindByIDWithMappings(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("transition not found: %w", err)
	}

	if !transition.CanApply() {
		if transition.IsApplied() {
			return nil, errors.New("transition has already been applied")
		}
		if transition.IsReverted() {
			return nil, errors.New("transition has been reverted")
		}
		return nil, errors.New("cannot apply transition: must be in draft status with mappings")
	}

	result := &TransitionResult{
		TransitionID: id,
		Warnings:     make([]string, 0),
	}

	// Execute in transaction
	err = s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		// Separate mappings into promote vs graduate
		var promoteFromClasses []string
		var graduateClasses []string

		for _, mapping := range transition.Mappings {
			if mapping.IsGraduating() {
				graduateClasses = append(graduateClasses, mapping.FromClass)
			} else {
				promoteFromClasses = append(promoteFromClasses, mapping.FromClass)
			}
		}

		// Get all affected students for history before any changes
		allClasses := append(promoteFromClasses, graduateClasses...)
		students, err := s.transitionRepo.GetStudentsByClasses(ctx, allClasses)
		if err != nil {
			return fmt.Errorf("failed to get students: %w", err)
		}

		// Create a map of from_class -> to_class for quick lookup
		classMapping := make(map[string]*string)
		for _, mapping := range transition.Mappings {
			classMapping[mapping.FromClass] = mapping.ToClass
		}

		// Create history records
		historyRecords := make([]*education.GradeTransitionHistory, 0, len(students))
		for _, student := range students {
			toClass := classMapping[student.SchoolClass]
			action := education.ActionPromoted
			if toClass == nil {
				action = education.ActionGraduated
			}

			historyRecords = append(historyRecords, &education.GradeTransitionHistory{
				TransitionID: id,
				StudentID:    student.StudentID,
				PersonName:   student.PersonName,
				FromClass:    student.SchoolClass,
				ToClass:      toClass,
				Action:       action,
			})
		}

		if len(historyRecords) > 0 {
			if err := s.transitionRepo.CreateHistoryBatch(ctx, historyRecords); err != nil {
				return fmt.Errorf("failed to create history: %w", err)
			}
		}

		// Apply promotions (UPDATE students SET school_class = to_class)
		if len(promoteFromClasses) > 0 {
			promoted, err := s.transitionRepo.UpdateStudentClasses(ctx, id)
			if err != nil {
				return fmt.Errorf("failed to promote students: %w", err)
			}
			result.StudentsPromoted = int(promoted)
		}

		// Delete graduating students
		if len(graduateClasses) > 0 {
			// Count graduates first
			for _, className := range graduateClasses {
				count, _ := s.transitionRepo.GetStudentCountByClass(ctx, className)
				result.StudentsGraduated += count
			}

			// Delete students (CASCADE handles related data)
			_, err := s.transitionRepo.DeleteStudentsByClasses(ctx, graduateClasses)
			if err != nil {
				return fmt.Errorf("failed to delete graduating students: %w", err)
			}
		}

		// Update transition status
		now := time.Now()
		transition.Status = education.TransitionStatusApplied
		transition.AppliedAt = &now
		transition.AppliedBy = &accountID

		if err := s.transitionRepo.Update(ctx, transition); err != nil {
			return fmt.Errorf("failed to update transition status: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	result.Status = education.TransitionStatusApplied
	result.CanRevert = true

	if result.StudentsGraduated > 0 {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("%d students were permanently deleted (graduates)",
				result.StudentsGraduated))
	}

	return result, nil
}

// Revert undoes an applied grade transition
func (s *gradeTransitionService) Revert(ctx context.Context, id int64, accountID int64) (*TransitionResult, error) {
	transition, err := s.transitionRepo.FindByIDWithMappings(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("transition not found: %w", err)
	}

	if !transition.CanRevert() {
		if transition.IsDraft() {
			return nil, errors.New("transition has not been applied yet")
		}
		return nil, errors.New("transition has already been reverted")
	}

	// Get history to know what to revert
	history, err := s.transitionRepo.GetHistory(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("failed to get transition history: %w", err)
	}

	result := &TransitionResult{
		TransitionID: id,
		Warnings:     make([]string, 0),
	}

	// Execute in transaction
	err = s.txHandler.RunInTx(ctx, func(ctx context.Context, tx bun.Tx) error {
		graduatedCount := 0

		// Revert promoted students
		for _, h := range history {
			if h.WasPromoted() {
				// Update student back to original class
				_, err := s.db.NewUpdate().
					Model((*users.Student)(nil)).
					ModelTableExpr(`users.students AS "student"`).
					Set("school_class = ?", h.FromClass).
					Set("updated_at = NOW()").
					Where(`"student".id = ?`, h.StudentID).
					Exec(ctx)
				if err != nil {
					// Student might have been deleted - skip
					continue
				}
				result.StudentsPromoted++
			} else if h.WasGraduated() {
				graduatedCount++
			}
		}

		if graduatedCount > 0 {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("%d graduated students cannot be restored (were permanently deleted)",
					graduatedCount))
		}

		// Update transition status
		now := time.Now()
		transition.Status = education.TransitionStatusReverted
		transition.RevertedAt = &now
		transition.RevertedBy = &accountID

		if err := s.transitionRepo.Update(ctx, transition); err != nil {
			return fmt.Errorf("failed to update transition status: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	result.Status = education.TransitionStatusReverted
	result.CanRevert = false

	return result, nil
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
