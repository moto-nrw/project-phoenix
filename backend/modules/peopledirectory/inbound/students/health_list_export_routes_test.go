// POST /students/export with the Gesundheitsliste preset (#3323), end to end
// through the production Router(): the permission gate, the rows that reach
// the file, and the audit record every export leaves behind.
package students_test

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"

	"github.com/moto-nrw/project-phoenix/auth/authorize/permissions"
	"github.com/moto-nrw/project-phoenix/modules/identityaccess/legacy/jwt"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func healthListClaims(tb testing.TB, accountID int64) jwt.AppClaims {
	return jwt.AppClaims{
		ID:        int(accountID),
		Sub:       "health-list@example.com",
		Username:  "health-list",
		FirstName: "Health",
		LastName:  "List",
		Roles:     []string{"user"},
		TenantID:  testpkg.RebaseTenantID(tb, 1),
	}
}

func setStudentHealthInfo(t *testing.T, db *bun.DB, studentID int64, note string) {
	t.Helper()
	_, err := db.NewRaw(`UPDATE users.student_care_profiles SET health_info = ?
		WHERE membership_id = (SELECT id FROM users.student_school_memberships WHERE student_profile_id = ? AND deleted_at IS NULL)`,
		note, studentID).Exec(context.Background())
	require.NoError(t, err, "failed to store health note")
}

func healthListAuditCount(t *testing.T, db *bun.DB, accountID int64) int {
	t.Helper()
	var count int
	err := db.NewSelect().
		TableExpr("audit.data_access_log").
		ColumnExpr("count(*)").
		Where("resource_type = ?", "student_health_list_export").
		Where("actor_account_id = ?", accountID).
		Where("student_id IS NULL").
		Scan(context.Background(), &count)
	require.NoError(t, err)
	return count
}

var (
	docxTag = regexp.MustCompile(`<[^>]+>`)
	// xmlEntities undoes the escaping the DOCX writer applies to cell text.
	xmlEntities = strings.NewReplacer("&lt;", "<", "&gt;", ">", "&quot;", `"`, "&apos;", "'", "&#39;", "'", "&#34;", `"`, "&amp;", "&")
)

// healthListDocument reads the exported DOCX with the standard library (the
// architecture policy keeps rendering libraries out of this package's tests).
// It returns every table row, cells joined by " | ", so a test can look for a
// child's line, and the whole document text for the paragraphs around it.
func healthListDocument(t *testing.T, data []byte) (rows []string, text string) {
	t.Helper()
	archive, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	require.NoError(t, err)
	file, err := archive.Open("word/document.xml")
	require.NoError(t, err)
	defer func() { _ = file.Close() }()
	raw, err := io.ReadAll(file)
	require.NoError(t, err)

	plain := func(fragment string) string {
		fragment = strings.ReplaceAll(fragment, "</w:tc>", " | ")
		return xmlEntities.Replace(docxTag.ReplaceAllString(fragment, ""))
	}
	for _, row := range strings.Split(string(raw), "</w:tr>") {
		if start := strings.LastIndex(row, "<w:tr>"); start >= 0 {
			rows = append(rows, strings.TrimSuffix(plain(row[start:]), " | "))
		}
	}
	return rows, plain(string(raw))
}

func rowsMentioning(lines []string, needle string) []string {
	var hits []string
	for _, line := range lines {
		if strings.Contains(line, needle) {
			hits = append(hits, line)
		}
	}
	return hits
}

func TestHealthListExportRequiresUsersRead(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)
	account := testpkg.CreateTestAccount(t, tc.db, "health-list-denied@example.com")

	rr := authExec(t, tc, birthdayExportRequest(t, `{"format":"xlsx","preset":"health_list"}`),
		healthListClaims(t, account.ID), []string{permissions.RoomsRead})

	assert.Equal(t, http.StatusForbidden, rr.Code, rr.Body.String())
	assert.Zero(t, healthListAuditCount(t, tc.db, account.ID), "a refused export must not be audited as served")
}

func TestHealthListExportPrintsNotesAndWritesAudit(t *testing.T) {
	t.Parallel()

	tc := setupStudentsRoute(t)
	account := testpkg.CreateTestAccount(t, tc.db, "health-list@example.com")

	// The search keeps the file to this test's two children.
	surname := fmt.Sprintf("Gesundheit%d", account.ID)
	withNote := testpkg.CreateTestStudent(t, tc.db, "Mila", surname, "3a")
	testpkg.CreateTestStudent(t, tc.db, "Jonas", surname, "3a")
	setStudentHealthInfo(t, tc.db, withNote.ID, "Nussallergie, Notfallset im Gruppenraum")

	t.Run("default lists only children with a note", func(t *testing.T) {
		body := fmt.Sprintf(`{"format":"docx","preset":"health_list","filters":{"search":%q}}`, surname)
		rr := authExec(t, tc, birthdayExportRequest(t, body), healthListClaims(t, account.ID),
			[]string{permissions.UsersRead})
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())
		assert.Contains(t, rr.Header().Get("Content-Disposition"), "gesundheitsliste.docx")

		lines, text := healthListDocument(t, rr.Body.Bytes())
		mila := rowsMentioning(lines, "Mila "+surname)
		require.Len(t, mila, 1, "the child with a note belongs on the list: %v", lines)
		assert.Contains(t, mila[0], "Nussallergie, Notfallset im Gruppenraum")
		assert.Empty(t, rowsMentioning(lines, "Jonas "+surname),
			"a child without a note stays off the default list")
		assert.NotEmpty(t, rowsMentioning(lines, "Gesundheitsinformationen"), "the health column needs its heading")
		assert.Contains(t, text, "Nur Kinder mit hinterlegten Gesundheitsinformationen",
			"the document must say that children without a note are left out")
	})

	t.Run("children without a note on request", func(t *testing.T) {
		body := fmt.Sprintf(`{"format":"docx","preset":"health_list","filters":{"search":%q,"include_without_health_info":true}}`, surname)
		rr := authExec(t, tc, birthdayExportRequest(t, body), healthListClaims(t, account.ID),
			[]string{permissions.UsersRead})
		require.Equal(t, http.StatusOK, rr.Code, rr.Body.String())

		lines, text := healthListDocument(t, rr.Body.Bytes())
		assert.NotContains(t, text, "Nur Kinder mit hinterlegten Gesundheitsinformationen",
			"with every child listed the scope note must go")
		jonas := rowsMentioning(lines, "Jonas "+surname)
		require.Len(t, jonas, 1, "the child without a note must be listed: %v", lines)
		assert.Contains(t, jonas[0], "Nicht hinterlegt", "an empty cell would read as \"no allergies\"")
	})

	assert.Equal(t, 2, healthListAuditCount(t, tc.db, account.ID), "every served export writes one audit record")
}
