package students_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	baseModel "github.com/moto-nrw/project-phoenix/models/base"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type fakeEnrollmentDecisionService struct {
	enrollmentService.DecisionService
	summaries []*enrollmentService.RequestSummary
	studentID int64
}

func (f *fakeEnrollmentDecisionService) ListByStudent(_ context.Context, studentID int64) ([]*enrollmentService.RequestSummary, error) {
	f.studentID = studentID
	return f.summaries, nil
}

type fakeEnrollmentFormSchemaService struct {
	enrollmentService.FormSchemaService
	schemas map[int64]*enrollmentModels.FormSchema
	err     error
}

func (f *fakeEnrollmentFormSchemaService) GetByID(_ context.Context, id int64) (*enrollmentModels.FormSchema, error) {
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
		summaries: []*enrollmentService.RequestSummary{
			{
				Request: &enrollmentModels.Request{
					Model:       baseModel.Model{ID: 77},
					SchemaID:    &schemaID,
					PhaseID:     5,
					SubmittedAt: submittedAt,
					CustomData: map[string]any{
						"guardian_question": "must not leak",
					},
				},
				Phase: &enrollmentModels.Phase{
					Model: baseModel.Model{ID: 5},
					Name:  "Anmeldung 2026",
				},
				Children: []*enrollmentModels.RequestChild{
					{
						Model:            baseModel.Model{ID: 701},
						CreatedStudentID: &linkedStudentID,
						CustomData: map[string]any{
							"swimming_level": "safe",
							"pickup_note":    "Oma darf abholen",
							"empty":          nil,
						},
					},
					{
						Model:            baseModel.Model{ID: 702},
						CreatedStudentID: &otherStudentID,
						CustomData: map[string]any{
							"swimming_level": "other child",
						},
					},
				},
			},
		},
	}
	tc.resource.EnrollmentDecision = decision
	tc.resource.EnrollmentFormSchema = &fakeEnrollmentFormSchemaService{
		schemas: map[int64]*enrollmentModels.FormSchema{
			schemaID: {
				Model: baseModel.Model{ID: schemaID},
				Fields: []enrollmentModels.FormField{
					{
						Key:         "guardian_question",
						Label:       "Elternfrage",
						Type:        enrollmentModels.FormFieldText,
						AppliesToCh: false,
					},
					{
						Key:         "swimming_level",
						Label:       "Schwimmfähigkeit",
						Type:        enrollmentModels.FormFieldSelect,
						AppliesToCh: true,
						Options: []enrollmentModels.FormFieldOption{
							{Label: "Kann sicher schwimmen", Value: "safe"},
						},
					},
					{
						Key:         "pickup_note",
						Label:       "Abholhinweis",
						Type:        enrollmentModels.FormFieldTextarea,
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
		summaries: []*enrollmentService.RequestSummary{
			{
				Request: &enrollmentModels.Request{
					Model:    baseModel.Model{ID: 77},
					SchemaID: &schemaID,
				},
				Children: []*enrollmentModels.RequestChild{
					{
						CreatedStudentID: &linkedStudentID,
						CustomData: map[string]any{
							"swimming_level": "safe",
						},
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
		summaries: []*enrollmentService.RequestSummary{
			{
				Request: &enrollmentModels.Request{
					Model:    baseModel.Model{ID: 77},
					SchemaID: &schemaID,
				},
				Children: []*enrollmentModels.RequestChild{
					{
						CreatedStudentID: &linkedStudentID,
						CustomData: map[string]any{
							"swimming_level": "safe",
						},
					},
				},
			},
		},
	}
	tc.resource.EnrollmentFormSchema = &fakeEnrollmentFormSchemaService{
		schemas: map[int64]*enrollmentModels.FormSchema{},
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
