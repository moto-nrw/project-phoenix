package schedule

import (
	"context"
	"errors"
	"testing"
	"time"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	model "github.com/moto-nrw/project-phoenix/models/schedule"
)

type planningTrackRepoStub struct {
	tracks               map[int64]*model.PlanningTrack
	nextID               int64
	createErr            error
	findErr              error
	findForShareErr      error
	listErr              error
	updateIfActiveErr    error
	updateIfActiveResult *bool
	updateColumnsErr     error
	updateSortOrdersErr  error
	restoreAtEndErr      error
}

func newPlanningTrackRepoStub(tracks ...*model.PlanningTrack) *planningTrackRepoStub {
	repo := &planningTrackRepoStub{tracks: make(map[int64]*model.PlanningTrack), nextID: 1}
	for _, track := range tracks {
		repo.tracks[track.ID] = track
		if track.ID >= repo.nextID {
			repo.nextID = track.ID + 1
		}
	}
	return repo
}

func planningTrackWithID(id int64, name, color string) *model.PlanningTrack {
	track := &model.PlanningTrack{Name: name, Color: color}
	track.ID = id
	return track
}

func (r *planningTrackRepoStub) Create(_ context.Context, track *model.PlanningTrack) error {
	if r.createErr != nil {
		return r.createErr
	}
	track.ID = r.nextID
	r.nextID++
	r.tracks[track.ID] = track
	return nil
}

func (r *planningTrackRepoStub) FindByID(_ context.Context, id any) (*model.PlanningTrack, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	track, ok := r.tracks[id.(int64)]
	if !ok {
		return nil, &modelBase.DatabaseError{Op: "find planning track", Err: modelBase.ErrNotFound}
	}
	return track, nil
}

func (r *planningTrackRepoStub) FindByIDs(_ context.Context, ids []int64) ([]*model.PlanningTrack, error) {
	if r.findErr != nil {
		return nil, r.findErr
	}
	result := make([]*model.PlanningTrack, 0, len(ids))
	for _, id := range ids {
		if track, ok := r.tracks[id]; ok {
			result = append(result, track)
		}
	}
	return result, nil
}

func (r *planningTrackRepoStub) Update(_ context.Context, track *model.PlanningTrack) error {
	r.tracks[track.ID] = track
	return nil
}

func (r *planningTrackRepoStub) Delete(_ context.Context, id any) error {
	delete(r.tracks, id.(int64))
	return nil
}

func (r *planningTrackRepoStub) List(context.Context, map[string]any) ([]*model.PlanningTrack, error) {
	return r.ListAll(context.Background())
}

func (r *planningTrackRepoStub) ListAll(context.Context) ([]*model.PlanningTrack, error) {
	if r.listErr != nil {
		return nil, r.listErr
	}
	result := make([]*model.PlanningTrack, 0, len(r.tracks))
	for _, track := range r.tracks {
		result = append(result, track)
	}
	return result, nil
}

func (r *planningTrackRepoStub) FindByIDForShare(ctx context.Context, id int64) (*model.PlanningTrack, error) {
	if r.findForShareErr != nil {
		return nil, r.findForShareErr
	}
	return r.FindByID(ctx, id)
}

func (r *planningTrackRepoStub) UpdateIfActive(_ context.Context, track *model.PlanningTrack) (bool, error) {
	if r.updateIfActiveErr != nil {
		return false, r.updateIfActiveErr
	}
	if r.updateIfActiveResult != nil {
		return *r.updateIfActiveResult, nil
	}
	stored, ok := r.tracks[track.ID]
	if !ok || stored.ArchivedAt != nil {
		return false, nil
	}
	r.tracks[track.ID] = track
	return true, nil
}

func (r *planningTrackRepoStub) UpdateSortOrders(context.Context, []int64) error {
	return r.updateSortOrdersErr
}

func (r *planningTrackRepoStub) RestoreAtEnd(_ context.Context, track *model.PlanningTrack) (bool, error) {
	if r.restoreAtEndErr != nil {
		return false, r.restoreAtEndErr
	}
	maxOrder := -1
	for _, candidate := range r.tracks {
		if !candidate.IsArchived() && candidate.SortOrder > maxOrder {
			maxOrder = candidate.SortOrder
		}
	}
	track.ArchivedAt = nil
	track.SortOrder = maxOrder + 1
	r.tracks[track.ID] = track
	return true, nil
}

func (r *planningTrackRepoStub) UpdateColumns(_ context.Context, track *model.PlanningTrack, _ ...string) (int64, error) {
	if r.updateColumnsErr != nil {
		return 0, r.updateColumnsErr
	}
	r.tracks[track.ID] = track
	return 1, nil
}

func TestPlanningTrackServiceCreateAndUpdateValidation(t *testing.T) {
	t.Parallel()

	repo := newPlanningTrackRepoStub()
	service := NewPlanningTrackService(repo, nil)

	created, err := service.CreatePlanningTrack(context.Background(), PlanningTrackInput{
		Name: "  Mittag  ", Color: "#F78C10", SortOrder: 2,
	})
	if err != nil {
		t.Fatalf("create planning track: %v", err)
	}
	if created.Name != "Mittag" || created.Color != "#F78C10" || created.SortOrder != 2 {
		t.Fatalf("unexpected created track: %#v", created)
	}

	_, err = service.UpdatePlanningTrack(context.Background(), created.ID, PlanningTrackInput{
		Name: "Mittag", Color: "orange", SortOrder: 2,
	})
	if !errors.Is(err, ErrPlanningTrackInvalid) {
		t.Fatalf("expected invalid planning track error, got %v", err)
	}
}

func TestPlanningTrackServiceListAndGet(t *testing.T) {
	t.Parallel()

	track := planningTrackWithID(1, "Früh", "#5080D8")
	repo := newPlanningTrackRepoStub(track)
	service := NewPlanningTrackService(repo, nil)

	tracks, err := service.ListPlanningTracks(context.Background())
	if err != nil || len(tracks) != 1 {
		t.Fatalf("list planning tracks: tracks=%#v err=%v", tracks, err)
	}
	found, err := service.GetPlanningTrack(context.Background(), track.ID)
	if err != nil || found != track {
		t.Fatalf("get planning track: track=%#v err=%v", found, err)
	}
	for _, id := range []int64{0, track.ID + 1} {
		if _, err := service.GetPlanningTrack(context.Background(), id); !errors.Is(err, ErrPlanningTrackNotFound) {
			t.Fatalf("get planning track %d: expected not found, got %v", id, err)
		}
	}

	wantErr := errors.New("database unavailable")
	repo.listErr = wantErr
	if _, err := service.ListPlanningTracks(context.Background()); !errors.Is(err, wantErr) {
		t.Fatalf("list planning tracks: expected %v, got %v", wantErr, err)
	}
	repo.listErr = nil
	repo.findErr = wantErr
	if _, err := service.GetPlanningTrack(context.Background(), track.ID); !errors.Is(err, wantErr) {
		t.Fatalf("get planning track: expected %v, got %v", wantErr, err)
	}
}

func TestPlanningTrackServiceCreateAndUpdateFailures(t *testing.T) {
	t.Parallel()

	wantErr := errors.New("database unavailable")
	repo := newPlanningTrackRepoStub()
	service := NewPlanningTrackService(repo, nil)
	input := PlanningTrackInput{Name: "Nord", Color: "#5080D8", SortOrder: 1}

	repo.createErr = wantErr
	if _, err := service.CreatePlanningTrack(context.Background(), input); !errors.Is(err, wantErr) {
		t.Fatalf("create planning track: expected %v, got %v", wantErr, err)
	}
	repo.createErr = nil
	created, err := service.CreatePlanningTrack(context.Background(), input)
	if err != nil {
		t.Fatalf("create planning track: %v", err)
	}

	updated, err := service.UpdatePlanningTrack(context.Background(), created.ID, PlanningTrackInput{
		Name: "Süd", Color: "#83CD2D", SortOrder: 2,
	})
	if err != nil || updated.Name != "Süd" || updated.SortOrder != 2 {
		t.Fatalf("update planning track: track=%#v err=%v", updated, err)
	}

	repo.updateIfActiveErr = wantErr
	if _, err := service.UpdatePlanningTrack(context.Background(), created.ID, input); !errors.Is(err, wantErr) {
		t.Fatalf("update planning track: expected %v, got %v", wantErr, err)
	}
	repo.updateIfActiveErr = nil
	notUpdated := false
	repo.updateIfActiveResult = &notUpdated
	if _, err := service.UpdatePlanningTrack(context.Background(), created.ID, input); !errors.Is(err, ErrPlanningTrackArchived) {
		t.Fatalf("update planning track: expected archived, got %v", err)
	}

	archivedAt := time.Now()
	archived := planningTrackWithID(50, "Alt", "#5080D8")
	archived.ArchivedAt = &archivedAt
	archivedService := NewPlanningTrackService(newPlanningTrackRepoStub(archived), nil)
	if _, err := archivedService.UpdatePlanningTrack(context.Background(), archived.ID, input); !errors.Is(err, ErrPlanningTrackArchived) {
		t.Fatalf("update archived planning track: got %v", err)
	}
}

func TestPlanningTrackServiceArchiveRestoreEdges(t *testing.T) {
	t.Parallel()

	active := planningTrackWithID(1, "Früh", "#5080D8")
	archivedAt := time.Now()
	archived := planningTrackWithID(2, "Alt", "#83CD2D")
	archived.ArchivedAt = &archivedAt
	repo := newPlanningTrackRepoStub(active, archived)
	service := NewPlanningTrackService(repo, nil)

	unchanged, err := service.ArchivePlanningTrack(context.Background(), archived.ID)
	if err != nil || unchanged != archived {
		t.Fatalf("archive archived track: track=%#v err=%v", unchanged, err)
	}
	unchanged, err = service.RestorePlanningTrack(context.Background(), active.ID)
	if err != nil || unchanged != active {
		t.Fatalf("restore active track: track=%#v err=%v", unchanged, err)
	}

	wantErr := errors.New("database unavailable")
	repo.updateColumnsErr = wantErr
	if _, err := service.ArchivePlanningTrack(context.Background(), active.ID); !errors.Is(err, wantErr) {
		t.Fatalf("archive planning track: expected %v, got %v", wantErr, err)
	}
	repo.restoreAtEndErr = wantErr
	if _, err := service.RestorePlanningTrack(context.Background(), archived.ID); !errors.Is(err, wantErr) {
		t.Fatalf("restore planning track: expected %v, got %v", wantErr, err)
	}
}

func TestPlanningTrackServiceValidateAssignment(t *testing.T) {
	t.Parallel()

	active := planningTrackWithID(1, "Früh", "#5080D8")
	archivedAt := time.Now()
	archived := planningTrackWithID(2, "Alt", "#83CD2D")
	archived.ArchivedAt = &archivedAt
	repo := newPlanningTrackRepoStub(active, archived)
	service := NewPlanningTrackService(repo, nil)

	if err := service.ValidatePlanningTrackAssignment(context.Background(), nil); err != nil {
		t.Fatalf("validate empty assignment: %v", err)
	}
	zero := int64(0)
	if err := service.ValidatePlanningTrackAssignment(context.Background(), &zero); !errors.Is(err, ErrPlanningTrackNotFound) {
		t.Fatalf("validate zero assignment: got %v", err)
	}
	if err := service.ValidatePlanningTrackAssignment(context.Background(), &active.ID); err != nil {
		t.Fatalf("validate active assignment: %v", err)
	}
	if err := service.ValidatePlanningTrackAssignment(context.Background(), &archived.ID); !errors.Is(err, ErrPlanningTrackArchived) {
		t.Fatalf("validate archived assignment: got %v", err)
	}
	missing := int64(99)
	if err := service.ValidatePlanningTrackAssignment(context.Background(), &missing); !errors.Is(err, ErrPlanningTrackNotFound) {
		t.Fatalf("validate missing assignment: got %v", err)
	}
	wantErr := errors.New("database unavailable")
	repo.findForShareErr = wantErr
	if err := service.ValidatePlanningTrackAssignment(context.Background(), &active.ID); !errors.Is(err, wantErr) {
		t.Fatalf("validate assignment: expected %v, got %v", wantErr, err)
	}
}

func TestPlanningTrackHelpers(t *testing.T) {
	t.Parallel()

	one := time.Now().UnixNano()
	two := one + 1
	if err := validateAssignablePlanningTrack(context.Background(), nil, nil, nil); err != nil {
		t.Fatalf("validate empty assignment: %v", err)
	}
	if err := validateAssignablePlanningTrack(context.Background(), nil, &one, nil); !errors.Is(err, ErrPlanningTrackNotFound) {
		t.Fatalf("validate without repository: got %v", err)
	}
	repo := newPlanningTrackRepoStub()
	if err := validateAssignablePlanningTrack(context.Background(), repo, &one, nil); !errors.Is(err, ErrPlanningTrackNotFound) {
		t.Fatalf("validate missing assignment: got %v", err)
	}
	wantErr := errors.New("database unavailable")
	repo.findForShareErr = wantErr
	if err := validateAssignablePlanningTrack(context.Background(), repo, &one, nil); !errors.Is(err, wantErr) {
		t.Fatalf("validate assignment: expected %v, got %v", wantErr, err)
	}
	if !samePlanningTrackID(nil, nil) || !samePlanningTrackID(&one, &one) {
		t.Fatal("equal planning track ids were not recognized")
	}
	if samePlanningTrackID(nil, &one) || samePlanningTrackID(&one, nil) || samePlanningTrackID(&one, &two) {
		t.Fatal("different planning track ids were treated as equal")
	}

	service := NewPlanningTrackService(newPlanningTrackRepoStub(), nil)
	if err := service.ReorderPlanningTracks(context.Background(), []int64{0}); !errors.Is(err, ErrPlanningTrackInvalid) {
		t.Fatalf("reorder zero id: got %v", err)
	}
	if err := service.ReorderPlanningTracks(context.Background(), []int64{one, one}); !errors.Is(err, ErrPlanningTrackInvalid) {
		t.Fatalf("reorder duplicate id: got %v", err)
	}
	if err := service.ReorderPlanningTracks(context.Background(), []int64{one}); err == nil {
		t.Fatal("reorder without database must fail")
	}
}

func TestPlanningTrackAssignmentRejectsArchivedExceptExistingReference(t *testing.T) {
	t.Parallel()

	archivedAt := time.Now()
	track := &model.PlanningTrack{
		Name: "Alt", Color: "#5080D8", ArchivedAt: &archivedAt,
	}
	repo := newPlanningTrackRepoStub()
	if err := repo.Create(context.Background(), track); err != nil {
		t.Fatalf("create planning track: %v", err)
	}
	id := track.ID

	if err := validateAssignablePlanningTrack(context.Background(), repo, &id, nil); !errors.Is(err, ErrPlanningTrackArchived) {
		t.Fatalf("expected archived assignment rejection, got %v", err)
	}
	if err := validateAssignablePlanningTrack(context.Background(), repo, &id, &id); err != nil {
		t.Fatalf("existing archived assignment must remain editable: %v", err)
	}
}

func TestPlanningTrackServiceArchiveAndRestore(t *testing.T) {
	t.Parallel()

	track := &model.PlanningTrack{
		Name: "Früh", Color: "#83CD2D", SortOrder: 0,
	}
	remaining := &model.PlanningTrack{Name: "Mittag", Color: "#F78C10", SortOrder: 0}
	remaining.ID = 2
	repo := newPlanningTrackRepoStub(remaining)
	if err := repo.Create(context.Background(), track); err != nil {
		t.Fatalf("create planning track: %v", err)
	}
	service := NewPlanningTrackService(repo, nil)

	archived, err := service.ArchivePlanningTrack(context.Background(), track.ID)
	if err != nil || archived.ArchivedAt == nil {
		t.Fatalf("archive planning track: track=%#v err=%v", archived, err)
	}
	restored, err := service.RestorePlanningTrack(context.Background(), track.ID)
	if err != nil || restored.ArchivedAt != nil {
		t.Fatalf("restore planning track: track=%#v err=%v", restored, err)
	}
	if restored.SortOrder != 1 {
		t.Fatalf("restored sort order = %d, want 1", restored.SortOrder)
	}
}
