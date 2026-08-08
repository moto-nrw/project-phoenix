package documents_test

import (
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/models/documents"
)

func validFile() *documents.File {
	f := &documents.File{
		Category:        "attest",
		FilenameDisplay: "Attest.pdf",
		FilenameStored:  "0f5b9d4e-1d3a-4a2f-9a3c-1b2c3d4e5f60.pdf",
		SizeBytes:       2048,
		ContentType:     "application/pdf",
		UploadedBy:      77,
	}
	return f
}

func TestValidateFileAcceptsACompleteRow(t *testing.T) {
	if err := documents.ValidateFile(validFile()); err != nil {
		t.Fatalf("ValidateFile = %v, want nil", err)
	}
	// Zero bytes is a degenerate but storable file; only a negative size is a
	// bug, and it must not reach a column the download handler trusts.
	zero := validFile()
	zero.SizeBytes = 0
	if err := documents.ValidateFile(zero); err != nil {
		t.Fatalf("ValidateFile with a zero-byte file = %v, want nil", err)
	}
}

func TestValidateFileRejectsIncompleteRows(t *testing.T) {
	cases := map[string]func(*documents.File){
		"missing category":         func(f *documents.File) { f.Category = "  " },
		"missing display filename": func(f *documents.File) { f.FilenameDisplay = "" },
		// Without the stored name nothing can ever find the bytes again, so
		// the row would be an unreclaimable orphan from the moment it is written.
		"missing stored filename": func(f *documents.File) { f.FilenameStored = " " },
		"negative size":           func(f *documents.File) { f.SizeBytes = -1 },
		"missing content type":    func(f *documents.File) { f.ContentType = "" },
		"missing uploader":        func(f *documents.File) { f.UploadedBy = 0 },
	}
	for name, break_ := range cases {
		t.Run(name, func(t *testing.T) {
			f := validFile()
			break_(f)
			if err := documents.ValidateFile(f); err == nil {
				t.Fatalf("ValidateFile(%s) = nil, want an error", name)
			}
		})
	}
}

func TestFileAccessors(t *testing.T) {
	f := validFile()
	if f.GetCategory() != "attest" {
		t.Errorf("GetCategory = %q", f.GetCategory())
	}
	if f.GetFilenameStored() != f.FilenameStored {
		t.Errorf("GetFilenameStored = %q", f.GetFilenameStored())
	}
	if f.GetFilenameDisplay() != "Attest.pdf" {
		t.Errorf("GetFilenameDisplay = %q", f.GetFilenameDisplay())
	}
}

// TestFileSoftDeleteIsNotFileDeletion pins the two-stage lifecycle the cleanup
// scheduler depends on: a soft-deleted row whose bytes are still on disk must
// keep reporting that its file needs removal, otherwise the sweep skips it and
// the object stays forever.
func TestFileSoftDeleteIsNotFileDeletion(t *testing.T) {
	f := validFile()
	if f.IsFileDeleted() {
		t.Fatal("a fresh row must not claim its bytes are gone")
	}

	at := time.Now()
	f.MarkSoftDeleted(at, 42)
	if f.DeletedAt == nil || !f.DeletedAt.Equal(at) {
		t.Fatalf("DeletedAt = %v, want %v", f.DeletedAt, at)
	}
	if f.DeletedBy == nil || *f.DeletedBy != 42 {
		t.Fatalf("DeletedBy = %v, want 42", f.DeletedBy)
	}
	if f.IsFileDeleted() {
		t.Fatal("soft-deleting the row must not mark the bytes as removed")
	}

	f.FileDeletedAt = &at
	if !f.IsFileDeleted() {
		t.Fatal("a stamped file_deleted_at must retire the object from cleanup")
	}
}

func TestFileCleanupAccessors(t *testing.T) {
	cleanup := &documents.FileCleanup{OwnerID: 9, FilenameStored: "orphan.pdf"}
	if cleanup.GetOwnerID() != 9 {
		t.Errorf("GetOwnerID = %d", cleanup.GetOwnerID())
	}
	if cleanup.GetFilenameStored() != "orphan.pdf" {
		t.Errorf("GetFilenameStored = %q", cleanup.GetFilenameStored())
	}
}
