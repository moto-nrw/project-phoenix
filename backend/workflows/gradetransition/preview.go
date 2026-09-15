package gradetransition

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/moto-nrw/project-phoenix/modules/peopledirectory"
	"github.com/moto-nrw/project-phoenix/modules/schoolstructure"
)

// cohortStudent is one child of a mapped class as resolved for a preview or
// under the apply's row locks.
type cohortStudent struct {
	StudentID   int64
	PersonID    int64
	SchoolClass string
	Status      string
}

// Preview returns what will happen when the transition is applied. The
// displayed counts and the fingerprint that binds the admin's confirmation
// MUST describe the same cohort, so both are derived from ONE read: under
// READ COMMITTED a second read sees a newer snapshot, and the confirmation
// would cover children the modal never displayed.
func (w *Workflow) Preview(ctx context.Context, id int64) (result Preview, err error) {
	err = w.run(ctx, "preview", OperationRead, id, func(txCtx context.Context, _ Actor) error {
		transition, err := w.deps.Structure.FindTransition(txCtx, id)
		if err != nil {
			return translateStructureError(err)
		}
		cohort, err := w.cohortOf(txCtx, mappedClassNames(transition.Mappings))
		if err != nil {
			return err
		}
		preview := Preview{
			TransitionID: id, AcademicYear: transition.AcademicYear,
			ByMapping: make([]MappingPreview, 0, len(transition.Mappings)), Warnings: make([]string, 0),
			UnmappedClasses: make([]UnmappedClass, 0),
		}
		counts := countByClass(cohort)
		mapped := make(map[string]bool, len(transition.Mappings))
		for _, mapping := range transition.Mappings {
			count := counts[mapping.FromClass]
			action := schoolstructure.TransitionActionPromoted
			if mapping.IsGraduating() {
				action = schoolstructure.TransitionActionGraduated
				preview.ToGraduate += count
			} else {
				preview.ToPromote += count
			}
			preview.ByMapping = append(preview.ByMapping, MappingPreview{
				FromClass: mapping.FromClass, ToClass: mapping.ToClass, StudentCount: count, Action: action,
			})
			preview.TotalStudents += count
			mapped[mapping.FromClass] = true
		}
		unmapped, err := w.unmappedClasses(txCtx, mapped)
		if err != nil {
			return err
		}
		preview.UnmappedClasses = unmapped
		preview.Fingerprint = fingerprintOf(transition.Mappings, cohort)
		if len(preview.UnmappedClasses) > 0 {
			preview.Warnings = append(preview.Warnings,
				fmt.Sprintf("%d classes with students are not included in this transition", len(preview.UnmappedClasses)))
		}
		if preview.ToGraduate > 0 {
			preview.Warnings = append(preview.Warnings,
				fmt.Sprintf("%d students will be marked as alumni and hidden from the app (graduates)", preview.ToGraduate))
		}
		result = preview
		return nil
	})
	if err != nil {
		return Preview{}, err
	}
	return result, nil
}

// cohortOf lists the non-alumni children of the classes in class/id order.
func (w *Workflow) cohortOf(ctx context.Context, classes []string) ([]cohortStudent, error) {
	if len(classes) == 0 {
		return []cohortStudent{}, nil
	}
	students, err := w.deps.Directory.ListStudentsByClasses(ctx, classes)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve transition cohort: %w", err)
	}
	cohort := make([]cohortStudent, 0, len(students))
	for _, student := range students {
		cohort = append(cohort, toCohortStudent(student))
	}
	return cohort, nil
}

func toCohortStudent(student peopledirectory.Student) cohortStudent {
	return cohortStudent{StudentID: student.ID, PersonID: student.PersonID, SchoolClass: student.SchoolClass, Status: student.Status}
}

// unmappedClasses lists the classes with children that the transition does
// not touch.
func (w *Workflow) unmappedClasses(ctx context.Context, mapped map[string]bool) ([]UnmappedClass, error) {
	classes, err := w.deps.Directory.ListSchoolClasses(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get distinct classes: %w", err)
	}
	counts, err := w.studentCountsByClass(ctx, classes)
	if err != nil {
		return nil, err
	}
	result := make([]UnmappedClass, 0)
	for _, className := range classes {
		if mapped[className] || counts[className] == 0 {
			continue
		}
		result = append(result, UnmappedClass{ClassName: className, StudentCount: counts[className]})
	}
	return result, nil
}

func (w *Workflow) studentCountsByClass(ctx context.Context, classes []string) (map[string]int, error) {
	cohort, err := w.cohortOf(ctx, classes)
	if err != nil {
		return nil, fmt.Errorf("failed to count students by class: %w", err)
	}
	return countByClass(cohort), nil
}

func countByClass(cohort []cohortStudent) map[string]int {
	counts := make(map[string]int)
	for _, student := range cohort {
		counts[student.SchoolClass]++
	}
	return counts
}

// mappedClassNames lists the distinct source classes a transition touches.
func mappedClassNames(mappings []schoolstructure.TransitionMapping) []string {
	seen := make(map[string]bool, len(mappings))
	classes := make([]string, 0, len(mappings))
	for _, mapping := range mappings {
		if seen[mapping.FromClass] {
			continue
		}
		seen[mapping.FromClass] = true
		classes = append(classes, mapping.FromClass)
	}
	return classes
}

// fingerprintOf builds the stable digest both Preview and Apply compute. The
// inputs are sorted, so neither read's row order can change the result. The
// mapping action is encoded as its own field instead of a "graduate" target
// sentinel, and every free-text field is quoted, so no class name can spoof
// the separators or turn a promotion into a graduation unnoticed.
func fingerprintOf(mappings []schoolstructure.TransitionMapping, cohort []cohortStudent) string {
	parts := make([]string, 0, len(mappings)+len(cohort))
	for _, mapping := range mappings {
		if mapping.ToClass == nil {
			parts = append(parts, fmt.Sprintf("m|graduate|%s", strconv.Quote(mapping.FromClass)))
			continue
		}
		parts = append(parts, fmt.Sprintf("m|promote|%s|%s", strconv.Quote(mapping.FromClass), strconv.Quote(*mapping.ToClass)))
	}
	for _, student := range cohort {
		parts = append(parts, fmt.Sprintf("s|%d|%s", student.StudentID, strconv.Quote(student.SchoolClass)))
	}
	sort.Strings(parts)
	sum := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return hex.EncodeToString(sum[:])
}

// ensureFingerprintMatches compares the caller's previewed cohort against
// the one resolved under the locks. An empty expectation means the caller
// has no preview to be stale and skips the check.
func ensureFingerprintMatches(expected string, mappings []schoolstructure.TransitionMapping, promotions, graduates []cohortStudent) error {
	if strings.TrimSpace(expected) == "" {
		return nil
	}
	current := make([]cohortStudent, 0, len(promotions)+len(graduates))
	current = append(current, promotions...)
	current = append(current, graduates...)
	if !equalFingerprint(fingerprintOf(mappings, current), expected) {
		return ErrPreviewStale
	}
	return nil
}

func equalFingerprint(actual, expected string) bool {
	actualBytes, actualErr := hex.DecodeString(actual)
	expectedBytes, expectedErr := hex.DecodeString(strings.TrimSpace(expected))
	if actualErr != nil || expectedErr != nil || len(actualBytes) != len(expectedBytes) {
		return false
	}
	return subtle.ConstantTimeCompare(actualBytes, expectedBytes) == 1
}

// classPattern matches an optional prefix, a grade number and optional
// letters: "1a", "10c", "1" and "Klasse 1a". The prefix is kept when the
// grade is incremented.
var classPattern = regexp.MustCompile(`^(.*?)(\d+)([a-zA-Z]*)$`)

// SuggestMappings proposes a mapping per class from its name: grades below
// four advance, grades four and up leave, and a class without a grade number
// is marked ambiguous so the editor requires an explicit choice.
func (w *Workflow) SuggestMappings(ctx context.Context) (result []SuggestedMapping, err error) {
	err = w.run(ctx, "suggest_mappings", OperationRead, 0, func(txCtx context.Context, _ Actor) error {
		classes, err := w.deps.Directory.ListSchoolClasses(txCtx)
		if err != nil {
			return fmt.Errorf("failed to get classes: %w", err)
		}
		counts, err := w.studentCountsByClass(txCtx, classes)
		if err != nil {
			return err
		}
		result = suggestMappings(classes, counts)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func suggestMappings(classes []string, counts map[string]int) []SuggestedMapping {
	suggestions := make([]SuggestedMapping, 0, len(classes))
	for _, className := range classes {
		count := counts[className]
		if count == 0 {
			continue
		}
		matches := classPattern.FindStringSubmatch(className)
		if matches == nil {
			suggestions = append(suggestions, SuggestedMapping{FromClass: className, StudentCount: count, IsGraduating: true, Ambiguous: true})
			continue
		}
		grade, err := strconv.Atoi(matches[2])
		if err != nil {
			continue
		}
		if grade >= 4 {
			suggestions = append(suggestions, SuggestedMapping{FromClass: className, StudentCount: count, IsGraduating: true})
			continue
		}
		next := fmt.Sprintf("%s%d%s", matches[1], grade+1, matches[3])
		suggestions = append(suggestions, SuggestedMapping{FromClass: className, ToClass: &next, StudentCount: count})
	}
	sort.Slice(suggestions, func(i, j int) bool { return suggestions[i].FromClass < suggestions[j].FromClass })
	return suggestions
}
