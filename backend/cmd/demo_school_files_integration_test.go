package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/delivery/objects"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	tenancyCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// The demo process removes the uploads of retired demo schools (#5544). The
// root binds the owner to the real uploads store here, as the dev tools
// binary does on the demo host: every
// namespace of the file catalog, orphaned files and subdirectories included,
// as the phoenix_demo role, and records a marker only when nothing is left.
// Live schools, the standing school, a school hidden less than an hour ago
// and the school of an order still being seeded keep every file.

func demoFileKinds(t *testing.T) []organizationtenancy.LifecycleFileKind {
	t.Helper()
	raw, err := organizationtenancy.DataLifecycleCatalogSource.ReadFile("data_lifecycle_catalog.json")
	require.NoError(t, err)
	var catalog organizationtenancy.DataLifecycleCatalog
	require.NoError(t, json.Unmarshal(raw, &catalog))
	require.NotEmpty(t, catalog.Files)
	return catalog.Files
}

func writeDemoUpload(t *testing.T, root string, parts ...string) {
	t.Helper()
	path := filepath.Join(append([]string{root}, parts...)...)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte(path), 0o600))
}

// seedDemoUploads writes objects of the school into every namespace of the
// catalog, nested ones and ones no database row names, plus decoys of a
// school whose ID only starts with the school's ID. Every file is an orphan:
// the erasure goes by namespace, never by a stored file name.
func seedDemoUploads(t *testing.T, root string, kinds []organizationtenancy.LifecycleFileKind, schoolID int64) {
	t.Helper()
	id := strconv.FormatInt(schoolID, 10)
	for _, kind := range kinds {
		switch kind.Layout {
		case "tenantDir":
			writeDemoUpload(t, root, kind.Kind, id, "orphan.pdf")
			writeDemoUpload(t, root, kind.Kind, id, "nested", "deeper", "orphan.png")
			writeDemoUpload(t, root, kind.Kind, id+"0", "decoy.pdf")
		case "tenantPrefix":
			writeDemoUpload(t, root, kind.Kind, id+"_orphan.png")
			writeDemoUpload(t, root, kind.Kind, id+"_second.pdf")
			writeDemoUpload(t, root, kind.Kind, id+"0_decoy.png")
		case "account":
			// Shared account objects stay; the legacy school directory goes.
			writeDemoUpload(t, root, kind.Kind, "global", id+"_account.png")
			writeDemoUpload(t, root, kind.Kind, id, "legacy.png")
			writeDemoUpload(t, root, kind.Kind, id, "nested", "legacy.png")
		default:
			t.Fatalf("file kind %s has layout %s; seed it here", kind.Kind, kind.Layout)
		}
	}
}

// demoUploadTree lists every file below root with its content.
func demoUploadTree(t *testing.T, root string) map[string]string {
	t.Helper()
	tree := map[string]string{}
	require.NoError(t, filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		tree[rel] = string(content)
		return err
	}))
	return tree
}

// schoolUploads is the part of the tree that names the school: a path
// segment that is its ID or a file name with its ID as prefix. The global
// account objects are shared and not the school's.
func schoolUploads(tree map[string]string, schoolID int64) map[string]string {
	id := strconv.FormatInt(schoolID, 10)
	owned := map[string]string{}
	for path, content := range tree {
		segments := strings.Split(filepath.ToSlash(path), "/")
		if len(segments) > 1 && segments[1] == "global" {
			continue
		}
		for _, segment := range segments {
			if segment == id || strings.HasPrefix(segment, id+"_") {
				owned[path] = content
				break
			}
		}
	}
	return owned
}

// schoolPaths lists every file or directory below root whose path names the
// school: a segment that is its ID or starts with its ID and an underscore.
// The shared global account objects are not the school's.
func schoolPaths(t *testing.T, root string, schoolID int64) []string {
	t.Helper()
	id := strconv.FormatInt(schoolID, 10)
	var paths []string
	require.NoError(t, filepath.WalkDir(root, func(path string, _ fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		segments := strings.Split(filepath.ToSlash(rel), "/")
		if len(segments) > 1 && segments[1] == "global" {
			return err
		}
		for _, segment := range segments {
			if segment == id || strings.HasPrefix(segment, id+"_") {
				paths = append(paths, rel)
				break
			}
		}
		return err
	}))
	return paths
}

func hideDemoSchool(t *testing.T, db *bun.DB, schoolID int64, ago string) {
	t.Helper()
	_, err := db.NewRaw(`UPDATE platform.schools SET deleted_at = NOW() - ?::interval WHERE id = ?`, ago, schoolID).Exec(t.Context())
	require.NoError(t, err)
}

// orderDemoSchools records the demo origin of the schools as the demo control
// plane does: an order names them, then the visitor erasure deletes the order
// (#4048). Only the origin the trigger recorded remains.
func orderDemoSchools(t *testing.T, db *bun.DB, schoolIDs ...int64) {
	t.Helper()
	for _, id := range schoolIDs {
		name := "ogs-origin-" + strconv.FormatInt(id, 10)
		_, err := db.NewRaw(`INSERT INTO platform.demo_school_states (name, tenant_id, seed_state, status) VALUES (?, ?, '{}'::jsonb, 'ready')`, name, id).Exec(t.Context())
		require.NoError(t, err)
		_, err = db.NewRaw(`DELETE FROM platform.demo_school_states WHERE name = ?`, name).Exec(t.Context())
		require.NoError(t, err)
	}
}

// storageSeam lets a test break the uploads store at one point: removal of
// one namespace fails, or an object appears there again right after its
// removal, as a late write would.
type storageSeam struct {
	*objects.Local
	failRemoval, writeLate string
	lateWrites             int
}

func (s *storageSeam) TenantObjects(ctx context.Context, kind, layout string, id int64, remove bool) (int64, error) {
	if remove && kind == s.failRemoval {
		return 0, errors.New("synthetic removal failure")
	}
	n, err := s.Local.TenantObjects(ctx, kind, layout, id, remove)
	if remove && err == nil && kind == s.writeLate {
		s.lateWrites++
		late := filepath.Join(s.Root, kind, strconv.FormatInt(id, 10), "late.pdf")
		if err := os.MkdirAll(filepath.Dir(late), 0o700); err != nil {
			return n, err
		}
		return n, os.WriteFile(late, []byte("late"), 0o600)
	}
	return n, err
}

func demoFileMarker(t *testing.T, db *bun.DB, schoolID int64) map[string]int64 {
	t.Helper()
	var raw []byte
	err := db.NewRaw(`SELECT file_counts FROM platform.demo_school_file_erasures WHERE school_id = ?`, schoolID).Scan(t.Context(), &raw)
	if err != nil {
		require.ErrorContains(t, err, "no rows")
		return nil
	}
	counts := map[string]int64{}
	require.NoError(t, json.Unmarshal(raw, &counts))
	return counts
}

func TestDemoProcessErasesTheUploadsOfRetiredDemoSchools(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	kinds := demoFileKinds(t)
	uploads := t.TempDir()

	retired, _ := testpkg.CreateTestTenant(t, db)
	live, _ := testpkg.CreateTestTenant(t, db)
	standing, _ := testpkg.CreateTestTenant(t, db)
	recent, _ := testpkg.CreateTestTenant(t, db)
	seeding, _ := testpkg.CreateTestTenant(t, db)
	restored, _ := testpkg.CreateTestTenant(t, db)
	orderDemoSchools(t, db, retired, live, recent)
	hideDemoSchool(t, db, retired, "2 hours")
	hideDemoSchool(t, db, recent, "10 minutes")
	// The standing school is never erased, even hidden for hours.
	hideDemoSchool(t, db, standing, "2 hours")
	// A hidden school whose order is still being seeded: the seeder may still
	// write to it, so its files wait for the order.
	hideDemoSchool(t, db, seeding, "2 hours")
	// A soft-deleted school without a demo origin, such as a real school
	// restored into the demo database, is not the demo process's to erase.
	hideDemoSchool(t, db, restored, "2 hours")
	_, err := db.NewRaw(`INSERT INTO platform.demo_school_states (name, tenant_id, seed_state, status) VALUES
		(?, ?, '{}'::jsonb, 'ready'), ('ogs-seeding-k3m9xp', ?, NULL, 'preparing')`,
		organizationtenancy.StandingDemoSchoolSlug, standing, seeding).Exec(ctx)
	require.NoError(t, err)
	for _, id := range []int64{retired, live, standing, recent, seeding, restored} {
		seedDemoUploads(t, uploads, kinds, id)
	}
	before := demoUploadTree(t, uploads)
	require.NotEmpty(t, schoolUploads(before, retired))

	files, err := tenancyCompose.NewDemoSchoolFiles(testpkg.DemoRoleDatabase(t, db), objects.NewLocal(uploads))
	require.NoError(t, err)
	schools, removed, err := files.EraseRetiredDemoSchoolFiles(ctx)
	require.NoError(t, err)

	after := demoUploadTree(t, uploads)
	assert.Equal(t, 1, schools)
	assert.Empty(t, schoolUploads(after, retired), "no object of the retired school is left in any namespace")
	assert.Empty(t, schoolPaths(t, uploads, retired), "no file or directory names the retired school")
	assert.Equal(t, int64(len(schoolUploads(before, retired))), removed, "every object of the retired school is counted")
	for path, content := range before {
		if _, gone := schoolUploads(before, retired)[path]; !gone {
			assert.Equal(t, content, after[path], "%s is not the retired school's and stays", path)
		}
	}
	for _, id := range []int64{live, standing, recent, seeding, restored} {
		assert.Equal(t, schoolUploads(before, id), schoolUploads(after, id), "school %d keeps every file", id)
		assert.Nil(t, demoFileMarker(t, db, id))
	}

	marker := demoFileMarker(t, db, retired)
	require.NotNil(t, marker, "the marker records the done school")
	assert.Len(t, marker, len(kinds), "one count per namespace of the catalog")
	var total int64
	for _, n := range marker {
		total += n
	}
	assert.Equal(t, removed, total)

	// A second run finds nothing to do and changes nothing.
	schools, removed, err = files.EraseRetiredDemoSchoolFiles(ctx)
	require.NoError(t, err)
	assert.Zero(t, schools)
	assert.Zero(t, removed)
	assert.Equal(t, after, demoUploadTree(t, uploads))
	assert.Equal(t, marker, demoFileMarker(t, db, retired))
}

// A namespace that is a symbolic link fails closed: nothing outside the
// uploads is followed, the school keeps no marker, and once the storage is
// repaired the next run finishes it.
func TestDemoSchoolFileErasureRetriesASchoolAfterAStorageFailure(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	kinds := demoFileKinds(t)
	uploads := t.TempDir()
	outside := t.TempDir()

	retired, _ := testpkg.CreateTestTenant(t, db)
	orderDemoSchools(t, db, retired)
	hideDemoSchool(t, db, retired, "2 hours")
	seedDemoUploads(t, uploads, kinds, retired)
	id := strconv.FormatInt(retired, 10)
	writeDemoUpload(t, outside, "not-the-schools.pdf")
	require.NoError(t, os.RemoveAll(filepath.Join(uploads, "files", id)))
	require.NoError(t, os.Symlink(outside, filepath.Join(uploads, "files", id)))

	files, err := tenancyCompose.NewDemoSchoolFiles(testpkg.DemoRoleDatabase(t, db), objects.NewLocal(uploads))
	require.NoError(t, err)
	schools, _, err := files.EraseRetiredDemoSchoolFiles(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "demo school "+id)
	assert.NotContains(t, err.Error(), "not-the-schools", "no file name in the error")
	assert.Zero(t, schools)
	assert.Nil(t, demoFileMarker(t, db, retired), "a failed school keeps no marker")
	assert.FileExists(t, filepath.Join(outside, "not-the-schools.pdf"), "the link is not followed")

	require.NoError(t, os.Remove(filepath.Join(uploads, "files", id)))
	writeDemoUpload(t, uploads, "files", id, "after-repair.pdf")
	schools, _, err = files.EraseRetiredDemoSchoolFiles(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, schools)
	assert.Empty(t, schoolPaths(t, uploads, retired))
	assert.NotNil(t, demoFileMarker(t, db, retired))
}

// The removal is checked by counting again: an object that is still there
// after the removal fails the school without a marker, and the next run,
// with nothing left behind, finishes it.
func TestDemoSchoolFileErasureCountsAgainAfterTheRemoval(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	uploads := t.TempDir()
	retired, _ := testpkg.CreateTestTenant(t, db)
	orderDemoSchools(t, db, retired)
	hideDemoSchool(t, db, retired, "2 hours")
	seedDemoUploads(t, uploads, demoFileKinds(t), retired)

	seam := &storageSeam{Local: objects.NewLocal(uploads), writeLate: "files"}
	files, err := tenancyCompose.NewDemoSchoolFiles(testpkg.DemoRoleDatabase(t, db), seam)
	require.NoError(t, err)
	schools, _, err := files.EraseRetiredDemoSchoolFiles(ctx)
	require.Error(t, err, "an object left after the removal fails the school")
	assert.Equal(t, 1, seam.lateWrites)
	assert.Zero(t, schools)
	assert.Nil(t, demoFileMarker(t, db, retired), "no marker while an object is left")
	assert.NotEmpty(t, schoolPaths(t, uploads, retired))

	seam.writeLate = ""
	schools, _, err = files.EraseRetiredDemoSchoolFiles(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, schools, "the next run finishes the school")
	assert.Empty(t, schoolPaths(t, uploads, retired))
	assert.NotNil(t, demoFileMarker(t, db, retired))
}

// The marker follows the removal, never the count before it: a removal that
// fails after every namespace was counted leaves no marker, and the next run
// erases the school.
func TestDemoSchoolFileErasureWritesNoMarkerWhenTheRemovalFails(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	uploads := t.TempDir()
	retired, _ := testpkg.CreateTestTenant(t, db)
	orderDemoSchools(t, db, retired)
	hideDemoSchool(t, db, retired, "2 hours")
	seedDemoUploads(t, uploads, demoFileKinds(t), retired)

	seam := &storageSeam{Local: objects.NewLocal(uploads), failRemoval: "student-documents"}
	files, err := tenancyCompose.NewDemoSchoolFiles(testpkg.DemoRoleDatabase(t, db), seam)
	require.NoError(t, err)
	schools, _, err := files.EraseRetiredDemoSchoolFiles(ctx)
	require.Error(t, err)
	assert.Zero(t, schools)
	assert.Nil(t, demoFileMarker(t, db, retired), "a failed removal leaves no marker")
	assert.FileExists(t, filepath.Join(uploads, "student-documents", strconv.FormatInt(retired, 10), "orphan.pdf"))

	seam.failRemoval = ""
	schools, _, err = files.EraseRetiredDemoSchoolFiles(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, schools)
	assert.Empty(t, schoolPaths(t, uploads, retired))
	assert.NotNil(t, demoFileMarker(t, db, retired))
}
