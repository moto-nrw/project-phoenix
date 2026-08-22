package storage

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestKeyRejectsTraversal(t *testing.T) {
	t.Parallel()

	for _, segments := range [][]string{
		{".."},
		{"staff-documents", "..", "etc"},
		{"staff-documents", "a/b"},
		{"staff-documents", ""},
		{"staff-documents", "."},
		{"staff-documents", `a\b`},
		// Not a segment of its own, but still a traversal payload once the
		// operating system normalises the path.
		{"staff-documents", "a..b"},
		{},
	} {
		if _, err := Key(segments...); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("Key(%v) = nil error, want ErrInvalidKey", segments)
		}
	}
}

func TestTenantKeySeparatesTenants(t *testing.T) {
	t.Parallel()

	first, err := TenantKey("student-documents", 7, "a.pdf")
	if err != nil {
		t.Fatalf("TenantKey: %v", err)
	}
	second, err := TenantKey("student-documents", 8, "a.pdf")
	if err != nil {
		t.Fatalf("TenantKey: %v", err)
	}
	if first == second {
		t.Fatal("keys of two tenants collided")
	}
	if first != "student-documents/7/a.pdf" {
		t.Fatalf("unexpected key %q", first)
	}
	if _, err := TenantKey("student-documents", 0, "a.pdf"); !errors.Is(err, ErrInvalidKey) {
		t.Fatal("tenant 0 must be rejected")
	}
}

func TestLocalSaveOpenRemoveRoundTrip(t *testing.T) {
	t.Parallel()

	backend := NewLocal(t.TempDir())
	ctx := context.Background()

	written, err := backend.Save(ctx, "docs/9/file.pdf", bytes.NewReader([]byte("payload")), SaveOptions{})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	if written != int64(len("payload")) {
		t.Fatalf("Save returned %d bytes, want %d", written, len("payload"))
	}

	size, err := backend.Stat(ctx, "docs/9/file.pdf")
	if err != nil || size != int64(len("payload")) {
		t.Fatalf("Stat = %d, %v", size, err)
	}

	object, err := backend.Open(ctx, "docs/9/file.pdf")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	content, err := io.ReadAll(object)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if err := object.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if string(content) != "payload" {
		t.Fatalf("content = %q", content)
	}
	if object.ModTime().IsZero() {
		t.Fatal("ModTime must be set so ServeContent can send Last-Modified")
	}

	if err := backend.Remove(ctx, "docs/9/file.pdf"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := backend.Open(ctx, "docs/9/file.pdf"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Open after Remove = %v, want ErrNotFound", err)
	}
	// Cleanup retries must converge, so removing a gone object stays success.
	if err := backend.Remove(ctx, "docs/9/file.pdf"); err != nil {
		t.Fatalf("second Remove: %v", err)
	}
}

func TestLocalSavePrivateUsesOwnerOnlyPermissions(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	backend := NewLocal(root)

	if _, err := backend.Save(context.Background(), "docs/1/secret.pdf", bytes.NewReader([]byte("x")), SaveOptions{Private: true}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	fileInfo, err := os.Stat(filepath.Join(root, "docs", "1", "secret.pdf"))
	if err != nil {
		t.Fatalf("stat file: %v", err)
	}
	if perm := fileInfo.Mode().Perm(); perm != privateFileMode {
		t.Errorf("file mode = %o, want %o", perm, privateFileMode)
	}

	dirInfo, err := os.Stat(filepath.Join(root, "docs", "1"))
	if err != nil {
		t.Fatalf("stat dir: %v", err)
	}
	if perm := dirInfo.Mode().Perm(); perm != privateDirMode {
		t.Errorf("dir mode = %o, want %o", perm, privateDirMode)
	}
}

func TestLocalRejectsEscapingKey(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	backend := NewLocal(root)
	outside := filepath.Join(filepath.Dir(root), "outside.txt")
	if err := os.WriteFile(outside, []byte("secret"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if _, err := backend.Open(context.Background(), "../"+filepath.Base(outside)); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("Open outside root = %v, want ErrInvalidKey", err)
	}
	if _, err := backend.Save(context.Background(), "../escaped.txt", bytes.NewReader([]byte("x")), SaveOptions{}); !errors.Is(err, ErrInvalidKey) {
		t.Fatal("Save outside root must be rejected")
	}
}

func TestLocalOpenMissingReturnsNotFound(t *testing.T) {
	t.Parallel()

	backend := NewLocal(t.TempDir())
	if _, err := backend.Open(context.Background(), "nope.pdf"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Open missing = %v, want ErrNotFound", err)
	}
	if _, err := backend.Stat(context.Background(), "nope.pdf"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Stat missing = %v, want ErrNotFound", err)
	}
}

// TestLocalSaveHonoursContext covers the deadline the document upload depends
// on. Its cleanup intent becomes eligible three minutes after that deadline,
// so a write allowed to run past it can re-create bytes whose intent the
// scheduler already settled — an object no sweep can find again.
func TestLocalSaveHonoursContext(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	backend := NewLocal(root)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := backend.Save(ctx, "docs/1/late.pdf", bytes.NewReader([]byte("payload")), SaveOptions{Private: true})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Save with a done context = %v, want context.Canceled", err)
	}
	if _, err := os.Stat(filepath.Join(root, "docs", "1", "late.pdf")); !os.IsNotExist(err) {
		t.Fatal("an aborted write must not leave its partial object behind")
	}
}

// TestLocalSaveStopsMidStream proves the abort happens between chunks rather
// than only before the first one: a copy already in flight when the deadline
// passes must stop, not run to completion.
func TestLocalSaveStopsMidStream(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	backend := NewLocal(root)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Cancels itself once the copy has consumed its first chunk.
	source := &cancelAfterFirstRead{
		reader: bytes.NewReader(make([]byte, 1<<20)),
		cancel: cancel,
	}

	_, err := backend.Save(ctx, "docs/1/stalled.pdf", source, SaveOptions{})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Save = %v, want context.Canceled", err)
	}
	if source.reads > 3 {
		t.Fatalf("copy kept reading after cancellation: %d reads", source.reads)
	}
	if _, err := os.Stat(filepath.Join(root, "docs", "1", "stalled.pdf")); !os.IsNotExist(err) {
		t.Fatal("an aborted write must not leave its partial object behind")
	}
}

// TestLocalRefusesAFileAsADirectory covers the failure a stray key produces on
// every operation: the caller must get an error rather than a half-written
// object, because an upload that reports success without storing bytes leaves a
// metadata row pointing at nothing.
func TestLocalRefusesAFileAsADirectory(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	backend := NewLocal(root)
	ctx := context.Background()

	if _, err := backend.Save(ctx, "blocker", bytes.NewReader([]byte("x")), SaveOptions{}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if _, err := backend.Save(ctx, "blocker/child.pdf", bytes.NewReader([]byte("x")), SaveOptions{}); err == nil {
		t.Fatal("Save below a plain file must fail")
	}
	if _, err := backend.Open(ctx, "blocker/child.pdf"); err == nil || errors.Is(err, ErrNotFound) {
		t.Fatalf("Open below a plain file = %v, want a real failure", err)
	}
	if _, err := backend.Stat(ctx, "blocker/child.pdf"); err == nil || errors.Is(err, ErrNotFound) {
		t.Fatalf("Stat below a plain file = %v, want a real failure", err)
	}
}

// TestLocalRefusesADirectoryAsAnObject pins that a key naming a directory never
// reads as an object. Open must say "not found" rather than hand
// http.ServeContent a directory handle.
func TestLocalRefusesADirectoryAsAnObject(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	backend := NewLocal(root)
	ctx := context.Background()

	if _, err := backend.Save(ctx, "docs/9/file.pdf", bytes.NewReader([]byte("x")), SaveOptions{}); err != nil {
		t.Fatalf("seed: %v", err)
	}

	if _, err := backend.Open(ctx, "docs/9"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Open of a directory = %v, want ErrNotFound", err)
	}
	if _, err := backend.Save(ctx, "docs/9", bytes.NewReader([]byte("x")), SaveOptions{}); err == nil {
		t.Fatal("Save over a directory must fail")
	}
	// A non-empty directory cannot be unlinked, and that failure has to reach
	// the caller: cleanup marks an object done only when Remove succeeded.
	if err := backend.Remove(ctx, "docs/9"); err == nil {
		t.Fatal("Remove of a non-empty directory must fail")
	}
}

// TestLocalSaveSurfacesReadFailure separates a broken source from a passed
// deadline. Both abort the write, but only the deadline case may be reported as
// a context error — the upload path decides its cleanup bookkeeping from it.
func TestLocalSaveSurfacesReadFailure(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	backend := NewLocal(root)

	broken := errors.New("connection reset")
	_, err := backend.Save(context.Background(), "docs/1/broken.pdf", failingReader{err: broken}, SaveOptions{})
	if err == nil {
		t.Fatal("Save with a failing source must fail")
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Save = %v, must not be reported as a context error", err)
	}
	if _, statErr := os.Stat(filepath.Join(root, "docs", "1", "broken.pdf")); !os.IsNotExist(statErr) {
		t.Fatal("a failed write must not leave its partial object behind")
	}
}

// TestLocalRemoveAndStatRejectEscapingKey completes the traversal guard: Save
// and Open already refuse to leave the root, and so must the two operations
// that a cleanup sweep uses.
func TestLocalRemoveAndStatRejectEscapingKey(t *testing.T) {
	t.Parallel()

	backend := NewLocal(t.TempDir())
	ctx := context.Background()

	if err := backend.Remove(ctx, "../escaped.txt"); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("Remove outside root = %v, want ErrInvalidKey", err)
	}
	if _, err := backend.Stat(ctx, "/etc/passwd"); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("Stat of an absolute key = %v, want ErrInvalidKey", err)
	}
}

type failingReader struct{ err error }

func (f failingReader) Read([]byte) (int, error) { return 0, f.err }

type cancelAfterFirstRead struct {
	reader *bytes.Reader
	cancel context.CancelFunc
	reads  int
}

func (c *cancelAfterFirstRead) Read(p []byte) (int, error) {
	n, err := c.reader.Read(p)
	c.reads++
	if c.reads == 1 {
		c.cancel()
	}
	return n, err
}
