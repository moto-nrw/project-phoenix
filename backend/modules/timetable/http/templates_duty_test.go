package timetablehttp

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dutyTemplateBody is a duty (#3822): staff only, no room, no children.
func dutyTemplateBody(s *templateSetup, name string) map[string]any {
	body := createTemplateBody(s, name)
	body["type"] = timetable.GroupTypeDuty
	body["target_group_type"] = timetable.TargetGroupTypeNone
	delete(body, "room_id")
	delete(body, "student_ids")
	delete(body, "max_participants")
	body["staff_ids"] = []int64{s.staffA}
	body["primary_staff_id"] = s.staffA
	body["required_staff"] = 1
	return body
}

func TestTemplateDutyRoundTripWithoutRoom(t *testing.T) {
	t.Parallel()

	s := buildTemplateModule(t, &mockMaterializationService{})
	defer s.cleanupFn()
	router := templateRouter(s.ctx, s.res)

	w := doTemplateJSON(t, router, http.MethodPost, "/templates", dutyTemplateBody(s, "Tpl-Busaufsicht"))
	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())
	created := decodeTemplateData[createTemplateResponse](t, w)

	listW := doTemplateJSON(t, router, http.MethodGet, "/templates", nil)
	require.Equal(t, http.StatusOK, listW.Code, "body=%s", listW.Body.String())
	var stored templateResponse
	for _, candidate := range decodeTemplateData[listTemplatesResponse](t, listW).Templates {
		if candidate.ID == created.TemplateID {
			stored = candidate
		}
	}
	require.Equal(t, created.TemplateID, stored.ID, "duty missing from the list")
	assert.Equal(t, timetable.GroupTypeDuty, stored.Type)
	assert.Nil(t, stored.RoomID)

	// An update keeps it room-less; adding a room is fine as well.
	updateW := doTemplateJSON(t, router, http.MethodPut, fmt.Sprintf("/templates/%d", created.TemplateID), dutyTemplateBody(s, "Tpl-Busaufsicht"))
	require.Equal(t, http.StatusOK, updateW.Code, "body=%s", updateW.Body.String())
	withRoom := dutyTemplateBody(s, "Tpl-Busaufsicht")
	withRoom["room_id"] = s.roomID
	updateW = doTemplateJSON(t, router, http.MethodPut, fmt.Sprintf("/templates/%d", created.TemplateID), withRoom)
	require.Equal(t, http.StatusOK, updateW.Code, "body=%s", updateW.Body.String())
	assert.Equal(t, s.roomID, *decodeTemplateData[templateResponse](t, updateW).RoomID)
}

func TestTemplateDutyRejectsChildrenAndRoomlessCare(t *testing.T) {
	t.Parallel()

	s := buildTemplateModule(t, &mockMaterializationService{})
	defer s.cleanupFn()
	router := templateRouter(s.ctx, s.res)

	cases := map[string]func(body map[string]any){
		"children": func(body map[string]any) { body["student_ids"] = []int64{s.studentA} },
		"target group": func(body map[string]any) {
			body["target_group_type"] = timetable.TargetGroupTypeSchoolClass
			body["target_school_class"] = "3a"
		},
		"participant limit": func(body map[string]any) { body["max_participants"] = 10 },
		"list kind":         func(body map[string]any) { body["list_kind"] = timetable.ListKindMensa },
		"care without room": func(body map[string]any) { body["type"] = timetable.GroupTypeCare },
	}
	for name, mutate := range cases {
		body := dutyTemplateBody(s, "Tpl-Dienst-"+name)
		mutate(body)
		w := doTemplateJSON(t, router, http.MethodPost, "/templates", body)
		assert.Equal(t, http.StatusBadRequest, w.Code, "%s: body=%s", name, w.Body.String())
	}
}
