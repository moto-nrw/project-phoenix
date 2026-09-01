package test

import (
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// sourceFile is one Go file under the backend root, read once for every
// ratchet walker in this package.
type sourceFile struct {
	rel     string // backend-relative path, forward slashes
	pkg     string // package directory of rel ("." for root files)
	content []byte // raw bytes, shared across consumers - never mutate
}

// rootIndex is the lazily built index of one root; the self-tests of the
// gates walk their own temp fixture trees, so the cache is per root.
type rootIndex struct {
	once  sync.Once
	files []sourceFile
	err   error
}

var sourceIndexes sync.Map // root string -> *rootIndex

// goSourceIndex walks root once per process and returns every .go file below
// it, unfiltered. The ratchet walkers used to run ~10 separate tree walks
// with a full re-read of every file per check; those walks were the bulk of
// this package's CPU (profiled: 33% WalkDir, 21% walkGoFilesRaw). Each
// consumer applies its own exclusions on the index, so per-check semantics
// stay exactly as they were.
func goSourceIndex(root string) ([]sourceFile, error) {
	entry, _ := sourceIndexes.LoadOrStore(root, &rootIndex{})
	idx := entry.(*rootIndex)
	idx.once.Do(func() {
		idx.err = filepath.WalkDir(root, func(p string, d fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if d.IsDir() || !strings.HasSuffix(p, ".go") {
				return nil
			}
			rel, err := filepath.Rel(root, p)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			content, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			idx.files = append(idx.files, sourceFile{rel: rel, pkg: path.Dir(rel), content: content})
			return nil
		})
	})
	return idx.files, idx.err
}

func TestGoSourceIndexReportsWalkErrors(t *testing.T) {
	t.Parallel()

	_, err := goSourceIndex(filepath.Join(t.TempDir(), "missing"))
	if err == nil {
		t.Fatal("goSourceIndex succeeded for a missing root")
	}
}
