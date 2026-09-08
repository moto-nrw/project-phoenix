package http

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/emergencysnapshot"
)

type fakeQuery struct {
	file emergencysnapshot.File
	err  error
}

func (f fakeQuery) Snapshot(context.Context, time.Time) (emergencysnapshot.Snapshot, error) {
	return emergencysnapshot.Snapshot{}, nil
}

func (f fakeQuery) Document(context.Context, time.Time) (emergencysnapshot.Document, error) {
	return emergencysnapshot.Document{}, nil
}

func (f fakeQuery) Export(context.Context) (emergencysnapshot.File, error) {
	return f.file, f.err
}

func TestExportSnapshotStreamsTheFile(t *testing.T) {
	t.Parallel()
	rs := NewResource(fakeQuery{file: emergencysnapshot.File{Filename: "notfallliste.pdf", ContentType: "application/pdf", Data: []byte("%PDF-1.4")}}, nil)
	rr := httptest.NewRecorder()
	rs.exportSnapshot(rr, httptest.NewRequest(http.MethodPost, "/snapshot/export", nil))

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "application/pdf", rr.Header().Get("Content-Type"))
	assert.Equal(t, `attachment; filename="notfallliste.pdf"`, rr.Header().Get("Content-Disposition"))
	assert.Equal(t, "8", rr.Header().Get("Content-Length"))
	assert.Equal(t, "%PDF-1.4", rr.Body.String())
}

// One owner-query failure surfaces through the existing error contract.
func TestExportSnapshotReportsProjectionFailures(t *testing.T) {
	t.Parallel()
	rs := NewResource(fakeQuery{err: errors.New("owner query failed")}, nil)
	rr := httptest.NewRecorder()
	rs.exportSnapshot(rr, httptest.NewRequest(http.MethodPost, "/snapshot/export", nil))

	require.Equal(t, http.StatusInternalServerError, rr.Code)
	assert.Contains(t, rr.Body.String(), "error")
}

func TestExportSnapshotRejectsMissingProjection(t *testing.T) {
	t.Parallel()
	rs := NewResource(nil, nil)
	rr := httptest.NewRecorder()
	rs.exportSnapshot(rr, httptest.NewRequest(http.MethodPost, "/snapshot/export", nil))

	require.Equal(t, http.StatusInternalServerError, rr.Code)
}
