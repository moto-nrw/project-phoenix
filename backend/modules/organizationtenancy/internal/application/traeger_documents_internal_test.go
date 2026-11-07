package application

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy"
	"github.com/moto-nrw/project-phoenix/modules/organizationtenancy/internal/ports"
)

// documentStoreDouble serves one file of one Träger and the schools its
// releases reach; the other store methods are not part of a download.
type documentStoreDouble struct {
	ports.TraegerDocumentStore
	reached     []int64
	everySchool bool
}

func (documentStoreDouble) DocumentFile(context.Context, int64, int64) (organizationtenancy.TraegerDocumentFile, bool, error) {
	return organizationtenancy.TraegerDocumentFile{FileName: "liste.pdf", ContentType: "application/pdf", Data: []byte("%PDF")}, true, nil
}

func (s documentStoreDouble) ReachedSchools(context.Context, int64, int64) ([]int64, bool, error) {
	return s.reached, s.everySchool, nil
}

// downloadRuntimeDouble records the transactions a download opens.
type downloadRuntimeDouble struct {
	traeger, schools []int64
}

func (r *downloadRuntimeDouble) WithinTraeger(ctx context.Context, organizationID int64, fn func(context.Context) error) error {
	r.traeger = append(r.traeger, organizationID)
	return fn(ctx)
}

func (r *downloadRuntimeDouble) WithinSchool(ctx context.Context, schoolID int64, fn func(context.Context) error) error {
	r.schools = append(r.schools, schoolID)
	return fn(ctx)
}

type traegerSchoolsDouble []int64

func (s traegerSchoolsDouble) TraegerSchools(context.Context, int64) ([]organizationtenancy.TraegerDocumentSchool, error) {
	schools := make([]organizationtenancy.TraegerDocumentSchool, 0, len(s))
	for _, id := range s {
		schools = append(schools, organizationtenancy.TraegerDocumentSchool{ID: id})
	}
	return schools, nil
}

type groupsDouble struct{}

func (groupsDouble) SchoolGroups(context.Context) ([]organizationtenancy.TraegerDocumentGroup, error) {
	return nil, nil
}

type downloadLogDouble struct {
	err     error
	entries []organizationtenancy.TraegerDocumentDownload
}

func (l *downloadLogDouble) RecordDownload(_ context.Context, download organizationtenancy.TraegerDocumentDownload) error {
	if l.err != nil {
		return l.err
	}
	l.entries = append(l.entries, download)
	return nil
}

type schoolAccessDouble struct {
	err     error
	entries []organizationtenancy.TraegerDocumentAccess
}

func (l *schoolAccessDouble) RecordAccess(_ context.Context, access organizationtenancy.TraegerDocumentAccess) error {
	if l.err != nil {
		return l.err
	}
	l.entries = append(l.entries, access)
	return nil
}

type downloadFixture struct {
	service   *TraegerDocuments
	runtime   *downloadRuntimeDouble
	downloads *downloadLogDouble
	schools   *schoolAccessDouble
}

func newDownloadFixture(store documentStoreDouble, current traegerSchoolsDouble) downloadFixture {
	f := downloadFixture{runtime: &downloadRuntimeDouble{}, downloads: &downloadLogDouble{}, schools: &schoolAccessDouble{}}
	f.service = NewTraegerDocuments(store, organizationtenancy.TraegerDocumentPorts{
		Schools: current, Groups: groupsDouble{}, Runtime: f.runtime, Access: f.schools, Downloads: f.downloads,
		Notices: &releaseNoticesDouble{},
	})
	return f
}

var download = organizationtenancy.TraegerDocumentDownload{
	OrganizationID: 3, AccountID: 7, DocumentID: 11, ClientIP: "203.0.113.9", UserAgent: "Testbrowser",
}

// TestTraegerDocumentDownloadIsRecordedOnceAtTheTraeger pins the ledger of
// every download (#4692, ADR 0157): one entry in the Träger transaction with
// the client; no school entry while no release reaches a school, also for a
// Träger without schools.
func TestTraegerDocumentDownloadIsRecordedOnceAtTheTraeger(t *testing.T) {
	t.Parallel()
	for name, current := range map[string]traegerSchoolsDouble{"with schools": {21, 22}, "without schools": {}} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newDownloadFixture(documentStoreDouble{}, current)
			file, err := f.service.File(t.Context(), download)
			require.NoError(t, err)
			assert.Equal(t, "liste.pdf", file.FileName)
			assert.Equal(t, []organizationtenancy.TraegerDocumentDownload{download}, f.downloads.entries)
			assert.Equal(t, []int64{3}, f.runtime.traeger)
			assert.Empty(t, f.schools.entries, "an unreleased document reaches no school")
			assert.Empty(t, f.runtime.schools)
		})
	}
}

// TestTraegerDocumentDownloadIsLoggedOnlyAtReachedCurrentSchools pins the
// school entries: a release for one school logs there; a release for all
// schools logs at every current school; a school that left the Träger never.
func TestTraegerDocumentDownloadIsLoggedOnlyAtReachedCurrentSchools(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		store documentStoreDouble
		want  []int64
	}{
		"one school":            {store: documentStoreDouble{reached: []int64{22}}, want: []int64{22}},
		"all schools":           {store: documentStoreDouble{everySchool: true}, want: []int64{21, 22}},
		"school that left":      {store: documentStoreDouble{reached: []int64{99}}, want: nil},
		"left and current kept": {store: documentStoreDouble{reached: []int64{99, 21}}, want: []int64{21}},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newDownloadFixture(tc.store, traegerSchoolsDouble{21, 22})
			_, err := f.service.File(t.Context(), download)
			require.NoError(t, err)
			assert.Equal(t, tc.want, f.runtime.schools)
			assert.Len(t, f.schools.entries, len(tc.want))
			assert.Len(t, f.downloads.entries, 1)
		})
	}
}

// TestTraegerDocumentDownloadFailsWithoutItsAudit pins "no audit, no
// download": a failed ledger or school entry refuses the file.
func TestTraegerDocumentDownloadFailsWithoutItsAudit(t *testing.T) {
	t.Parallel()
	refused := errors.New("audit unavailable")

	f := newDownloadFixture(documentStoreDouble{}, traegerSchoolsDouble{21})
	f.downloads.err = refused
	file, err := f.service.File(t.Context(), download)
	require.ErrorIs(t, err, refused, "a failed ledger entry fails the download")
	assert.Empty(t, file.Data)

	f = newDownloadFixture(documentStoreDouble{reached: []int64{21}}, traegerSchoolsDouble{21})
	f.schools.err = refused
	file, err = f.service.File(t.Context(), download)
	require.ErrorIs(t, err, refused, "a failed school entry fails the download")
	assert.Empty(t, file.Data)
}
