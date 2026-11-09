package carecontracthttp

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
)

// The paper copy keeps the name it was uploaded under; a quote in it must
// not end the filename parameter and smuggle in a second one.
func TestWriteFile_EscapesHostileFilename(t *testing.T) {
	t.Parallel()

	rec := httptest.NewRecorder()
	WriteFile(rec, carecontract.File{
		Filename: `x"; filename*=UTF-8''x.html; a=".pdf`, ContentType: "application/pdf", Data: []byte("%PDF-1.4"),
	}, "attachment")

	assert.Equal(t, `attachment; filename="x_; filename*=UTF-8''x.html; a=_.pdf"; filename*=UTF-8''x%22%3B%20filename%2A%3DUTF-8%27%27x.html%3B%20a%3D%22.pdf`,
		rec.Header().Get("Content-Disposition"))
}

// The application decides whether a refusal concerns fields or the
// selection; the code follows that kind, not the field names (#6106).
func TestRenderError_CodeFollowsValidationKind(t *testing.T) {
	t.Parallel()

	field := func(name, reason string) *carecontract.ValidationError {
		return &carecontract.ValidationError{Fields: map[string]string{name: reason}}
	}
	selection := func(name, reason string) *carecontract.ValidationError {
		return &carecontract.ValidationError{Fields: map[string]string{name: reason}, Selection: true}
	}
	cases := []struct {
		name   string
		render func(http.ResponseWriter, *http.Request, error)
		err    error
		code   string
	}{
		{"notice received in the future", renderNoticeError, field("received_on", "out_of_range"), "care_contract.field_invalid"},
		{"paper signing date", renderError, field("signed_on", "invalid"), "care_contract.field_invalid"},
		{"billing period", renderError, field("period", "invalid"), "care_contract.field_invalid"},
		{"resend address", renderError, field("recipient_email", "invalid"), "care_contract.field_invalid"},
		{"template fields", renderError, field("term_months", "required"), "care_contract.field_invalid"},
		{"too many contracts", renderError, selection("contracts", "count"), "care_contract.selection_invalid"},
		{"no contract to send", renderError, selection("contract_ids", "count"), "care_contract.selection_invalid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			rec := httptest.NewRecorder()
			tc.render(rec, httptest.NewRequest(http.MethodPost, "/", nil), tc.err)

			require.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			var body struct {
				Code   string `json:"code"`
				Errors []struct {
					Field string `json:"field"`
				} `json:"errors"`
			}
			require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
			assert.Equal(t, tc.code, body.Code)
			require.Len(t, body.Errors, 1, "the refused field stays marked")
		})
	}
}
