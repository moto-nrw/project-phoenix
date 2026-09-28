package compose

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// planningTrackOwnerStub is an in-memory planning-track capability for the
// administration's validation and error-propagation branches.
type planningTrackOwnerStub struct {
	timetable.PlanningTrackCapability
	tracks               map[int64]timetable.PlanningTrack
	nextID               int64
	createErr            error
	findErr              error
	findForShareErr      error
	listErr              error
	updateActiveErr      error
	updateActiveResult   *bool
	archiveErr           error
	reorderErr           error
	restoreErr           error
	lastListFilter       timetable.PlanningTrackFilter
	reorderedIDs         []int64
	archivedAtOnArchived *time.Time
}

func newPlanningTrackOwnerStub(tracks ...timetable.PlanningTrack) *planningTrackOwnerStub {
	stub := &planningTrackOwnerStub{tracks: map[int64]timetable.PlanningTrack{}, nextID: 1}
	for _, track := range tracks {
		stub.tracks[track.ID] = track
		if track.ID >= stub.nextID {
			stub.nextID = track.ID + 1
		}
	}
	return stub
}

func (s *planningTrackOwnerStub) FindPlanningTrack(_ context.Context, id int64) (timetable.PlanningTrack, error) {
	if s.findErr != nil {
		return timetable.PlanningTrack{}, s.findErr
	}
	track, ok := s.tracks[id]
	if !ok {
		return timetable.PlanningTrack{}, timetable.ErrPlanningTrackNotFound
	}
	return track, nil
}

func (s *planningTrackOwnerStub) FindPlanningTrackForShare(ctx context.Context, id int64) (timetable.PlanningTrack, error) {
	if s.findForShareErr != nil {
		return timetable.PlanningTrack{}, s.findForShareErr
	}
	return s.FindPlanningTrack(ctx, id)
}

func (s *planningTrackOwnerStub) ListPlanningTracks(_ context.Context, filter timetable.PlanningTrackFilter) ([]timetable.PlanningTrack, error) {
	s.lastListFilter = filter
	if s.listErr != nil {
		return nil, s.listErr
	}
	result := make([]timetable.PlanningTrack, 0, len(s.tracks))
	for _, track := range s.tracks {
		result = append(result, track)
	}
	return result, nil
}

func (s *planningTrackOwnerStub) CreatePlanningTrack(_ context.Context, input timetable.PlanningTrackInput) (timetable.PlanningTrack, error) {
	if s.createErr != nil {
		return timetable.PlanningTrack{}, s.createErr
	}
	track := timetable.PlanningTrack{ID: s.nextID, Name: input.Name, Color: input.Color, SortOrder: input.SortOrder}
	s.nextID++
	s.tracks[track.ID] = track
	return track, nil
}

func (s *planningTrackOwnerStub) UpdateActivePlanningTrack(_ context.Context, id int64, input timetable.PlanningTrackInput) (timetable.PlanningTrack, bool, error) {
	if s.updateActiveErr != nil {
		return timetable.PlanningTrack{}, false, s.updateActiveErr
	}
	if s.updateActiveResult != nil {
		return timetable.PlanningTrack{}, *s.updateActiveResult, nil
	}
	track, ok := s.tracks[id]
	if !ok || track.IsArchived() {
		return timetable.PlanningTrack{}, false, nil
	}
	track.Name, track.Color, track.SortOrder = input.Name, input.Color, input.SortOrder
	s.tracks[id] = track
	return track, true, nil
}

func (s *planningTrackOwnerStub) SetPlanningTrackArchivedAt(_ context.Context, id int64, value *time.Time) (timetable.PlanningTrack, bool, error) {
	if s.archiveErr != nil {
		return timetable.PlanningTrack{}, false, s.archiveErr
	}
	track := s.tracks[id]
	track.ArchivedAt = value
	s.archivedAtOnArchived = value
	s.tracks[id] = track
	return track, true, nil
}

func (s *planningTrackOwnerStub) ReorderPlanningTracks(_ context.Context, ids []int64) error {
	s.reorderedIDs = ids
	return s.reorderErr
}

func (s *planningTrackOwnerStub) RestorePlanningTrackAtEnd(_ context.Context, id int64) (timetable.PlanningTrack, bool, error) {
	if s.restoreErr != nil {
		return timetable.PlanningTrack{}, false, s.restoreErr
	}
	maxOrder := -1
	for _, candidate := range s.tracks {
		if !candidate.IsArchived() && candidate.SortOrder > maxOrder {
			maxOrder = candidate.SortOrder
		}
	}
	track := s.tracks[id]
	track.ArchivedAt = nil
	track.SortOrder = maxOrder + 1
	s.tracks[id] = track
	return track, true, nil
}

func archivedPlanningTrack(id int64, name string) timetable.PlanningTrack {
	archivedAt := time.Now()
	return timetable.PlanningTrack{ID: id, Name: name, Color: "#83CD2D", ArchivedAt: &archivedAt}
}

func TestPlanningTrackAdministrationValidatesDrafts(t *testing.T) {
	t.Parallel()
	owner := newPlanningTrackOwnerStub()
	admin := NewPlanningTrackAdministration(owner, nil)
	ctx := context.Background()

	created, err := admin.AddPlanningTrack(ctx, timetable.PlanningTrackDraft{Name: "  Mittag  ", Color: "#F78C10", SortOrder: 2})
	require.NoError(t, err)
	assert.Equal(t, "Mittag", created.Name)
	assert.Equal(t, "#F78C10", created.Color)
	assert.Equal(t, 2, created.SortOrder)

	_, err = admin.EditPlanningTrack(ctx, created.ID, timetable.PlanningTrackDraft{Name: "Mittag", Color: "orange", SortOrder: 2})
	require.ErrorIs(t, err, timetable.ErrInvalidPlanningTrack)
	assert.EqualError(t, err, "invalid planning track: planning track color must use #RRGGBB")

	for draft, reason := range map[timetable.PlanningTrackDraft]string{
		{Name: " ", Color: "#F78C10"}:                         "planning track name is required",
		{Name: string(make([]byte, 101)), Color: "#F78C10"}:   "planning track name cannot exceed 100 characters",
		{Name: "Nord", Color: "#F78C10", SortOrder: -1}:       "planning track sort order cannot be negative",
		{Name: "", Color: "blue", SortOrder: -1}:              "planning track name is required",
		{Name: "Nord", Color: "#F78C1", SortOrder: 0}:         "planning track color must use #RRGGBB",
		{Name: "Nord", Color: "F78C10", SortOrder: 0}:         "planning track color must use #RRGGBB",
		{Name: "Nord", Color: "#F78C10x", SortOrder: 0}:       "planning track color must use #RRGGBB",
		{Name: "Nord", Color: "#GGGGGG", SortOrder: 0}:        "planning track color must use #RRGGBB",
		{Name: "Nord", Color: "#f78c10", SortOrder: -2}:       "planning track sort order cannot be negative",
		{Name: "\tNord\n", Color: "#123", SortOrder: 0}:       "planning track color must use #RRGGBB",
		{Name: "Süd", Color: "#83CD2D ", SortOrder: 1}:        "planning track color must use #RRGGBB",
		{Name: "Ost", Color: "", SortOrder: 1}:                "planning track color must use #RRGGBB",
		{Name: "West", Color: "#ABCDEF", SortOrder: -100}:     "planning track sort order cannot be negative",
		{Name: "Mitte", Color: "rgb(1,2,3)", SortOrder: 0}:    "planning track color must use #RRGGBB",
		{Name: "   Leer   ", Color: "#ABCDEF", SortOrder: -1}: "planning track sort order cannot be negative",
	} {
		_, err := admin.AddPlanningTrack(ctx, draft)
		require.ErrorIs(t, err, timetable.ErrInvalidPlanningTrack, "draft %+v", draft)
		assert.EqualError(t, err, "invalid planning track: "+reason, "draft %+v", draft)
	}
}

func TestPlanningTrackAdministrationMissingTracks(t *testing.T) {
	t.Parallel()
	admin := NewPlanningTrackAdministration(newPlanningTrackOwnerStub(), nil)
	ctx := context.Background()
	invalid := timetable.PlanningTrackDraft{Name: "", Color: "blue", SortOrder: -1}

	_, err := admin.EditPlanningTrack(ctx, 0, invalid)
	require.ErrorIs(t, err, timetable.ErrPlanningTrackNotFound)
	_, err = admin.ArchivePlanningTrack(ctx, 0)
	require.ErrorIs(t, err, timetable.ErrPlanningTrackNotFound)
	_, err = admin.RestorePlanningTrack(ctx, 0)
	require.ErrorIs(t, err, timetable.ErrPlanningTrackNotFound)
	_, err = admin.GetPlanningTrack(ctx, time.Now().UnixNano())
	require.ErrorIs(t, err, timetable.ErrPlanningTrackNotFound)
}

func TestPlanningTrackAdministrationListAndGet(t *testing.T) {
	t.Parallel()
	track := timetable.PlanningTrack{ID: time.Now().UnixNano(), Name: "Früh", Color: "#5080D8"}
	owner := newPlanningTrackOwnerStub(track)
	admin := NewPlanningTrackAdministration(owner, nil)
	ctx := context.Background()

	tracks, err := admin.ListAllPlanningTracks(ctx)
	require.NoError(t, err)
	assert.Equal(t, []timetable.PlanningTrack{track}, tracks)
	assert.Equal(t, timetable.PlanningTrackFilter{Ordered: true}, owner.lastListFilter)
	found, err := admin.GetPlanningTrack(ctx, track.ID)
	require.NoError(t, err)
	assert.Equal(t, track, found)

	wantErr := errors.New("database unavailable")
	owner.listErr = wantErr
	_, err = admin.ListAllPlanningTracks(ctx)
	require.ErrorIs(t, err, wantErr)
	owner.findErr = wantErr
	_, err = admin.GetPlanningTrack(ctx, track.ID)
	require.ErrorIs(t, err, wantErr)
}

func TestPlanningTrackAdministrationWriteFailures(t *testing.T) {
	t.Parallel()
	wantErr := errors.New("database unavailable")
	owner := newPlanningTrackOwnerStub()
	admin := NewPlanningTrackAdministration(owner, nil)
	ctx := context.Background()
	draft := timetable.PlanningTrackDraft{Name: "Nord", Color: "#5080D8", SortOrder: 1}

	owner.createErr = wantErr
	_, err := admin.AddPlanningTrack(ctx, draft)
	require.ErrorIs(t, err, wantErr)
	owner.createErr = timetable.ErrPlanningTrackNameExists
	_, err = admin.AddPlanningTrack(ctx, draft)
	require.ErrorIs(t, err, timetable.ErrPlanningTrackNameTaken)
	owner.createErr = nil
	created, err := admin.AddPlanningTrack(ctx, draft)
	require.NoError(t, err)

	updated, err := admin.EditPlanningTrack(ctx, created.ID, timetable.PlanningTrackDraft{Name: "Süd", Color: "#83CD2D", SortOrder: 2})
	require.NoError(t, err)
	assert.Equal(t, "Süd", updated.Name)
	assert.Equal(t, 2, updated.SortOrder)

	owner.updateActiveErr = wantErr
	_, err = admin.EditPlanningTrack(ctx, created.ID, draft)
	require.ErrorIs(t, err, wantErr)
	owner.updateActiveErr = timetable.ErrPlanningTrackNameExists
	_, err = admin.EditPlanningTrack(ctx, created.ID, draft)
	require.ErrorIs(t, err, timetable.ErrPlanningTrackNameTaken)
	owner.updateActiveErr = nil
	notUpdated := false
	owner.updateActiveResult = &notUpdated
	_, err = admin.EditPlanningTrack(ctx, created.ID, draft)
	require.ErrorIs(t, err, timetable.ErrPlanningTrackArchived)

	archived := archivedPlanningTrack(created.ID+1, "Alt")
	archivedAdmin := NewPlanningTrackAdministration(newPlanningTrackOwnerStub(archived), nil)
	_, err = archivedAdmin.EditPlanningTrack(ctx, archived.ID, draft)
	require.ErrorIs(t, err, timetable.ErrPlanningTrackArchived)
}

func TestPlanningTrackAdministrationArchiveAndRestore(t *testing.T) {
	t.Parallel()
	base := time.Now().UnixNano()
	remaining := timetable.PlanningTrack{ID: base, Name: "Mittag", Color: "#F78C10", SortOrder: 0}
	track := timetable.PlanningTrack{ID: base + 1, Name: "Früh", Color: "#83CD2D", SortOrder: 0}
	owner := newPlanningTrackOwnerStub(remaining, track)
	admin := NewPlanningTrackAdministration(owner, nil)
	ctx := context.Background()

	archived, err := admin.ArchivePlanningTrack(ctx, track.ID)
	require.NoError(t, err)
	require.NotNil(t, archived.ArchivedAt)
	assert.Equal(t, owner.archivedAtOnArchived, archived.ArchivedAt)
	restored, err := admin.RestorePlanningTrack(ctx, track.ID)
	require.NoError(t, err)
	assert.Nil(t, restored.ArchivedAt)
	assert.Equal(t, 1, restored.SortOrder)
}

func TestPlanningTrackAdministrationArchiveRestoreEdges(t *testing.T) {
	t.Parallel()
	base := time.Now().UnixNano()
	active := timetable.PlanningTrack{ID: base, Name: "Früh", Color: "#5080D8"}
	archived := archivedPlanningTrack(base+1, "Alt")
	owner := newPlanningTrackOwnerStub(active, archived)
	admin := NewPlanningTrackAdministration(owner, nil)
	ctx := context.Background()

	unchanged, err := admin.ArchivePlanningTrack(ctx, archived.ID)
	require.NoError(t, err)
	assert.Equal(t, archived, unchanged)
	unchanged, err = admin.RestorePlanningTrack(ctx, active.ID)
	require.NoError(t, err)
	assert.Equal(t, active, unchanged)

	wantErr := errors.New("database unavailable")
	owner.archiveErr = wantErr
	_, err = admin.ArchivePlanningTrack(ctx, active.ID)
	require.ErrorIs(t, err, wantErr)
	owner.restoreErr = wantErr
	_, err = admin.RestorePlanningTrack(ctx, archived.ID)
	require.ErrorIs(t, err, wantErr)
	owner.restoreErr = timetable.ErrPlanningTrackNameExists
	_, err = admin.RestorePlanningTrack(ctx, archived.ID)
	require.ErrorIs(t, err, timetable.ErrPlanningTrackNameTaken)
}

func TestPlanningTrackAdministrationValidatesAssignments(t *testing.T) {
	t.Parallel()
	base := time.Now().UnixNano()
	active := timetable.PlanningTrack{ID: base, Name: "Früh", Color: "#5080D8"}
	archived := archivedPlanningTrack(base+1, "Alt")
	owner := newPlanningTrackOwnerStub(active, archived)
	admin := NewPlanningTrackAdministration(owner, nil)
	ctx := context.Background()

	require.NoError(t, admin.ValidatePlanningTrackAssignment(ctx, nil, nil))
	zero := int64(0)
	require.ErrorIs(t, admin.ValidatePlanningTrackAssignment(ctx, &zero, nil), timetable.ErrPlanningTrackNotFound)
	require.NoError(t, admin.ValidatePlanningTrackAssignment(ctx, &active.ID, nil))
	require.ErrorIs(t, admin.ValidatePlanningTrackAssignment(ctx, &archived.ID, nil), timetable.ErrPlanningTrackArchived)
	require.ErrorIs(t, admin.ValidatePlanningTrackAssignment(ctx, &archived.ID, &active.ID), timetable.ErrPlanningTrackArchived)
	require.NoError(t, admin.ValidatePlanningTrackAssignment(ctx, &archived.ID, &archived.ID),
		"a template keeps the archived track it already has")
	missing := base + 2
	require.ErrorIs(t, admin.ValidatePlanningTrackAssignment(ctx, &missing, nil), timetable.ErrPlanningTrackNotFound)

	wantErr := errors.New("database unavailable")
	owner.findForShareErr = wantErr
	require.ErrorIs(t, admin.ValidatePlanningTrackAssignment(ctx, &active.ID, nil), wantErr)
}

func TestPlanningTrackAdministrationReorder(t *testing.T) {
	t.Parallel()
	owner := newPlanningTrackOwnerStub()
	one := time.Now().UnixNano()

	unwired := NewPlanningTrackAdministration(owner, nil)
	require.ErrorIs(t, unwired.OrderPlanningTracks(context.Background(), []int64{0}), timetable.ErrInvalidPlanningTrack)
	require.ErrorIs(t, unwired.OrderPlanningTracks(context.Background(), []int64{one, one}), timetable.ErrInvalidPlanningTrack)
	require.EqualError(t, unwired.OrderPlanningTracks(context.Background(), []int64{one}), "planning track service is not configured")

	db := testpkg.SetupTestDB(t)
	ctx := testpkg.Ctx(t)
	admin := NewPlanningTrackAdministration(owner, db)
	require.NoError(t, admin.OrderPlanningTracks(ctx, []int64{one, one + 1}))
	assert.Equal(t, []int64{one, one + 1}, owner.reorderedIDs)

	wantErr := errors.New("reorder failed")
	owner.reorderErr = wantErr
	require.ErrorIs(t, admin.OrderPlanningTracks(ctx, []int64{one}), wantErr)
	owner.reorderErr = timetable.ErrPlanningTrackNotFound
	err := admin.OrderPlanningTracks(ctx, []int64{one})
	require.ErrorIs(t, err, timetable.ErrPlanningTrackNotFound)
	assert.EqualError(t, err, timetable.ErrPlanningTrackNotFound.Error())
}

func TestPlanningTrackAdministrationNameConflictAndArchiveLifecycle(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	admin := NewPlanningTrackAdministration(buildModule(t, db), db)
	ctx := testpkg.Ctx(t)
	draft := timetable.PlanningTrackDraft{Name: "Nord", Color: "#5080D8", SortOrder: 0}

	first, err := admin.AddPlanningTrack(ctx, draft)
	require.NoError(t, err)
	_, err = admin.AddPlanningTrack(ctx, timetable.PlanningTrackDraft{Name: " nord ", Color: "#83CD2D", SortOrder: 1})
	require.ErrorIs(t, err, timetable.ErrPlanningTrackNameTaken)
	_, err = admin.ArchivePlanningTrack(ctx, first.ID)
	require.NoError(t, err)
	second, err := admin.AddPlanningTrack(ctx, draft)
	require.NoError(t, err)
	third, err := admin.AddPlanningTrack(ctx, timetable.PlanningTrackDraft{Name: "Süd", Color: "#F78C10", SortOrder: 1})
	require.NoError(t, err)
	_, err = admin.EditPlanningTrack(ctx, third.ID, timetable.PlanningTrackDraft{Name: "NORD", Color: "#83CD2D", SortOrder: 1})
	require.ErrorIs(t, err, timetable.ErrPlanningTrackNameTaken)
	_, err = admin.RestorePlanningTrack(ctx, first.ID)
	require.ErrorIs(t, err, timetable.ErrPlanningTrackNameTaken)
	assert.NotEqual(t, first.ID, second.ID)
}

// TestPlanningTrackAdministrationAgainstOwner runs the editor over the real
// owner: names stay unique among active tracks, an archived track is frozen,
// a restored one moves to the end and a template may keep its archived track.
func TestPlanningTrackAdministrationAgainstOwner(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	admin := NewPlanningTrackAdministration(buildModule(t, db), db)
	ctx := testpkg.Ctx(t)

	early, err := admin.AddPlanningTrack(ctx, timetable.PlanningTrackDraft{Name: "Früh", Color: "#5080D8", SortOrder: 0})
	require.NoError(t, err)
	late, err := admin.AddPlanningTrack(ctx, timetable.PlanningTrackDraft{Name: "Spät", Color: "#F78C10", SortOrder: 1})
	require.NoError(t, err)
	_, err = admin.AddPlanningTrack(ctx, timetable.PlanningTrackDraft{Name: " früh ", Color: "#83CD2D", SortOrder: 2})
	require.ErrorIs(t, err, timetable.ErrPlanningTrackNameTaken)
	_, err = admin.EditPlanningTrack(ctx, late.ID, timetable.PlanningTrackDraft{Name: "Früh", Color: "#F78C10", SortOrder: 1})
	require.ErrorIs(t, err, timetable.ErrPlanningTrackNameTaken)

	archived, err := admin.ArchivePlanningTrack(ctx, early.ID)
	require.NoError(t, err)
	require.NotNil(t, archived.ArchivedAt)
	_, err = admin.EditPlanningTrack(ctx, early.ID, timetable.PlanningTrackDraft{Name: "Früher", Color: "#5080D8", SortOrder: 0})
	require.ErrorIs(t, err, timetable.ErrPlanningTrackArchived)
	require.ErrorIs(t, admin.ValidatePlanningTrackAssignment(ctx, &early.ID, nil), timetable.ErrPlanningTrackArchived)
	require.NoError(t, admin.ValidatePlanningTrackAssignment(ctx, &early.ID, &early.ID))

	replacement, err := admin.AddPlanningTrack(ctx, timetable.PlanningTrackDraft{Name: "Früh", Color: "#83CD2D", SortOrder: 2})
	require.NoError(t, err, "an archived track frees its name")
	_, err = admin.RestorePlanningTrack(ctx, early.ID)
	require.ErrorIs(t, err, timetable.ErrPlanningTrackNameTaken)
	renamed, err := admin.EditPlanningTrack(ctx, replacement.ID, timetable.PlanningTrackDraft{Name: "Früh neu", Color: "#83CD2D", SortOrder: 2})
	require.NoError(t, err)
	assert.Equal(t, "Früh neu", renamed.Name)
	restored, err := admin.RestorePlanningTrack(ctx, early.ID)
	require.NoError(t, err)
	assert.Nil(t, restored.ArchivedAt)
	assert.Equal(t, 3, restored.SortOrder)

	require.NoError(t, admin.OrderPlanningTracks(ctx, []int64{restored.ID, replacement.ID, late.ID}))
	listed, err := admin.ListAllPlanningTracks(ctx)
	require.NoError(t, err)
	assert.Equal(t, []int64{restored.ID, replacement.ID, late.ID}, planningTrackIDs(listed))
}
