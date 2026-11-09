package timetablehttp

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTemplateUpdateRejectsPredecessorNotePropagationWithOtherEdit(t *testing.T) {
	t.Parallel()

	s := buildTemplateModule(t, &mockMaterializationService{})
	defer s.cleanupFn()
	router := templateRouter(s.ctx, s.res, s.db)

	const name = "Frühbetreuung Wochennotiz"
	createdBody := createTemplateBody(s, name)
	createdBody["notes"] = "Alt"
	createdResponse := doTemplateJSON(t, router, http.MethodPost, "/templates", createdBody)
	require.Equal(t, http.StatusCreated, createdResponse.Code, "body=%s", createdResponse.Body.String())
	created := decodeTemplateData[createTemplateResponse](t, createdResponse)

	malicious := createTemplateBody(s, "Andere Frühbetreuung")
	malicious["notes"] = "Neu"
	malicious["propagate_notes_to_predecessors"] = true
	rejected := doTemplateJSON(t, router, http.MethodPut, fmt.Sprintf("/templates/%d", created.TemplateID), malicious)
	assert.Equal(t, http.StatusBadRequest, rejected.Code, "body=%s", rejected.Body.String())

	valid := createTemplateBody(s, name)
	valid["notes"] = "Neu"
	valid["propagate_notes_to_predecessors"] = true
	updated := doTemplateJSON(t, router, http.MethodPut, fmt.Sprintf("/templates/%d", created.TemplateID), valid)
	require.Equal(t, http.StatusOK, updated.Code, "body=%s", updated.Body.String())
	assert.Equal(t, name, decodeTemplateData[templateResponse](t, updated).Name)
}

func TestValidateNotesOnlyPredecessorPropagation(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name        string
		change      func(*parsedUpdateTemplate)
		wantAllowed bool
	}{
		{name: "changed note only", wantAllowed: true},
		{
			name: "cleared note only",
			change: func(parsed *parsedUpdateTemplate) {
				parsed.req.Notes = nil
			},
			wantAllowed: true,
		},
		{
			name: "changed title",
			change: func(parsed *parsedUpdateTemplate) {
				parsed.req.Name = "Andere Frühbetreuung"
			},
		},
		{
			name: "changed roster",
			change: func(parsed *parsedUpdateTemplate) {
				parsed.req.StudentIDs = []int64{101}
			},
		},
		{
			name: "changed recurrence",
			change: func(parsed *parsedUpdateTemplate) {
				parsed.weekPattern = 1
			},
		},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			stored := notesPropagationStoredTemplate()
			parsed := notesPropagationParsedUpdate(stored)
			if testCase.change != nil {
				testCase.change(parsed)
			}

			response := httptest.NewRecorder()
			allowed := validateNotesOnlyPredecessorPropagation(
				response,
				httptest.NewRequest(http.MethodPut, "/templates/7", nil),
				parsed,
				stored,
			)

			assert.Equal(t, testCase.wantAllowed, allowed)
			if !testCase.wantAllowed {
				assert.Equal(t, http.StatusBadRequest, response.Code)
			}
		})
	}
}

func TestNotesOnlyTemplateUpdateUsesSchedulePeriod(t *testing.T) {
	t.Parallel()

	stored := notesPropagationStoredTemplate()
	stored.CalendarPeriodID = nil
	parsed := notesPropagationParsedUpdate(stored)
	parsed.req.CalendarPeriodID = stored.Schedules[0].CalendarPeriodID

	assert.True(t, notesOnlyTemplateUpdate(parsed, stored))
}

func TestNotesOnlyTemplateUpdateUsesTemplatePeriodFallbackForSchedule(t *testing.T) {
	t.Parallel()

	stored := notesPropagationStoredTemplate()
	stored.Schedules[0].CalendarPeriodID = nil
	parsed := notesPropagationParsedUpdate(stored)

	assert.True(t, notesOnlyTemplateUpdate(parsed, stored))
}

func notesPropagationStoredTemplate() templateResponse {
	return templateResponse{
		Name:                  "Frühbetreuung",
		Type:                  "care",
		CategoryID:            2,
		PlanningTrackID:       notesPropagationInt64(3),
		RoomID:                notesPropagationInt64(4),
		EducationGroupID:      notesPropagationInt64(5),
		MaxParticipants:       notesPropagationInt(20),
		CalendarPeriodID:      notesPropagationInt64(6),
		TargetGroupType:       "none",
		ListKind:              notesPropagationString("anwesenheit"),
		Notes:                 notesPropagationString("Alt"),
		IncludeClosingDays:    true,
		RequiredStaffOverride: notesPropagationInt(2),
		StudentIDs:            []int64{101, 102},
		StaffIDs:              []int64{201},
		PrimaryStaffID:        notesPropagationInt64(201),
		Schedules: []templateScheduleResponse{{
			Weekday:          1,
			StartTime:        "14:00",
			EndTime:          "15:00",
			WeekPattern:      0,
			CalendarPeriodID: notesPropagationInt64(6),
		}},
	}
}

func notesPropagationParsedUpdate(stored templateResponse) *parsedUpdateTemplate {
	return &parsedUpdateTemplate{
		req: &updateTemplateRequest{
			Name:                         stored.Name,
			Type:                         stored.Type,
			Weekdays:                     []int{1},
			RoomID:                       *stored.RoomID,
			CategoryID:                   stored.CategoryID,
			PlanningTrackID:              nullableInt64{Set: true, Value: stored.PlanningTrackID},
			MaxParticipants:              nullableInt{Set: true, Value: stored.MaxParticipants},
			RequiredStaff:                stored.RequiredStaffOverride,
			CalendarPeriodID:             stored.CalendarPeriodID,
			EducationGroupID:             stored.EducationGroupID,
			TargetGroupType:              stored.TargetGroupType,
			ListKind:                     stored.ListKind,
			Notes:                        notesPropagationString("Neu"),
			IncludeClosingDays:           notesPropagationBool(stored.IncludeClosingDays),
			StudentIDs:                   stored.StudentIDs,
			StaffIDs:                     stored.StaffIDs,
			PrimaryStaffID:               stored.PrimaryStaffID,
			PropagateNotesToPredecessors: true,
		},
		startTime:   time.Date(2000, 1, 1, 14, 0, 0, 0, time.UTC),
		endTime:     time.Date(2000, 1, 1, 15, 0, 0, 0, time.UTC),
		weekPattern: 0,
	}
}

func notesPropagationInt64(value int64) *int64 { return &value }
func notesPropagationInt(value int) *int       { return &value }
func notesPropagationBool(value bool) *bool    { return &value }
func notesPropagationString(value string) *string {
	return &value
}
