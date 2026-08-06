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
	for _, segments := range [][]string{
		{".."},
		{"staff-documents", "..", "etc"},
		{"staff-documents", "a/b"},
		{"staff-documents", ""},
		{},
	} {
		if _, err := Key(segments...); !errors.Is(err, ErrInvalidKey) {
			t.Errorf("Key(%v) = nil error, want ErrInvalidKey", segments)
		}
	}
}

func TestTenantKeySeparatesTenants(t *testing.T) {
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
	backend := NewLocal(t.TempDir())
	if _, err := backend.Open(context.Background(), "nope.pdf"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Open missing = %v, want ErrNotFound", err)
	}
	if _, err := backend.Stat(context.Background(), "nope.pdf"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Stat missing = %v, want ErrNotFound", err)
	}
}
