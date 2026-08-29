package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/api/common"
	configModels "github.com/moto-nrw/project-phoenix/models/config"
	"github.com/moto-nrw/project-phoenix/models/filestore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type seededFolder struct {
	Name       string         `json:"name"`
	Visibility string         `json:"visibility"`
	RoleIDs    []string       `json:"role_ids"`
	AccountIDs []string       `json:"account_ids"`
	Files      []seededUpload `json:"-"`
}

type seededUpload struct {
	Filename    string
	ContentType string
	Size        int64
}

// fileStorageSeedServer answers the four endpoints the step drives and, for
// uploads, runs the production upload validator: demo bytes that the real
// endpoint would reject must fail this test, not the seed run.
func fileStorageSeedServer(t *testing.T, settings map[string]any) (*httptest.Server, *[]seededFolder) {
	t.Helper()

	folders := make([]seededFolder, 0, 3)
	byID := make(map[string]int)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		switch {
		case strings.HasPrefix(path, "/api/settings/values/"):
			var body struct {
				Value any `json:"value"`
			}
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			settings[strings.TrimPrefix(path, "/api/settings/values/")] = body.Value
			_, _ = fmt.Fprint(w, `{"status":"success","data":null}`)

		case path == "/api/files/audience":
			_, _ = fmt.Fprint(w, `{"status":"success","data":{
				"roles":[{"id":"7","name":"Verwaltung"},{"id":"9007199254740993","name":"Betreuungskraft"}],
				"accounts":[{"account_id":"41"},{"account_id":"42"},{"account_id":"43"}]}}`)

		case path == "/api/files/folders":
			var body seededFolder
			require.NoError(t, json.NewDecoder(r.Body).Decode(&body))
			folders = append(folders, body)
			id := fmt.Sprintf("%d", len(folders))
			byID[id] = len(folders) - 1
			_, _ = fmt.Fprintf(w, `{"status":"success","data":{"id":"%s"}}`, id)

		case strings.HasSuffix(path, "/files"):
			id := strings.TrimSuffix(strings.TrimPrefix(path, "/api/files/folders/"), "/files")
			index, ok := byID[id]
			require.True(t, ok, "upload into unknown folder %q", id)

			uploaded, err := common.ParseOfficeFileWithLimits(w, r, "file", 25<<20, 26<<20)
			require.NoError(t, err, "demo file rejected by the real upload validator")
			defer common.CloseFile(uploaded.File)

			folders[index].Files = append(folders[index].Files, seededUpload{
				Filename:    uploaded.Filename,
				ContentType: uploaded.ContentType,
				Size:        r.ContentLength,
			})
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"id":"1"}}`)

		default:
			t.Errorf("unexpected request %s %s", r.Method, path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &folders
}

func TestSeedFileStorageCreatesEveryVisibility(t *testing.T) {
	t.Parallel()

	settings := make(map[string]any)
	srv, folders := fileStorageSeedServer(t, settings)

	rt := &Runtime{Client: newTestClient(srv.URL, false)}
	require.NoError(t, seedFileStorageStep{}.Run(t.Context(), rt))

	assert.Equal(t, true, settings[configModels.KeyFilesStaffUploadEnabled],
		"the demo should show the team-upload side of the storage")

	require.Len(t, *folders, 3)
	seen := make(map[string]seededFolder, len(*folders))
	for _, folder := range *folders {
		seen[folder.Visibility] = folder
	}
	require.Contains(t, seen, filestore.VisibilityAllStaff)
	require.Contains(t, seen, filestore.VisibilityAdmins)
	require.Contains(t, seen, filestore.VisibilitySelected)

	shared := seen[filestore.VisibilitySelected]
	assert.Equal(t, []string{"9007199254740993"}, shared.RoleIDs,
		"the named role wins over the one that sorts first, and its id stays exact")
	assert.Equal(t, []string{"41", "42"}, shared.AccountIDs)

	files := 0
	for _, folder := range *folders {
		files += len(folder.Files)
		assert.NotEmpty(t, folder.Files, "folder %q was seeded without a file", folder.Name)
	}
	assert.Equal(t, 4, files)
}

func TestSeedFileStorageDemoFilesAreRealFiles(t *testing.T) {
	t.Parallel()

	settings := make(map[string]any)
	srv, folders := fileStorageSeedServer(t, settings)

	rt := &Runtime{Client: newTestClient(srv.URL, false)}
	require.NoError(t, seedFileStorageStep{}.Run(t.Context(), rt))

	byName := make(map[string]seededUpload)
	for _, folder := range *folders {
		for _, file := range folder.Files {
			byName[file.Filename] = file
		}
	}

	pdf, ok := byName["Notfallblatt.pdf"]
	require.True(t, ok)
	assert.Equal(t, "application/pdf", pdf.ContentType)

	docx, ok := byName["Elternbrief Vorlage.docx"]
	require.True(t, ok)
	assert.Equal(t, common.DocxContentType, docx.ContentType,
		"a DOCX is only accepted when the container really carries word/document.xml")
}

func TestSeedFileStorageRefusesEmptyAudience(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/files/audience" {
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"roles":[],"accounts":[]}}`)
			return
		}
		_, _ = fmt.Fprint(w, `{"status":"success","data":null}`)
	}))
	defer srv.Close()

	rt := &Runtime{Client: newTestClient(srv.URL, false)}
	err := seedFileStorageStep{}.Run(t.Context(), rt)
	require.Error(t, err, "a storage whose share model cannot be seeded must fail loudly")
	assert.Contains(t, err.Error(), "audience is empty")
}
