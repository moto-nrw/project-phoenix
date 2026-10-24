// Router-level tests for the homework of moto schule (#4796): a Lehrkraft
// enters homework for an assigned class, changes and deletes every entry of
// that class, whoever entered it, and reads the OGS feedback per child in the
// class view.
package classdayhttp_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	rootAPI "github.com/moto-nrw/project-phoenix/api"
	"github.com/moto-nrw/project-phoenix/api/testutil"
	classdayhttp "github.com/moto-nrw/project-phoenix/modules/classday/http"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/sessiontoken"
	"github.com/moto-nrw/project-phoenix/tenant"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

const (
	homeworkPermission = "class_day:homework_write"
	homeworkDay        = "2099-03-02"
)

type homeworkFixture struct {
	db        *testpkg.DB
	router    chi.Router
	tenantID  int64
	class     string
	child     int64
	lehrkraft sessiontoken.AppClaims
	staffID   int64
	colleague sessiontoken.AppClaims
}

// setupHomeworkFixture builds two Lehrkräfte assigned to the same class, a
// child of that class, and a class nobody of them teaches.
func setupHomeworkFixture(t *testing.T) *homeworkFixture {
	t.Helper()
	db, module := setupClassDayModule(t)
	router := classdayhttp.NewResource(module.ClassDay, nil).SchoolRouter(rootAPI.ProtectedSchoolRoutes)
	tenantID := testpkg.Tenant(t)
	suffix := time.Now().UnixNano()
	className := fmt.Sprintf("hw%d", suffix%100000)
	claims := func(first string) (sessiontoken.AppClaims, int64) {
		staff, account := testpkg.CreateTestStaffWithAccount(t, db, first, fmt.Sprintf("Lehrkraft-%d", suffix))
		_ = testpkg.CreateTestClassTeacher(t, db, staff.ID, className)
		return sessiontoken.AppClaims{ID: int(account.ID), Sub: account.Email, Roles: []string{"lehrkraft"}, TenantID: tenantID, Scope: tenant.ScopeSchool}, staff.ID
	}
	child := testpkg.CreateTestStudent(t, db, "Hanna", "Aufgabe", className)
	lehrkraft, staffID := claims("Anna")
	colleague, _ := claims("Bert")
	return &homeworkFixture{
		db: db, router: router, tenantID: tenantID, class: className, child: child.ID,
		lehrkraft: lehrkraft, staffID: staffID, colleague: colleague,
	}
}

func (f *homeworkFixture) do(t *testing.T, claims sessiontoken.AppClaims, method, path, body string, perms ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	return testutil.ExecuteWithAuthPermissions(t, f.router, req, claims, perms)
}

type homeworkReport struct {
	CanWrite bool `json:"can_write_homework"`
	Homework []struct {
		ID          string `json:"id"`
		Subject     string `json:"subject"`
		Description string `json:"description"`
	} `json:"homework"`
	Rows []struct {
		StudentID int64 `json:"student_id"`
		Feedback  *struct {
			Status string `json:"status"`
			Note   string `json:"note"`
		} `json:"homework_feedback"`
	} `json:"rows"`
}

func (f *homeworkFixture) report(t *testing.T, claims sessiontoken.AppClaims, perms ...string) homeworkReport {
	t.Helper()
	rec := f.do(t, claims, http.MethodGet, "/?class="+f.class+"&date="+homeworkDay, "", perms...)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var envelope struct {
		Data homeworkReport `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	return envelope.Data
}

func (f *homeworkFixture) add(t *testing.T, claims sessiontoken.AppClaims, body string) string {
	t.Helper()
	rec := f.do(t, claims, http.MethodPost, "/homework/"+f.class, body, homeworkPermission)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
	var envelope struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &envelope))
	require.NotEmpty(t, envelope.Data.ID)
	return envelope.Data.ID
}

func TestHomeworkRoundTrip(t *testing.T) {
	t.Parallel()
	f := setupHomeworkFixture(t)

	id := f.add(t, f.lehrkraft, `{"date":"`+homeworkDay+`","subject":" Mathe ","description":"Seite 12, Nummer 3"}`)
	_ = f.add(t, f.lehrkraft, `{"date":"`+homeworkDay+`","description":"Lesen"}`)

	report := f.report(t, f.lehrkraft, classDayReadPermission, homeworkPermission)
	assert.True(t, report.CanWrite)
	require.Len(t, report.Homework, 2, "several entries per class and day")
	assert.Equal(t, "Mathe", report.Homework[0].Subject, "the subject is trimmed")
	assert.Equal(t, "Seite 12, Nummer 3", report.Homework[0].Description)
	assert.Empty(t, report.Homework[1].Subject, "the subject is optional")

	rec := f.do(t, f.lehrkraft, http.MethodPut, "/homework/"+f.class+"/"+id, `{"subject":"Deutsch","description":"Diktat üben"}`, homeworkPermission)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	report = f.report(t, f.lehrkraft, classDayReadPermission)
	assert.False(t, report.CanWrite, "the verdict needs the permission")
	assert.Equal(t, "Diktat üben", report.Homework[0].Description)

	rec = f.do(t, f.lehrkraft, http.MethodDelete, "/homework/"+f.class+"/"+id, "", homeworkPermission)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	report = f.report(t, f.lehrkraft, classDayReadPermission)
	require.Len(t, report.Homework, 1)
	assert.Equal(t, "Lesen", report.Homework[0].Description)
}

func TestHomeworkScope(t *testing.T) {
	t.Parallel()
	f := setupHomeworkFixture(t)
	id := f.add(t, f.lehrkraft, `{"date":"`+homeworkDay+`","description":"Lesen"}`)

	t.Run("another teacher's class is refused", func(t *testing.T) {
		other := f.class + "x"
		_ = testpkg.CreateTestStudent(t, f.db, "Fremd", "Kind", other)
		rec := f.do(t, f.lehrkraft, http.MethodPost, "/homework/"+other, `{"date":"`+homeworkDay+`","description":"Lesen"}`, homeworkPermission)
		require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
		rec = f.do(t, f.lehrkraft, http.MethodPut, "/homework/"+other+"/"+id, `{"description":"Lesen"}`, homeworkPermission)
		require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
		rec = f.do(t, f.lehrkraft, http.MethodDelete, "/homework/"+other+"/"+id, "", homeworkPermission)
		require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	})

	t.Run("a Lehrkraft of another class cannot reach the entry", func(t *testing.T) {
		outsiderClass := fmt.Sprintf("hy%d", time.Now().UnixNano()%100000)
		staff, account := testpkg.CreateTestStaffWithAccount(t, f.db, "Carla", fmt.Sprintf("Fremd-%d", time.Now().UnixNano()))
		_ = testpkg.CreateTestClassTeacher(t, f.db, staff.ID, outsiderClass)
		outsider := sessiontoken.AppClaims{ID: int(account.ID), Sub: account.Email, Roles: []string{"lehrkraft"}, TenantID: f.tenantID, Scope: tenant.ScopeSchool}
		rec := f.do(t, outsider, http.MethodPut, "/homework/"+f.class+"/"+id, `{"description":"fremd"}`, homeworkPermission)
		require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
		rec = f.do(t, outsider, http.MethodDelete, "/homework/"+f.class+"/"+id, "", homeworkPermission)
		require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
		rec = f.do(t, outsider, http.MethodPut, "/homework/"+outsiderClass+"/"+id, `{"description":"fremd"}`, homeworkPermission)
		require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
		assert.Contains(t, rec.Body.String(), "classday.homework_not_found")
		rec = f.do(t, outsider, http.MethodDelete, "/homework/"+outsiderClass+"/"+id, "", homeworkPermission)
		require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	})

	t.Run("the entry is reached only through its own class", func(t *testing.T) {
		second := fmt.Sprintf("hz%d", time.Now().UnixNano()%100000)
		_ = testpkg.CreateTestClassTeacher(t, f.db, f.staffID, second)
		rec := f.do(t, f.lehrkraft, http.MethodDelete, "/homework/"+second+"/"+id, "", homeworkPermission)
		require.Equal(t, http.StatusNotFound, rec.Code, rec.Body.String())
	})

	t.Run("without the permission nothing is written", func(t *testing.T) {
		rec := f.do(t, f.lehrkraft, http.MethodPost, "/homework/"+f.class, `{"date":"`+homeworkDay+`","description":"Lesen"}`, classDayReadPermission)
		require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
		rec = f.do(t, f.colleague, http.MethodPut, "/homework/"+f.class+"/"+id, `{"description":"fremd"}`, classDayReadPermission)
		require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
		rec = f.do(t, f.colleague, http.MethodDelete, "/homework/"+f.class+"/"+id, "", classDayReadPermission)
		require.Equal(t, http.StatusForbidden, rec.Code, rec.Body.String())
	})

	t.Run("invalid entries", func(t *testing.T) {
		for _, body := range []string{
			`{"date":"` + homeworkDay + `","description":"  "}`,
			`{"date":"2099-02-30","description":"Lesen"}`,
			`{"description":"Lesen"}`,
			`{"date":"` + homeworkDay + `","description":"` + strings.Repeat("a", 2001) + `"}`,
			`{"date":"` + homeworkDay + `","subject":"` + strings.Repeat("a", 61) + `","description":"Lesen"}`,
		} {
			rec := f.do(t, f.lehrkraft, http.MethodPost, "/homework/"+f.class, body, homeworkPermission)
			assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
		}
	})

	var description string
	var count int
	require.NoError(t, f.db.NewRaw(`SELECT COUNT(*), MAX(description) FROM education.class_homework WHERE tenant_id = ?`, f.tenantID).Scan(testpkg.Ctx(t), &count, &description))
	assert.Equal(t, 1, count, "no refused write stored or deleted anything")
	assert.Equal(t, "Lesen", description, "no refused write changed the entry")
}

// TestHomeworkColleagueOfTheClass: every Lehrkraft assigned to the class
// changes and deletes its entries, whoever entered them; the author stays on
// the row as provenance.
func TestHomeworkColleagueOfTheClass(t *testing.T) {
	t.Parallel()
	f := setupHomeworkFixture(t)
	id := f.add(t, f.lehrkraft, `{"date":"`+homeworkDay+`","description":"Lesen"}`)

	report := f.report(t, f.colleague, classDayReadPermission, homeworkPermission)
	assert.True(t, report.CanWrite, "the colleague may write the class's homework")
	require.Len(t, report.Homework, 1)

	rec := f.do(t, f.colleague, http.MethodPut, "/homework/"+f.class+"/"+id, `{"subject":"Deutsch","description":"Diktat üben"}`, homeworkPermission)
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	report = f.report(t, f.lehrkraft, classDayReadPermission)
	require.Len(t, report.Homework, 1)
	assert.Equal(t, "Deutsch", report.Homework[0].Subject)
	assert.Equal(t, "Diktat üben", report.Homework[0].Description)
	var author int64
	require.NoError(t, f.db.NewRaw(`SELECT created_by_staff_id FROM education.class_homework WHERE tenant_id = ? AND id = ?`, f.tenantID, id).Scan(testpkg.Ctx(t), &author))
	assert.Equal(t, f.staffID, author, "a change keeps the author")

	rec = f.do(t, f.colleague, http.MethodDelete, "/homework/"+f.class+"/"+id, "", homeworkPermission)
	require.Equal(t, http.StatusNoContent, rec.Code, rec.Body.String())
	assert.Empty(t, f.report(t, f.lehrkraft, classDayReadPermission).Homework)
}

func TestHomeworkFeedbackShowsInTheClassView(t *testing.T) {
	t.Parallel()
	f := setupHomeworkFixture(t)
	_ = f.add(t, f.lehrkraft, `{"date":"`+homeworkDay+`","description":"Lesen"}`)
	_, err := f.db.ExecContext(testpkg.Ctx(t), `INSERT INTO education.homework_feedback (tenant_id, student_id, date, status, note)
		VALUES (?, ?, ?, 'difficult', 'Aufgabe 3 war zu schwer')`, f.tenantID, f.child, homeworkDay)
	require.NoError(t, err)

	report := f.report(t, f.lehrkraft, classDayReadPermission)
	var found bool
	for _, row := range report.Rows {
		if row.StudentID == f.child {
			found = true
			require.NotNil(t, row.Feedback)
			assert.Equal(t, "difficult", row.Feedback.Status)
			assert.Equal(t, "Aufgabe 3 war zu schwer", row.Feedback.Note)
		}
	}
	assert.True(t, found, "the child is on the class view")
}
