package test

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
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

// parsedGoFile is one cached parse result. content is kept so a later request
// for the same path with different bytes (self-tests rewriting temp fixtures)
// reparses instead of returning a stale tree.
type parsedGoFile struct {
	once    sync.Once
	content []byte
	file    *ast.File
	err     error
}

var (
	parsedGoFiles sync.Map // path as passed -> *parsedGoFile
	// sharedGoFileSet positions every cached tree. token.FileSet is safe for
	// concurrent use, and positions resolve identically to a per-call set.
	sharedGoFileSet = token.NewFileSet()
)

// parseGoSourceCached parses path once per process with parser.ParseComments
// (a superset of mode 0; comments are not visited by ast.Inspect) and returns
// the shared FileSet with the tree. src follows parser.ParseFile: nil reads
// path, otherwise []byte or string. Callers must treat the *ast.File as
// read-only because other ratchets share it.
func parseGoSourceCached(path string, src any) (*token.FileSet, *ast.File, error) {
	var content []byte
	switch s := src.(type) {
	case nil:
		b, err := os.ReadFile(path) // #nosec G304 -- test scans repo-local source files
		if err != nil {
			return nil, nil, err
		}
		content = b
	case []byte:
		content = s
	case string:
		content = []byte(s)
	default:
		return nil, nil, fmt.Errorf("parseGoSourceCached: unsupported src type %T", src)
	}
	// Keyed by the exact path spelling so positions and parse errors report
	// the filename each caller passed, as a per-call parse would.
	key := path
	for {
		entry, _ := parsedGoFiles.LoadOrStore(key, &parsedGoFile{})
		parsed := entry.(*parsedGoFile)
		parsed.once.Do(func() {
			parsed.content = content
			parsed.file, parsed.err = parser.ParseFile(sharedGoFileSet, path, content, parser.ParseComments)
		})
		if bytes.Equal(parsed.content, content) {
			return sharedGoFileSet, parsed.file, parsed.err
		}
		parsedGoFiles.CompareAndDelete(key, parsed)
	}
}

func TestParseGoSourceCachedReparsesChangedContent(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), "a.go")
	_, first, err := parseGoSourceCached(path, "package a\n")
	if err != nil || first.Name.Name != "a" {
		t.Fatalf("first parse: %v", err)
	}
	_, again, _ := parseGoSourceCached(path, "package a\n")
	if again != first {
		t.Fatal("identical content was reparsed instead of served from cache")
	}
	_, changed, err := parseGoSourceCached(path, "package b\n")
	if err != nil || changed.Name.Name != "b" {
		t.Fatalf("changed content served stale tree: %v", err)
	}
}
