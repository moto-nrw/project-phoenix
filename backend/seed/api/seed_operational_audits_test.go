package api

import (
	"fmt"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The import audit updates a child the school already has instead of
// creating a test child without parents, group or plan (#3894).
func TestSeedImportAuditUpdatesAnExistingChild(t *testing.T) {
	t.Parallel()

	student := DemoStudents[importAuditStudentIndex]
	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		assert.Equal(t, "/api/import/students/import", r.URL.Path)
		require.NoError(t, r.ParseMultipartForm(1<<20))
		assert.Equal(t, "upsert", r.FormValue("mode"))
		file, _, err := r.FormFile("file")
		require.NoError(t, err)
		defer func() { _ = file.Close() }()
		contents, err := io.ReadAll(file)
		require.NoError(t, err)
		assert.Equal(t, fmt.Sprintf("Vorname,Nachname,Klasse\n%s,%s,%s\n", student.FirstName, student.LastName, student.Class), string(contents))
		_, _ = fmt.Fprint(w, `{"status":"success","data":{"CreatedCount":0,"UpdatedCount":1,"ErrorCount":0}}`)
	})
	defer srv.Close()

	rt := &Runtime{Client: newTestClient(srv.URL, false), TenantAuth: AuthRef{Token: "admin"}}
	require.NoError(t, (seedImportAuditStep{}).Run(t.Context(), rt))
}

func TestSeedImportAuditRejectsACreatedChild(t *testing.T) {
	t.Parallel()

	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, _ *seedHTTPRequest) {
		_, _ = fmt.Fprint(w, `{"status":"success","data":{"CreatedCount":1,"UpdatedCount":0,"ErrorCount":0}}`)
	})
	defer srv.Close()

	rt := &Runtime{Client: newTestClient(srv.URL, false), TenantAuth: AuthRef{Token: "admin"}}
	err := (seedImportAuditStep{}).Run(t.Context(), rt)
	require.ErrorContains(t, err, "want 1 updated child")
}
