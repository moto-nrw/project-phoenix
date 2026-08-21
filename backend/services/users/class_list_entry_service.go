package users

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/moto-nrw/project-phoenix/internal/collation"
	auditModels "github.com/moto-nrw/project-phoenix/models/audit"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
)

// Class-list entry errors (#2382). User-facing German messages by design —
// the API renders them verbatim.
var (
	ErrClassListEntryNotFound = errors.New("Klassenlisteneintrag nicht gefunden") //nolint:staticcheck // ST1005: user-facing German message
	// ErrClassListEntryDuplicate: an entry with the same name already exists
	// in the same class (case-insensitive, backed by the DB unique index).
	ErrClassListEntryDuplicate = errors.New("Ein Eintrag mit diesem Namen existiert in dieser Klasse bereits") //nolint:staticcheck // ST1005: user-facing German message
	// ErrClassListEntryStudentExists: a regular student with the same name
	// already exists in the same class — the child is already on the class
	// list, a duplicate entry would list them twice.
	ErrClassListEntryStudentExists = errors.New("Ein Kind mit diesem Namen ist in dieser Klasse bereits angelegt") //nolint:staticcheck // ST1005: user-facing German message
	// ErrClassListEntryStudentNotFound: the assign target does not exist.
	ErrClassListEntryStudentNotFound = errors.New("Das ausgewählte Kind wurde nicht gefunden") //nolint:staticcheck // ST1005: user-facing German message
	// ErrClassListEntryAssignMismatch: the assign target is a real student but
	// does not carry the entry's name and class — resolving the entry into it
	// would delete one child's row in favor of a different child.
	ErrClassListEntryAssignMismatch = errors.New("Das ausgewählte Kind stimmt nicht mit dem Eintrag überein: Name und Klasse müssen übereinstimmen") //nolint:staticcheck // ST1005: user-facing German message
)

// ClassListEntryInput carries the three fields an entry consists of.
type ClassListEntryInput struct {
	FirstName   string
	LastName    string
	SchoolClass string
}

// ClassListEntryWithMatches pairs an entry with the IDs of regular students
// sharing its name and class — the candidates for a deliberate "Zuordnen"
// resolution. Matching is a hint only: resolution is ALWAYS an explicit admin
// action, never automatic (#2382: gleichnamige Kinder dürfen nicht
// verwechselt werden).
type ClassListEntryWithMatches struct {
	Entry              *userModels.ClassListEntry
	MatchingStudentIDs []int64
}

// ClassListEntryService manages the minimal class-list-only entries (#2382):
// children of the class cohort without an OGS record. Every write records an
// audit.class_list_entry_changes row in the same tenant transaction.
type ClassListEntryService interface {
	// List returns all entries of the tenant, class-then-name sorted, each
	// with its matching-student hint.
	List(ctx context.Context) ([]ClassListEntryWithMatches, error)
	// ListAll returns all entries class-then-name sorted, without match
	// hints — the cheap form for export surfaces.
	ListAll(ctx context.Context) ([]*userModels.ClassListEntry, error)
	Create(ctx context.Context, input ClassListEntryInput, changedBy int64) (*userModels.ClassListEntry, error)
	Update(ctx context.Context, id int64, input ClassListEntryInput, changedBy int64) (*userModels.ClassListEntry, error)
	Delete(ctx context.Context, id int64, changedBy int64) error
	// Assign resolves the entry as a duplicate of an existing student: the
	// entry is deleted, the audit row records which student it was attached
	// to. The child keeps exactly one row on the class list — the student.
	Assign(ctx context.Context, id int64, studentID int64, changedBy int64) error
}

type classListEntryService struct {
	entryRepo   userModels.ClassListEntryRepository
	studentRepo userModels.StudentRepository
	auditRepo   auditModels.ClassListEntryChangeRepository
}

// NewClassListEntryService creates a ClassListEntryService.
func NewClassListEntryService(
	entryRepo userModels.ClassListEntryRepository,
	studentRepo userModels.StudentRepository,
	auditRepo auditModels.ClassListEntryChangeRepository,
) ClassListEntryService {
	return &classListEntryService{
		entryRepo:   entryRepo,
		studentRepo: studentRepo,
		auditRepo:   auditRepo,
	}
}

func (s *classListEntryService) ListAll(ctx context.Context) ([]*userModels.ClassListEntry, error) {
	entries, err := s.entryRepo.List(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("list class list entries: %w", err)
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if r := collation.CompareSchoolClasses(entries[i].SchoolClass, entries[j].SchoolClass); r != 0 {
			return r < 0
		}
		if r := collation.CompareGermanNames(entries[i].LastName, entries[i].FirstName, entries[j].LastName, entries[j].FirstName); r != 0 {
			return r < 0
		}
		return entries[i].ID < entries[j].ID
	})
	return entries, nil
}

func (s *classListEntryService) List(ctx context.Context) ([]ClassListEntryWithMatches, error) {
	entries, err := s.ListAll(ctx)
	if err != nil {
		return nil, err
	}

	// One lookup per entry: the entry set is a hand-maintained handful per
	// school, and FindByNameAndClass is an indexed point query — a bulk
	// name-join variant would buy nothing but complexity here.
	out := make([]ClassListEntryWithMatches, 0, len(entries))
	for _, entry := range entries {
		matches, err := s.studentRepo.FindByNameAndClass(ctx, entry.FirstName, entry.LastName, entry.SchoolClass)
		if err != nil {
			return nil, fmt.Errorf("match class list entry %d: %w", entry.ID, err)
		}
		ids := make([]int64, 0, len(matches))
		for _, match := range matches {
			if match != nil {
				ids = append(ids, match.ID)
			}
		}
		out = append(out, ClassListEntryWithMatches{Entry: entry, MatchingStudentIDs: ids})
	}
	return out, nil
}

// requireNoDuplicate rejects the name+class combination when another entry or
// a regular student already carries it. ignoreEntryID skips the entry being
// updated. The DB unique index backs the entry check race-safely; the student
// check is advisory (a student created concurrently surfaces as a match hint
// in List instead).
func (s *classListEntryService) requireNoDuplicate(ctx context.Context, input ClassListEntryInput, ignoreEntryID int64) error {
	existing, err := s.entryRepo.FindByNameAndClass(ctx, input.FirstName, input.LastName, input.SchoolClass)
	if err != nil {
		return fmt.Errorf("class list entry duplicate check: %w", err)
	}
	for _, entry := range existing {
		if entry != nil && entry.ID != ignoreEntryID {
			return ErrClassListEntryDuplicate
		}
	}
	students, err := s.studentRepo.FindByNameAndClass(ctx, input.FirstName, input.LastName, input.SchoolClass)
	if err != nil {
		return fmt.Errorf("class list entry student check: %w", err)
	}
	if len(students) > 0 {
		return ErrClassListEntryStudentExists
	}
	return nil
}

func trimClassListEntryInput(input ClassListEntryInput) ClassListEntryInput {
	return ClassListEntryInput{
		FirstName:   strings.TrimSpace(input.FirstName),
		LastName:    strings.TrimSpace(input.LastName),
		SchoolClass: strings.TrimSpace(input.SchoolClass),
	}
}

func (s *classListEntryService) Create(ctx context.Context, input ClassListEntryInput, changedBy int64) (*userModels.ClassListEntry, error) {
	input = trimClassListEntryInput(input)
	entry := &userModels.ClassListEntry{
		FirstName:   input.FirstName,
		LastName:    input.LastName,
		SchoolClass: input.SchoolClass,
	}
	if changedBy > 0 {
		entry.CreatedBy = &changedBy
	}
	if err := entry.Validate(); err != nil {
		return nil, err
	}
	if err := s.requireNoDuplicate(ctx, input, 0); err != nil {
		return nil, err
	}
	if err := s.entryRepo.Create(ctx, entry); err != nil {
		if modelBase.IsUniqueViolationOn(err, userModels.ClassListEntryUniqueIndexName) {
			// A concurrent create slipped past the advisory check above; the
			// DB index is the race-safe backstop — report it as the duplicate
			// it is, not as a server error.
			return nil, ErrClassListEntryDuplicate
		}
		return nil, fmt.Errorf("create class list entry: %w", err)
	}
	if err := s.recordChange(ctx, entry.ID, auditModels.ClassListEntryActionCreated, "", entry.DisplayValue(), nil, changedBy); err != nil {
		return nil, err
	}
	return entry, nil
}

func (s *classListEntryService) Update(ctx context.Context, id int64, input ClassListEntryInput, changedBy int64) (*userModels.ClassListEntry, error) {
	entry, err := s.findEntry(ctx, id)
	if err != nil {
		return nil, err
	}
	input = trimClassListEntryInput(input)
	oldValue := entry.DisplayValue()

	entry.FirstName = input.FirstName
	entry.LastName = input.LastName
	entry.SchoolClass = input.SchoolClass
	if err := entry.Validate(); err != nil {
		return nil, err
	}
	if entry.DisplayValue() == oldValue {
		return entry, nil
	}
	if err := s.requireNoDuplicate(ctx, input, entry.ID); err != nil {
		return nil, err
	}
	if err := s.entryRepo.Update(ctx, entry); err != nil {
		if modelBase.IsUniqueViolationOn(err, userModels.ClassListEntryUniqueIndexName) {
			return nil, ErrClassListEntryDuplicate
		}
		return nil, fmt.Errorf("update class list entry: %w", err)
	}
	if err := s.recordChange(ctx, entry.ID, auditModels.ClassListEntryActionUpdated, oldValue, entry.DisplayValue(), nil, changedBy); err != nil {
		return nil, err
	}
	return entry, nil
}

func (s *classListEntryService) Delete(ctx context.Context, id int64, changedBy int64) error {
	entry, err := s.findEntry(ctx, id)
	if err != nil {
		return err
	}
	if err := s.entryRepo.Delete(ctx, entry.ID); err != nil {
		return fmt.Errorf("delete class list entry: %w", err)
	}
	return s.recordChange(ctx, entry.ID, auditModels.ClassListEntryActionDeleted, entry.DisplayValue(), "", nil, changedBy)
}

func (s *classListEntryService) Assign(ctx context.Context, id int64, studentID int64, changedBy int64) error {
	entry, err := s.findEntry(ctx, id)
	if err != nil {
		return err
	}
	student, err := s.studentRepo.FindByID(ctx, studentID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrClassListEntryStudentNotFound
		}
		return fmt.Errorf("assign class list entry: load student: %w", err)
	}
	if student == nil || student.IsAlumnus() {
		return ErrClassListEntryStudentNotFound
	}
	// The target must BE the child the entry names: same name in the same
	// class, matched exactly like the "Mögliche Dublette" hint (case-
	// insensitive, trimmed). Anything else would resolve the entry into an
	// unrelated child and silently drop the named child from the list.
	matches, err := s.studentRepo.FindByNameAndClass(ctx, entry.FirstName, entry.LastName, entry.SchoolClass)
	if err != nil {
		return fmt.Errorf("assign class list entry: match student: %w", err)
	}
	matched := false
	for _, match := range matches {
		if match != nil && match.ID == student.ID {
			matched = true
			break
		}
	}
	if !matched {
		return ErrClassListEntryAssignMismatch
	}
	if err := s.entryRepo.Delete(ctx, entry.ID); err != nil {
		return fmt.Errorf("assign class list entry: delete entry: %w", err)
	}
	return s.recordChange(ctx, entry.ID, auditModels.ClassListEntryActionAssigned, entry.DisplayValue(), "", &studentID, changedBy)
}

func (s *classListEntryService) findEntry(ctx context.Context, id int64) (*userModels.ClassListEntry, error) {
	entry, err := s.entryRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrClassListEntryNotFound
		}
		return nil, fmt.Errorf("load class list entry %d: %w", id, err)
	}
	if entry == nil {
		return nil, ErrClassListEntryNotFound
	}
	return entry, nil
}

// recordChange writes the audit row in the same tenant transaction as the
// change. A nil audit repo (pure unit tests) disables the trail.
func (s *classListEntryService) recordChange(ctx context.Context, entryID int64, action, oldValue, newValue string, matchedStudentID *int64, changedBy int64) error {
	if s.auditRepo == nil {
		return nil
	}
	change := &auditModels.ClassListEntryChange{
		EntryID:          entryID,
		Action:           action,
		OldValue:         oldValue,
		NewValue:         newValue,
		MatchedStudentID: matchedStudentID,
		ChangedBy:        changedBy,
	}
	if err := s.auditRepo.Create(ctx, change); err != nil {
		return fmt.Errorf("record class list entry change: %w", err)
	}
	return nil
}
