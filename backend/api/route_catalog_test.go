package api

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRouteCatalogFragments(t *testing.T) {
	t.Parallel()
	source := map[string]string{
		"testdata/routes/api/students.golden":    "POST /api/students/\nGET /api/students/{id}\n",
		"testdata/routes/school/calendar.golden": "GET /school/calendar/\n",
	}
	routes, err := readRouteCatalog(catalogTestFS(t, source))
	require.NoError(t, err)
	require.Equal(t, []Route{{"GET", "/api/students/{id}"}, {"GET", "/school/calendar/"}, {"POST", "/api/students/"}}, routes)
}

func TestRouteCatalogRejectsAmbiguousOrMissingContracts(t *testing.T) {
	t.Parallel()
	for name, source := range map[string]map[string]string{
		"empty":          {},
		"malformed":      {"testdata/routes/api/students.golden": "GET\n"},
		"duplicate":      {"testdata/routes/api/students.golden": "GET /api/students/\nGET /api/students/\n"},
		"misplaced":      {"testdata/routes/api/staff.golden": "GET /api/students/\n"},
		"unknown file":   {"testdata/routes/api/students.json": "GET /api/students/\n"},
		"invalid method": {"testdata/routes/api/students.golden": " /api/students/\n"},
		"invalid path":   {"testdata/routes/api/students.golden": "GET /api/students/ extra\n"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			_, err := readRouteCatalog(catalogTestFS(t, source))
			require.Error(t, err)
		})
	}
}

func catalogTestFS(t *testing.T, files map[string]string) fs.FS {
	t.Helper()
	root := t.TempDir()
	for name, data := range files {
		filename := filepath.Join(root, name)
		require.NoError(t, os.MkdirAll(filepath.Dir(filename), 0750))
		require.NoError(t, os.WriteFile(filename, []byte(data), 0600))
	}
	return os.DirFS(root)
}
