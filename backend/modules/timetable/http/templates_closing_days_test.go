package timetablehttp

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A series used for holiday care opts into closing days (#3594). The flag
// travels through create, the list read and update; an update that omits it
// keeps the stored value.
func TestTemplateIncludeClosingDaysRoundTrip(t *testing.T) {
	t.Parallel()

	s := buildTemplateModule(t, &mockMaterializationService{})
	defer s.cleanupFn()
	router := templateRouter(s.ctx, s.res)

	body := createTemplateBody(s, "Tpl-Ferienbetreuung")
	body["include_closing_days"] = true
	w := doTemplateJSON(t, router, http.MethodPost, "/templates", body)
	require.Equal(t, http.StatusCreated, w.Code, "body=%s", w.Body.String())
	created := decodeTemplateData[createTemplateResponse](t, w)

	listed := func() templateResponse {
		t.Helper()
		listW := doTemplateJSON(t, router, http.MethodGet, "/templates", nil)
		require.Equal(t, http.StatusOK, listW.Code, "body=%s", listW.Body.String())
		for _, candidate := range decodeTemplateData[listTemplatesResponse](t, listW).Templates {
			if candidate.ID == created.TemplateID {
				return candidate
			}
		}
		t.Fatalf("template %d missing from list", created.TemplateID)
		return templateResponse{}
	}
	assert.True(t, listed().IncludeClosingDays)

	update := func(body map[string]any) templateResponse {
		t.Helper()
		updateW := doTemplateJSON(t, router, http.MethodPut, fmt.Sprintf("/templates/%d", created.TemplateID), body)
		require.Equal(t, http.StatusOK, updateW.Code, "body=%s", updateW.Body.String())
		return decodeTemplateData[templateResponse](t, updateW)
	}

	assert.True(t, update(createTemplateBody(s, "Tpl-Ferienbetreuung")).IncludeClosingDays, "omitted keeps the flag")

	off := createTemplateBody(s, "Tpl-Ferienbetreuung")
	off["include_closing_days"] = false
	assert.False(t, update(off).IncludeClosingDays)
	assert.False(t, listed().IncludeClosingDays)
}
