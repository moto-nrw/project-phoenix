package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/enrollment"
	"github.com/stretchr/testify/require"
)

type studentAnswerRecords struct {
	rows    []enrollment.LinkedChildAnswers
	schemas map[int64]*enrollment.FormSchema
	err     error
}

func (r studentAnswerRecords) LinkedChildAnswers(context.Context) ([]enrollment.LinkedChildAnswers, error) {
	return r.rows, nil
}

func (r studentAnswerRecords) StudentExtraFieldSchemas(_ context.Context, ids []int64) ([]enrollment.SchemaVersionResult, error) {
	out := make([]enrollment.SchemaVersionResult, 0, len(ids))
	for _, id := range ids {
		out = append(out, enrollment.SchemaVersionResult{ID: id, Schema: r.schemas[id], Err: r.err})
	}
	return out, nil
}

func answers(values map[string]any) enrollment.FormValues {
	var out enrollment.FormValues
	for key, value := range values {
		out.Set(key, value)
	}
	return out
}

// answerFixture: two form versions; the newer one renames the swimming
// question and offers a third club option. Student 11 registered twice, the
// newer answer counts. Student 13 answered no.
func answerFixture() studentAnswerRecords {
	older := &enrollment.FormSchema{ID: 4, Fields: []enrollment.FormField{
		{Key: "seepferdchen", Label: "Seepferdchen?", Type: enrollment.FormFieldBoolean, AppliesToCh: true},
		{Key: "club", Label: "AG", Type: enrollment.FormFieldSelect, AppliesToCh: true, Options: []enrollment.FormFieldOption{{Value: "chor", Label: "Chor"}}},
	}}
	newer := &enrollment.FormSchema{ID: 7, Fields: []enrollment.FormField{
		{Key: "seepferdchen", Label: "Seepferdchen vorhanden", Type: enrollment.FormFieldBoolean, AppliesToCh: true},
		{Key: "club", Label: "Wunsch-AG", Type: enrollment.FormFieldSelect, AppliesToCh: true, Options: []enrollment.FormFieldOption{{Value: "chor", Label: "Chor"}, {Value: "fussball", Label: "Fußball"}}},
		{Key: "allergies", Label: "Allergien", Type: enrollment.FormFieldTextarea, AppliesToCh: true},
		{Key: "health", Label: "Gesundheit", Type: enrollment.FormFieldTextarea, AppliesToCh: true, Target: enrollment.TargetStudentHealthInfo},
		{Key: "parent_job", Label: "Beruf", Type: enrollment.FormFieldText},
		{Key: "since", Label: "Schwimmt seit", Type: enrollment.FormFieldDate, AppliesToCh: true},
	}}
	day := time.Date(2026, 9, 1, 8, 0, 0, 0, time.UTC)
	return studentAnswerRecords{
		schemas: map[int64]*enrollment.FormSchema{4: older, 7: newer},
		rows: []enrollment.LinkedChildAnswers{
			{StudentID: 11, SchemaID: 7, SubmittedAt: day.AddDate(1, 0, 0), CustomData: answers(map[string]any{
				"seepferdchen": true, "club": "fussball", "allergies": "Nüsse", "health": "Asthma", "parent_job": "Ärztin", "since": "2025-06-01",
			})},
			{StudentID: 12, SchemaID: 7, SubmittedAt: day, CustomData: answers(map[string]any{"seepferdchen": true, "club": "chor"})},
			{StudentID: 13, SchemaID: 7, SubmittedAt: day, CustomData: answers(map[string]any{"seepferdchen": false})},
			{StudentID: 11, SchemaID: 4, SubmittedAt: day.AddDate(-1, 0, 0), CustomData: answers(map[string]any{"seepferdchen": false, "club": "chor"})},
		},
	}
}

func TestStudentAnswerFiltersCountTheNewestAnswerPerChild(t *testing.T) {
	t.Parallel()
	filters, err := NewStudentAnswers(answerFixture()).StudentAnswerFilters(context.Background())
	require.NoError(t, err)
	require.Equal(t, []enrollment.StudentAnswerFilter{
		{Key: "seepferdchen", Label: "Seepferdchen vorhanden", Type: enrollment.FormFieldBoolean, Options: []enrollment.StudentAnswerOption{
			{Value: "true", Label: "Ja", Count: 2}, {Value: "false", Label: "Nein", Count: 1},
		}},
		{Key: "club", Label: "Wunsch-AG", Type: enrollment.FormFieldSelect, Options: []enrollment.StudentAnswerOption{
			{Value: "chor", Label: "Chor", Count: 1}, {Value: "fussball", Label: "Fußball", Count: 1},
		}},
	}, filters, "newest labels; text, date, targeted and guardian questions are no filters")
}

func TestStudentsWithAnswerUsesEachChildsNewestAnswer(t *testing.T) {
	t.Parallel()
	reads := NewStudentAnswers(answerFixture())
	yes, err := reads.StudentsWithAnswer(context.Background(), "seepferdchen", "true")
	require.NoError(t, err)
	require.Equal(t, []int64{11, 12}, yes, "the older no of child 11 no longer counts")
	no, err := reads.StudentsWithAnswer(context.Background(), "seepferdchen", "false")
	require.NoError(t, err)
	require.Equal(t, []int64{13}, no)
	none, err := reads.StudentsWithAnswer(context.Background(), "health", "Asthma")
	require.NoError(t, err)
	require.Empty(t, none, "a targeted question is read on the child record, not here")
	_, err = reads.StudentsWithAnswer(context.Background(), "seepferdchen", "")
	require.Error(t, err)
}

func TestSearchStudentAnswersMatchesStatedAnswersOnly(t *testing.T) {
	t.Parallel()
	reads := NewStudentAnswers(answerFixture())
	matches, err := reads.SearchStudentAnswers(context.Background(), "seepferd", 10)
	require.NoError(t, err)
	require.Equal(t, []enrollment.StudentAnswerMatch{
		{StudentID: 11, Label: "Seepferdchen vorhanden", Answer: "Ja"},
		{StudentID: 12, Label: "Seepferdchen vorhanden", Answer: "Ja"},
	}, matches, "a no names the question but says nothing")

	byAnswer, err := reads.SearchStudentAnswers(context.Background(), "nusse", 10)
	require.NoError(t, err)
	require.Equal(t, []enrollment.StudentAnswerMatch{{StudentID: 11, Label: "Allergien", Answer: "Nüsse"}}, byAnswer, "folded like a name search")

	byOption, err := reads.SearchStudentAnswers(context.Background(), "Fußball", 10)
	require.NoError(t, err)
	require.Equal(t, []enrollment.StudentAnswerMatch{{StudentID: 11, Label: "Wunsch-AG", Answer: "Fußball"}}, byOption)

	byDate, err := reads.SearchStudentAnswers(context.Background(), "01.06.2025", 10)
	require.NoError(t, err)
	require.Equal(t, []enrollment.StudentAnswerMatch{{StudentID: 11, Label: "Schwimmt seit", Answer: "01.06.2025"}}, byDate)

	for _, hidden := range []string{"Asthma", "Ärztin"} {
		found, err := reads.SearchStudentAnswers(context.Background(), hidden, 10)
		require.NoError(t, err)
		require.Empty(t, found, hidden)
	}
	limited, err := reads.SearchStudentAnswers(context.Background(), "seepferd", 1)
	require.NoError(t, err)
	require.Len(t, limited, 1)
}

func TestStudentAnswersFailOnAnUnreadableFormVersion(t *testing.T) {
	t.Parallel()
	records := answerFixture()
	records.err = errors.New("broken version")
	_, err := NewStudentAnswers(records).SearchStudentAnswers(context.Background(), "seepferd", 10)
	require.ErrorContains(t, err, "broken version")
}
