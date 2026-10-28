package api

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// The complete production walk is partitioned only AFTER the independent
// security/module assertions. No expected file determines the walked routes.
func partitionRouteLines(t *testing.T, table string) map[string][]string {
	t.Helper()
	parts := map[string][]string{}
	seen := map[string]bool{}
	for _, line := range strings.Split(strings.TrimSuffix(table, "\n"), "\n") {
		fields := strings.Fields(line)
		require.GreaterOrEqual(t, len(fields), 2)
		identity := fields[0] + " " + fields[1]
		require.Falsef(t, seen[identity], "duplicate observed route: %s", identity)
		seen[identity] = true
		key := routeResource(fields[1])
		parts[key] = append(parts[key], line)
	}
	for _, lines := range parts {
		sort.Strings(lines)
	}
	return parts
}

func goldenResourceFiles(t *testing.T, directory string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(directory, func(filename string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		require.Truef(t, entry.Type().IsRegular() && strings.HasSuffix(filename, ".golden"), "unexpected golden entry %s", filename)
		relative, err := filepath.Rel(directory, filename)
		if err != nil {
			return err
		}
		files[strings.TrimSuffix(filepath.ToSlash(relative), ".golden")] = filename
		return nil
	})
	require.NoError(t, err)
	require.NotEmpty(t, files)
	return files
}

func compareResourceGoldens(t *testing.T, directory, got, hint string, middleware bool) {
	t.Helper()
	parts := partitionRouteLines(t, got)
	existing := goldenResourceFiles(t, directory)
	for key, filename := range existing {
		if _, present := parts[key]; !present {
			if *updateGoldens {
				require.NoError(t, os.Remove(filename))
			} else {
				t.Errorf("stale resource golden %s: %s", filename, hint)
			}
		}
	}
	for key, lines := range parts {
		filename := filepath.Join(directory, filepath.FromSlash(key)+".golden")
		if *updateGoldens {
			require.NoError(t, os.MkdirAll(filepath.Dir(filename), 0750))
		}
		table := strings.Join(lines, "\n") + "\n"
		if middleware {
			compareMiddlewareGolden(t, filename, table, hint)
		} else {
			compareGolden(t, filename, table, hint)
		}
	}
}

// The command-line update flag is process-global. Keep this test sequential;
// parallel contract tests start only after sequential top-level tests return.
func TestResourceGoldenUpdateAddsAndRetiresFragments(t *testing.T) {
	previous := *updateGoldens
	*updateGoldens = true
	defer func() { *updateGoldens = previous }()
	root := t.TempDir()
	directory := filepath.Join(root, "testdata", "routes")
	retired := filepath.Join(directory, "api", "retired.golden")
	require.NoError(t, os.MkdirAll(filepath.Dir(retired), 0750))
	require.NoError(t, os.WriteFile(retired, []byte("GET /api/retired/\n"), 0600))
	compareResourceGoldens(t, directory, "GET /api/students/\nPOST /school/calendar/\n", "intentional resource change", false)
	_, err := os.Stat(retired)
	require.ErrorIs(t, err, os.ErrNotExist)
	routes, err := readRouteCatalog(os.DirFS(root))
	require.NoError(t, err)
	require.Equal(t, []Route{{"GET", "/api/students/"}, {"POST", "/school/calendar/"}}, routes)
}
