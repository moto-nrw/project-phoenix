package api

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"testing"

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
// uploads, records the multipart metadata emitted by the seed client.
func fileStorageSeedServer(t *testing.T, settings map[string]any) (*seedHTTPTestServer, *[]seededFolder) {
	t.Helper()

	folders := make([]seededFolder, 0, 3)
	byID := make(map[string]int)

	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
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

			require.NoError(t, r.ParseMultipartForm(26<<20))
			file, header, err := r.FormFile("file")
			require.NoError(t, err)
			defer func() { require.NoError(t, file.Close()) }()

			contents, err := io.ReadAll(file)
			require.NoError(t, err)
			contentType := validatedSeedFileContentType(t, header.Filename, contents)
			folders[index].Files = append(folders[index].Files, seededUpload{
				Filename:    header.Filename,
				ContentType: contentType,
				Size:        r.ContentLength,
			})
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"id":"1"}}`)

		default:
			t.Errorf("unexpected request %s %s", r.Method, path)
			w.WriteHeader(seedHTTPStatusNotFound)
		}
	})
	t.Cleanup(srv.Close)
	return srv, &folders
}

func validatedSeedFileContentType(t *testing.T, filename string, contents []byte) string {
	t.Helper()
	if strings.HasSuffix(filename, ".pdf") {
		require.True(t, bytes.HasPrefix(contents, []byte("%PDF-")), "seeded PDF has no PDF signature")
		return "application/pdf"
	}

	reader, err := zip.NewReader(bytes.NewReader(contents), int64(len(contents)))
	require.NoError(t, err, "seeded DOCX is not a ZIP container")
	for _, file := range reader.File {
		if file.Name == "word/document.xml" {
			return "application/vnd.openxmlformats-officedocument.wordprocessingml.document"
		}
	}
	require.FailNow(t, "seeded DOCX has no word/document.xml")
	return ""
}

func TestSeedFileStorageCreatesEveryVisibility(t *testing.T) {
	t.Parallel()

	settings := make(map[string]any)
	srv, folders := fileStorageSeedServer(t, settings)

	rt := &Runtime{Client: newTestClient(srv.URL, false)}
	require.NoError(t, seedFileStorageStep{}.Run(t.Context(), rt))

	assert.Equal(t, true, settings["files.staff_upload_enabled"],
		"the demo should show the team-upload side of the storage")

	require.Len(t, *folders, 3)
	seen := make(map[string]seededFolder, len(*folders))
	for _, folder := range *folders {
		seen[folder.Visibility] = folder
	}
	require.Contains(t, seen, "all_staff")
	require.Contains(t, seen, "admins")
	require.Contains(t, seen, "selected")

	shared := seen["selected"]
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
	assert.Equal(t, "application/vnd.openxmlformats-officedocument.wordprocessingml.document", docx.ContentType)
}

func TestSeedFileStorageRefusesEmptyAudience(t *testing.T) {
	t.Parallel()

	srv := newSeedHTTPTestServer(func(w seedHTTPResponseWriter, r *seedHTTPRequest) {
		if r.URL.Path == "/api/files/audience" {
			_, _ = fmt.Fprint(w, `{"status":"success","data":{"roles":[],"accounts":[]}}`)
			return
		}
		_, _ = fmt.Fprint(w, `{"status":"success","data":null}`)
	})
	defer srv.Close()

	rt := &Runtime{Client: newTestClient(srv.URL, false)}
	err := seedFileStorageStep{}.Run(t.Context(), rt)
	require.Error(t, err, "a storage whose share model cannot be seeded must fail loudly")
	assert.Contains(t, err.Error(), "audience is empty")
}
