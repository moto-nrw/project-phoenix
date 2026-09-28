package students_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"testing"
	"time"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type fakeEnrollmentDecisionService struct {
	capability.Decisions
	summaries []*capability.DecisionSummary
	studentID int64
}

func (f *fakeEnrollmentDecisionService) StudentDecisionRequests(_ context.Context, studentID int64) ([]*capability.DecisionSummary, error) {
	f.studentID = studentID
	return f.summaries, nil
}

type fakeEnrollmentFormSchemaService struct {
	capability.FormSchemaAdministration
	schemas map[int64]*capability.FormSchema
	err     error
}

func (f *fakeEnrollmentFormSchemaService) SchemaVersion(_ context.Context, id int64) (*capability.FormSchema, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.schemas[id], nil
}

func TestGetStudentEnrollmentExtraFields_ReturnsOnlyLinkedChildFields(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)
	student := testpkg.CreateTestStudent(t, tc.db, "Extra", "Fields", "3a")

	linkedStudentID := student.ID
	otherStudentID := student.ID + 999
	schemaID := int64(42)
	submittedAt := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	decision := &fakeEnrollmentDecisionService{
		summaries: []*capability.DecisionSummary{
			{
				Request: &capability.Request{
					ID:          77,
					SchemaID:    &schemaID,
					PhaseID:     5,
					SubmittedAt: submittedAt,
					CustomData: rawEnrollmentJSON(t, map[string]any{
						"guardian_question": "must not leak",
					}),
				},
				Phase: &capability.Phase{
					ID:   5,
					Name: "Anmeldung 2026",
				},
				Children: []*capability.RequestChild{
					{
						ID:               701,
						CreatedStudentID: &linkedStudentID,
						CustomData: rawEnrollmentJSON(t, map[string]any{
							"swimming_level": "safe",
							"pickup_note":    "Oma darf abholen",
							"empty":          nil,
						}),
					},
					{
						ID:               702,
						CreatedStudentID: &otherStudentID,
						CustomData: rawEnrollmentJSON(t, map[string]any{
							"swimming_level": "other child",
						}),
					},
				},
			},
		},
	}
	tc.resource.EnrollmentDecision = decision
	tc.resource.EnrollmentFormSchema = &fakeEnrollmentFormSchemaService{
		schemas: map[int64]*capability.FormSchema{
			schemaID: {
				ID: schemaID,
				Fields: []capability.FormField{
					{
						Key:         "guardian_question",
						Label:       "Elternfrage",
						Type:        capability.FormFieldText,
						AppliesToCh: false,
					},
					{
						Key:         "swimming_level",
						Label:       "Schwimmfähigkeit",
						Type:        capability.FormFieldSelect,
						AppliesToCh: true,
						Options: []capability.FormFieldOption{
							{Label: "Kann sicher schwimmen", Value: "safe"},
						},
					},
					{
						Key:         "pickup_note",
						Label:       "Abholhinweis",
						Type:        capability.FormFieldTextarea,
						AppliesToCh: true,
					},
				},
			},
		},
	}

	req := testutil.NewRequest("GET", "/"+strconv.FormatInt(student.ID, 10)+"/enrollment-extra-fields", nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

	require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())
	assert.Equal(t, student.ID, decision.studentID)

	var body struct {
		Data []struct {
			RequestID string `json:"request_id"`
			PhaseName string `json:"phase_name"`
			Fields    []struct {
				Key   string `json:"key"`
				Label string `json:"label"`
				Value any    `json:"value"`
			} `json:"fields"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	require.Len(t, body.Data, 1)
	assert.Equal(t, "77", body.Data[0].RequestID)
	assert.Equal(t, "Anmeldung 2026", body.Data[0].PhaseName)
	require.Len(t, body.Data[0].Fields, 2)
	assert.Equal(t, "swimming_level", body.Data[0].Fields[0].Key)
	assert.Equal(t, "Schwimmfähigkeit", body.Data[0].Fields[0].Label)
	assert.Equal(t, "safe", body.Data[0].Fields[0].Value)
	assert.Equal(t, "pickup_note", body.Data[0].Fields[1].Key)
}

func TestGetStudentEnrollmentExtraFields_EmptyWhenNoLinkedAnswers(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)
	student := testpkg.CreateTestStudent(t, tc.db, "No", "Extras", "3a")

	tc.resource.EnrollmentDecision = &fakeEnrollmentDecisionService{}
	tc.resource.EnrollmentFormSchema = &fakeEnrollmentFormSchemaService{}

	req := testutil.NewRequest("GET", "/"+strconv.FormatInt(student.ID, 10)+"/enrollment-extra-fields", nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

	require.Equal(t, http.StatusOK, rr.Code, "body: %s", rr.Body.String())
	var body struct {
		Data []any `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Empty(t, body.Data)
}

func TestGetStudentEnrollmentExtraFields_FailsWhenSchemaLookupFails(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)
	student := testpkg.CreateTestStudent(t, tc.db, "Schema", "Failure", "3a")

	linkedStudentID := student.ID
	schemaID := int64(42)
	tc.resource.EnrollmentDecision = &fakeEnrollmentDecisionService{
		summaries: []*capability.DecisionSummary{
			{
				Request: &capability.Request{
					ID:       77,
					SchemaID: &schemaID,
				},
				Children: []*capability.RequestChild{
					{
						CreatedStudentID: &linkedStudentID,
						CustomData: rawEnrollmentJSON(t, map[string]any{
							"swimming_level": "safe",
						}),
					},
				},
			},
		},
	}
	tc.resource.EnrollmentFormSchema = &fakeEnrollmentFormSchemaService{
		err: errors.New("schema repository unavailable"),
	}

	req := testutil.NewRequest("GET", "/"+strconv.FormatInt(student.ID, 10)+"/enrollment-extra-fields", nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

	require.Equal(t, http.StatusInternalServerError, rr.Code, "body: %s", rr.Body.String())
	var body struct {
		Error string `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, "failed to load enrollment extra fields", body.Error)
	assert.NotContains(t, rr.Body.String(), "schema repository unavailable")
}

func TestGetStudentEnrollmentExtraFields_FailsWhenSchemaIsMissing(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)
	student := testpkg.CreateTestStudent(t, tc.db, "Schema", "Missing", "3a")

	linkedStudentID := student.ID
	schemaID := int64(42)
	tc.resource.EnrollmentDecision = &fakeEnrollmentDecisionService{
		summaries: []*capability.DecisionSummary{
			{
				Request: &capability.Request{
					ID:       77,
					SchemaID: &schemaID,
				},
				Children: []*capability.RequestChild{
					{
						CreatedStudentID: &linkedStudentID,
						CustomData: rawEnrollmentJSON(t, map[string]any{
							"swimming_level": "safe",
						}),
					},
				},
			},
		},
	}
	tc.resource.EnrollmentFormSchema = &fakeEnrollmentFormSchemaService{
		schemas: map[int64]*capability.FormSchema{},
	}

	req := testutil.NewRequest("GET", "/"+strconv.FormatInt(student.ID, 10)+"/enrollment-extra-fields", nil)
	rr := authExec(t, tc, req, testutil.AdminTestClaims(1), []string{"admin:*"})

	require.Equal(t, http.StatusInternalServerError, rr.Code, "body: %s", rr.Body.String())
	var body struct {
		Error string `json:"error"`
	}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	assert.Equal(t, "failed to load enrollment extra fields", body.Error)
	assert.NotContains(t, rr.Body.String(), "enrollment schema 42 not found")
}

// rawEnrollmentJSON encodes custom answers the way the Enrollment owner hands
// them over: as raw JSON.
func rawEnrollmentJSON(t *testing.T, value map[string]any) json.RawMessage {
	t.Helper()
	raw, err := json.Marshal(value)
	require.NoError(t, err)
	return raw
}
