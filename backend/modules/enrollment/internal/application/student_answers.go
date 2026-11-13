package application

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
	"github.com/moto-ogs/project-phoenix/sharedkernel/collation"
)

// StudentAnswerRecords reads the linked answers and their pinned form
// versions.
type StudentAnswerRecords interface {
	LinkedChildAnswers(ctx context.Context) ([]enrollment.LinkedChildAnswers, error)
	StudentExtraFieldSchemas(ctx context.Context, ids []int64) ([]enrollment.SchemaVersionResult, error)
}

// StudentAnswers implements enrollment.StudentAnswers over one read of the
// school's linked answers.
type StudentAnswers struct{ records StudentAnswerRecords }

// NewStudentAnswers builds the read; records is required.
func NewStudentAnswers(records StudentAnswerRecords) *StudentAnswers {
	if records == nil {
		panic("enrollment: student answer records are required")
	}
	return &StudentAnswers{records: records}
}

// answerableTypes are the free question types with an answer text.
var answerableTypes = map[enrollment.FormFieldType]bool{
	enrollment.FormFieldBoolean: true, enrollment.FormFieldSelect: true,
	enrollment.FormFieldText: true, enrollment.FormFieldTextarea: true,
	enrollment.FormFieldNumber: true, enrollment.FormFieldDate: true,
}

// studentAnswer is a child's newest answer to one question.
type studentAnswer struct {
	studentID int64
	field     enrollment.FormField
	// value is the canonical filter value ("true", "false", an option
	// value), text the answer as the school reads it.
	value, text string
	// affirmative says the answer states something: yes or any filled
	// answer, never no.
	affirmative bool
}

// answerSet is the school's newest answers, newest registration first, and
// the newest definition of every question.
type answerSet struct {
	answers   []studentAnswer
	questions map[string]enrollment.FormField
	versions  map[string]int64
}

func (s *StudentAnswers) load(ctx context.Context) (answerSet, error) {
	set := answerSet{questions: map[string]enrollment.FormField{}, versions: map[string]int64{}}
	rows, err := s.records.LinkedChildAnswers(ctx)
	if err != nil {
		return set, err
	}
	schemas, err := s.schemas(ctx, rows)
	if err != nil {
		return set, err
	}
	seen := map[string]bool{}
	for _, row := range rows {
		schema := schemas[row.SchemaID]
		for _, field := range schema.Fields {
			if !field.AppliesToCh || field.Target != "" || !answerableTypes[field.Type] {
				continue
			}
			if set.versions[field.Key] < schema.ID {
				set.questions[field.Key], set.versions[field.Key] = field, schema.ID
			}
			raw, ok := row.CustomData.Lookup(field.Key)
			key := strconv.FormatInt(row.StudentID, 10) + "\x00" + field.Key
			if !ok || seen[key] {
				continue
			}
			answer, ok := readAnswer(field, raw)
			if !ok {
				continue
			}
			seen[key] = true
			answer.studentID = row.StudentID
			set.answers = append(set.answers, answer)
		}
	}
	return set, nil
}

// schemas loads every form version the rows were given in. A version that
// cannot be read fails the read, as on the child's page.
func (s *StudentAnswers) schemas(ctx context.Context, rows []enrollment.LinkedChildAnswers) (map[int64]*enrollment.FormSchema, error) {
	ids := make([]int64, 0)
	for _, row := range rows {
		if !slices.Contains(ids, row.SchemaID) {
			ids = append(ids, row.SchemaID)
		}
	}
	out := make(map[int64]*enrollment.FormSchema, len(ids))
	if len(ids) == 0 {
		return out, nil
	}
	results, err := s.records.StudentExtraFieldSchemas(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("student answers: load form versions: %w", err)
	}
	for _, result := range results {
		if result.Err != nil {
			return nil, fmt.Errorf("student answers: form version %d: %w", result.ID, result.Err)
		}
		if result.Schema == nil {
			return nil, fmt.Errorf("student answers: form version %d: %w", result.ID, enrollment.ErrFormSchemaNotFound)
		}
		out[result.ID] = result.Schema
	}
	for _, id := range ids {
		if out[id] == nil {
			return nil, fmt.Errorf("student answers: form version %d: %w", id, enrollment.ErrFormSchemaNotFound)
		}
	}
	return out, nil
}

// readAnswer turns one stored value into the answer the school reads. A value
// of the wrong shape or an empty one is no answer.
func readAnswer(field enrollment.FormField, raw any) (studentAnswer, bool) {
	answer := studentAnswer{field: field}
	switch field.Type {
	case enrollment.FormFieldBoolean:
		yes, ok := raw.(bool)
		if !ok {
			return answer, false
		}
		answer.value, answer.text, answer.affirmative = strconv.FormatBool(yes), "Nein", yes
		if yes {
			answer.text = "Ja"
		}
		return answer, true
	case enrollment.FormFieldNumber:
		number, ok := raw.(float64)
		if !ok {
			return answer, false
		}
		answer.text = strings.ReplaceAll(strconv.FormatFloat(number, 'f', -1, 64), ".", ",")
	case enrollment.FormFieldSelect:
		value, ok := raw.(string)
		if !ok || strings.TrimSpace(value) == "" {
			return answer, false
		}
		answer.value, answer.text = value, optionLabel(field, value)
	case enrollment.FormFieldDate:
		value, ok := raw.(string)
		if !ok || value == "" {
			return answer, false
		}
		answer.text = value
		if date, err := calendar.ParseDate(value); err == nil {
			answer.text = date.Format("02.01.2006")
		}
	default:
		value, ok := raw.(string)
		if !ok || strings.TrimSpace(value) == "" {
			return answer, false
		}
		answer.text = strings.TrimSpace(value)
	}
	answer.affirmative = true
	return answer, true
}

func optionLabel(field enrollment.FormField, value string) string {
	for _, option := range field.Options {
		if option.Value == value {
			return option.Label
		}
	}
	return value
}

// StudentAnswerFilters lists the yes/no and choice questions with answers.
func (s *StudentAnswers) StudentAnswerFilters(ctx context.Context) ([]enrollment.StudentAnswerFilter, error) {
	set, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	counts := map[string]map[string]int{}
	for _, answer := range set.answers {
		if answer.value == "" {
			continue
		}
		key := answer.field.Key
		if counts[key] == nil {
			counts[key] = map[string]int{}
		}
		counts[key][answer.value]++
	}
	filters := make([]enrollment.StudentAnswerFilter, 0, len(counts))
	for key, byValue := range counts {
		question := set.questions[key]
		filters = append(filters, enrollment.StudentAnswerFilter{
			Key: key, Label: question.Label, Type: question.Type, Options: filterOptions(question, byValue),
		})
	}
	slices.SortFunc(filters, func(a, b enrollment.StudentAnswerFilter) int {
		if c := collation.CompareGerman(a.Label, b.Label); c != 0 {
			return c
		}
		return strings.Compare(a.Key, b.Key)
	})
	return filters, nil
}

// filterOptions lists the answers in the question's order: yes before no, a
// choice in the order of the newest form, answers no longer offered last.
func filterOptions(question enrollment.FormField, counts map[string]int) []enrollment.StudentAnswerOption {
	options := []enrollment.StudentAnswerOption{}
	listed := map[string]bool{}
	add := func(value, label string) {
		listed[value] = true
		options = append(options, enrollment.StudentAnswerOption{Value: value, Label: label, Count: counts[value]})
	}
	if question.Type == enrollment.FormFieldBoolean {
		add("true", "Ja")
		add("false", "Nein")
		return options
	}
	for _, option := range question.Options {
		if counts[option.Value] > 0 {
			add(option.Value, option.Label)
		}
	}
	rest := make([]string, 0)
	for value := range counts {
		if !listed[value] {
			rest = append(rest, value)
		}
	}
	slices.Sort(rest)
	for _, value := range rest {
		add(value, value)
	}
	return options
}

// StudentsWithAnswer returns the children whose newest answer to key is value.
func (s *StudentAnswers) StudentsWithAnswer(ctx context.Context, key, value string) ([]int64, error) {
	if key == "" || value == "" {
		return nil, errors.New("student answers: key and value are required")
	}
	set, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	ids := make([]int64, 0)
	for _, answer := range set.answers {
		if answer.field.Key == key && answer.value == value {
			ids = append(ids, answer.studentID)
		}
	}
	slices.Sort(ids)
	return ids, nil
}

// SearchStudentAnswers finds children by question or answer text.
func (s *StudentAnswers) SearchStudentAnswers(ctx context.Context, text string, limit int) ([]enrollment.StudentAnswerMatch, error) {
	query := collation.FoldForSearch(strings.TrimSpace(text))
	if query == "" || limit <= 0 {
		return []enrollment.StudentAnswerMatch{}, nil
	}
	set, err := s.load(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]enrollment.StudentAnswerMatch, 0, limit)
	found := map[int64]bool{}
	for _, answer := range set.answers {
		if found[answer.studentID] || !answerMatches(answer, set.questions[answer.field.Key].Label, query) {
			continue
		}
		found[answer.studentID] = true
		out = append(out, enrollment.StudentAnswerMatch{
			StudentID: answer.studentID, Label: set.questions[answer.field.Key].Label, Answer: answer.text,
		})
		if len(out) == limit {
			break
		}
	}
	return out, nil
}

// answerMatches compares the folded query with the question of a stated
// answer and with the text of any answer but yes/no.
func answerMatches(answer studentAnswer, label, query string) bool {
	if answer.affirmative && (strings.Contains(collation.FoldForSearch(label), query) ||
		strings.Contains(collation.FoldForSearch(answer.field.Label), query)) {
		return true
	}
	return answer.field.Type != enrollment.FormFieldBoolean && strings.Contains(collation.FoldForSearch(answer.text), query)
}
