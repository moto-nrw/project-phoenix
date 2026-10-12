package compose_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/api/testutil"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// newIServRequest uploads a file with the raw "iserv_user_types" field of the
// import page; an empty field is left out.
func newIServRequest(t *testing.T, target, content, choice string, opts ...testutil.RequestOption) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	if choice != "" {
		require.NoError(t, writer.WriteField("iserv_user_types", choice))
	}
	fw, err := writer.CreateFormFile("file", "Benutzer_schule.example.csv")
	require.NoError(t, err)
	_, err = fw.Write([]byte(content))
	require.NoError(t, err)
	require.NoError(t, writer.Close())
	req := httptest.NewRequest(http.MethodPost, target, &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	for _, opt := range opts {
		opt(req)
	}
	return req
}

func iservChoice(t *testing.T, userTypes ...string) string {
	t.Helper()
	encoded, err := json.Marshal(append([]string{}, userTypes...))
	require.NoError(t, err)
	return string(encoded)
}

// iservUserType mirrors the wire shape of one IServ user type.
type iservUserType struct {
	Value    string `json:"value"`
	Count    int    `json:"count"`
	Selected bool   `json:"selected"`
}

type iservPreview struct {
	Data struct {
		TotalRows    int
		CreatedCount int
		IServ        *struct {
			UserTypes        []iservUserType `json:"user_types"`
			DeactivatedCount int             `json:"deactivated_count"`
		} `json:"iserv"`
	} `json:"data"`
}

// TestImportStudents_IServExport verifies the IServ user export end to end
// (#1446): the preview lists the user types and preselects pupils, the
// person's choice decides which rows are imported, and staff rows never
// become children.
func TestImportStudents_IServExport(t *testing.T) {
	t.Parallel()
	tc := setupImportRoute(t)
	_, account := testpkg.CreateTestTeacherWithAccount(t, tc.db, "IServ", "Admin")
	claims := adminBearer(t, account.ID)
	router := tc.resource.Router()

	unique := time.Now().UnixNano()
	export := fmt.Sprintf("Account;Vorname;Nachname;Status;Benutzertyp;Klasse/Information;Geburtsdatum;Gruppen\n"+
		"mia.k%[1]d;Mia;Kind%[1]d;aktiviert;Schüler(innen);1a;15.08.2019;Klasse 1a\n"+
		"lena.l%[1]d;Lena;Lehrkraft%[1]d;aktiviert;Lehrer(innen);LEH;;Lehrer\n"+
		"tom.w%[1]d;Tom;Weg%[1]d;deaktiviert;Schüler(innen);4c;;\n", unique)

	rr := testutil.ExecuteRequest(router, newIServRequest(t, "/students/preview", export, "", claims))
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	var preview iservPreview
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &preview))
	assert.Equal(t, 1, preview.Data.TotalRows, "only the active pupil is previewed")
	assert.Equal(t, 1, preview.Data.CreatedCount)
	require.NotNil(t, preview.Data.IServ)
	assert.Equal(t, 1, preview.Data.IServ.DeactivatedCount)
	assert.ElementsMatch(t, []iservUserType{
		{Value: "Schüler(innen)", Count: 1, Selected: true},
		{Value: "Lehrer(innen)", Count: 1, Selected: false},
	}, preview.Data.IServ.UserTypes)

	// Nothing chosen: an empty preview, not an error, still listing the types.
	rr = testutil.ExecuteRequest(router, newIServRequest(t, "/students/preview", export, iservChoice(t), claims))
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	preview = iservPreview{}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &preview))
	assert.Equal(t, 0, preview.Data.TotalRows)
	require.NotNil(t, preview.Data.IServ)
	assert.Len(t, preview.Data.IServ.UserTypes, 2)

	rr = testutil.ExecuteRequest(router, newIServRequest(t, "/students/import", export, iservChoice(t, "Schüler(innen)"), claims))
	require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
	assert.Contains(t, rr.Body.String(), `"CreatedCount":1`)

	var stored []struct {
		LastName    string `bun:"last_name"`
		SchoolClass string `bun:"school_class"`
	}
	err := tc.db.NewSelect().TableExpr("users.student_profiles AS students").
		ColumnExpr("p.last_name, membership.school_class").
		Join("JOIN users.student_school_memberships AS membership ON membership.student_profile_id = students.id AND membership.tenant_id = students.tenant_id AND membership.deleted_at IS NULL").
		Join("JOIN users.persons AS p ON p.id = students.person_id").
		Where("p.last_name IN (?, ?, ?)", fmt.Sprintf("Kind%d", unique), fmt.Sprintf("Lehrkraft%d", unique), fmt.Sprintf("Weg%d", unique)).
		Scan(context.Background(), &stored)
	require.NoError(t, err)
	require.Len(t, stored, 1, "neither the teacher nor the deactivated pupil became a child")
	assert.Equal(t, fmt.Sprintf("Kind%d", unique), stored[0].LastName)
	assert.Equal(t, "1a", stored[0].SchoolClass)

	// A choice that is no JSON list is refused, never read as "all".
	rr = testutil.ExecuteRequest(router, newIServRequest(t, "/students/preview", export, "Schüler(innen)", claims))
	assert.Equal(t, http.StatusBadRequest, rr.Code, rr.Body.String())
}
