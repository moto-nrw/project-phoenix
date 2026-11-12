package sftp

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"testing"
	"time"

	"github.com/pkg/sftp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type memoryWriteCloser struct {
	bytes.Buffer
	onClose func([]byte)
}

func (w *memoryWriteCloser) Close() error {
	w.onClose(w.Bytes())
	return nil
}

type memoryFileInfo struct{ name string }

func (f memoryFileInfo) Name() string     { return f.name }
func (memoryFileInfo) Size() int64        { return 0 }
func (memoryFileInfo) Mode() fs.FileMode  { return 0 }
func (memoryFileInfo) ModTime() time.Time { return time.Time{} }
func (memoryFileInfo) IsDir() bool        { return false }
func (memoryFileInfo) Sys() any           { return nil }

func TestWriteAtomicallyPreservesExistingFileWhenRenameFails(t *testing.T) {
	t.Parallel()

	const finalPath = "/exports/monat.csv"
	files := map[string][]byte{finalPath: []byte("bisherige fassung")}
	renameErr := errors.New("rename refused")
	operations := remoteFileOperations{
		create: func(name string) (io.WriteCloser, error) {
			return &memoryWriteCloser{onClose: func(data []byte) {
				files[name] = bytes.Clone(data)
			}}, nil
		},
		open: func(name string) (io.ReadCloser, error) {
			data, ok := files[name]
			if !ok {
				return nil, fs.ErrNotExist
			}
			return io.NopCloser(bytes.NewReader(data)), nil
		},
		stat: func(name string) (os.FileInfo, error) {
			if _, ok := files[name]; !ok {
				return nil, fs.ErrNotExist
			}
			return memoryFileInfo{name: name}, nil
		},
		remove: func(name string) error {
			delete(files, name)
			return nil
		},
		rename: func(_, _ string) error {
			t.Fatal("an existing file must be replaced with atomic POSIX rename")
			return nil
		},
		posixRename:  func(_, _ string) error { return renameErr },
		hasExtension: func(string) bool { return true },
	}

	_, err := New().writeAtomically(operations, "/exports", "monat.csv", []byte("korrigiert"))
	require.ErrorIs(t, err, renameErr)
	assert.Equal(t, []byte("bisherige fassung"), files[finalPath])
	assert.Len(t, files, 1, "the failed replacement must clean up only its temporary file")
}

// A rename the counterpart refused with an SFTP status failed for certain;
// one it did not answer leaves the outcome open.
func TestRenameFailureIsUncertainOnlyWithoutAnAnswer(t *testing.T) {
	t.Parallel()
	var uncertain interface{ OutcomeUncertain() bool }
	refused := renameFailure("rename", &sftp.StatusError{Code: 4})
	require.ErrorIs(t, refused, ErrUpload)
	assert.False(t, errors.As(refused, &uncertain), "an SFTP status is an answer")
	lost := renameFailure("rename", io.ErrUnexpectedEOF)
	require.ErrorIs(t, lost, ErrUpload)
	require.ErrorAs(t, lost, &uncertain)
	assert.True(t, uncertain.OutcomeUncertain())
}
