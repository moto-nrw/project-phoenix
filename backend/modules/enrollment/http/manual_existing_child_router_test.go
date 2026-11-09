// The manual enrollment of a child the school already cares for (#6148):
// the refusal names the candidates, and the office's choice renews the
// enrolled child or creates a new one, like the approval in the detail view
// (#6053).
package enrollmenthttp_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentAPI "github.com/moto-nrw/project-phoenix/modules/enrollment/http"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func manualPaperChild() enrollmentAPI.SubmitChildRequest {
	grade := int16(1)
	return enrollmentAPI.SubmitChildRequest{
		FirstName: "Mara", LastName: "Papier", DateOfBirth: "2019-03-01", TargetGradeLevel: &grade,
	}
}

type manualEnrollmentCreated struct {
	Data struct {
		StudentID string `json:"student_id"`
		Status    string `json:"status"`
	} `json:"data"`
}

func requireManualEnrollmentCreated(t *testing.T, rec *httptest.ResponseRecorder) manualEnrollmentCreated {
	t.Helper()
	require.Equal(t, http.StatusCreated, rec.Code, "Body: %s", rec.Body.String())
	var body manualEnrollmentCreated
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "approved", body.Data.Status)
	return body
}

func TestCreateManualApprovedEnrollment_RefusalNamesTheCandidates(t *testing.T) {
	t.Parallel()
	m := setupManualEnrollmentRouter(t)
	existing := enrolledChild(t, m.db, "Mara", "Papier", calendar.NewDate(2019, 3, 1))

	rec := m.postManualEnrollment(t, manualPaperChild())

	require.Equal(t, http.StatusConflict, rec.Code, "Body: %s", rec.Body.String())
	var body struct {
		Code    string `json:"code"`
		Details struct {
			Candidates []enrollmentAPI.AdminExistingChildCandidate `json:"candidates"`
		} `json:"details"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	assert.Equal(t, "enrollment.child_possibly_enrolled", body.Code)
	require.Len(t, body.Details.Candidates, 1)
	candidate := body.Details.Candidates[0]
	assert.Equal(t, strconv.FormatInt(existing.ID, 10), candidate.StudentID)
	assert.Equal(t, "Mara", candidate.FirstName)
	assert.Equal(t, "Papier", candidate.LastName)
	assert.Equal(t, "1a", candidate.SchoolClass)
	require.NotNil(t, candidate.DateOfBirth)
	assert.Equal(t, "2019-03-01", *candidate.DateOfBirth)
}

func TestCreateManualApprovedEnrollment_OfficeRenewsTheEnrolledChild(t *testing.T) {
	t.Parallel()
	m := setupManualEnrollmentRouter(t)
	existing := enrolledChild(t, m.db, "Mara", "Papier", calendar.NewDate(2019, 3, 1))

	rec := m.postManualEnrollmentChoosing(t, manualPaperChild(), strconv.FormatInt(existing.ID, 10), false)

	body := requireManualEnrollmentCreated(t, rec)
	assert.Equal(t, strconv.FormatInt(existing.ID, 10), body.Data.StudentID, "the enrollment renews the chosen child")
	assert.Equal(t, 1, studentsNamed(t, m.db, "Mara", "Papier"), "no second child is created")
	var links int
	require.NoError(t, m.db.NewSelect().TableExpr("users.student_guardian_relationships").ColumnExpr("COUNT(*)").
		Where("student_id = ?", existing.ID).Scan(m.ctx, &links))
	assert.Zero(t, links, "the office's choice links none of the entered guardians")
}

func TestCreateManualApprovedEnrollment_OfficeCreatesANewChild(t *testing.T) {
	t.Parallel()
	m := setupManualEnrollmentRouter(t)
	existing := enrolledChild(t, m.db, "Mara", "Papier", calendar.NewDate(2019, 3, 1))

	rec := m.postManualEnrollmentChoosing(t, manualPaperChild(), "", true)

	body := requireManualEnrollmentCreated(t, rec)
	assert.NotEqual(t, strconv.FormatInt(existing.ID, 10), body.Data.StudentID)
	assert.Equal(t, 2, studentsNamed(t, m.db, "Mara", "Papier"))
}

func TestCreateManualApprovedEnrollment_RefusesAChoiceThatIsNoCandidate(t *testing.T) {
	t.Parallel()
	m := setupManualEnrollmentRouter(t)
	enrolledChild(t, m.db, "Mara", "Papier", calendar.NewDate(2019, 3, 1))
	unrelated := enrolledChild(t, m.db, "Paula", "Fremd", calendar.NewDate(2017, 1, 1))

	rec := m.postManualEnrollmentChoosing(t, manualPaperChild(), strconv.FormatInt(unrelated.ID, 10), false)

	require.Equal(t, http.StatusConflict, rec.Code, "Body: %s", rec.Body.String())
	assertDecisionErrorCode(t, rec, "enrollment.existing_child_choice_invalid")
	assert.Equal(t, 1, studentsNamed(t, m.db, "Mara", "Papier"))
}

func TestCreateManualApprovedEnrollment_RefusesBothChoicesAtOnce(t *testing.T) {
	t.Parallel()
	m := setupManualEnrollmentRouter(t)
	existing := enrolledChild(t, m.db, "Mara", "Papier", calendar.NewDate(2019, 3, 1))

	rec := m.postManualEnrollmentChoosing(t, manualPaperChild(), strconv.FormatInt(existing.ID, 10), true)

	require.Equal(t, http.StatusBadRequest, rec.Code, "Body: %s", rec.Body.String())
	leftRequests, err := m.db.NewSelect().TableExpr("enrollment.requests").
		Where("tenant_id = ?", testpkg.Tenant(t)).Where("phase_id = ?", m.phase.ID).Count(m.ctx)
	require.NoError(t, err)
	assert.Zero(t, leftRequests)
}

// Without a candidate the manual enrollment stays as it was: no choice is
// needed, and the child is created.
func TestCreateManualApprovedEnrollment_WithoutCandidateNeedsNoChoice(t *testing.T) {
	t.Parallel()
	m := setupManualEnrollmentRouter(t)

	rec := m.postManualEnrollment(t, manualPaperChild())

	body := requireManualEnrollmentCreated(t, rec)
	assert.NotEmpty(t, body.Data.StudentID)
	assert.Equal(t, 1, studentsNamed(t, m.db, "Mara", "Papier"))
}
