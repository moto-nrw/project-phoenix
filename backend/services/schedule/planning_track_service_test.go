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
	tracks map[int64]*model.PlanningTrack
	nextID int64
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

func (r *planningTrackRepoStub) Create(_ context.Context, track *model.PlanningTrack) error {
	track.ID = r.nextID
	r.nextID++
	r.tracks[track.ID] = track
	return nil
}

func (r *planningTrackRepoStub) FindByID(_ context.Context, id any) (*model.PlanningTrack, error) {
	track, ok := r.tracks[id.(int64)]
	if !ok {
		return nil, &modelBase.DatabaseError{Op: "find planning track", Err: errors.New("sql: no rows in result set")}
	}
	return track, nil
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
	result := make([]*model.PlanningTrack, 0, len(r.tracks))
	for _, track := range r.tracks {
		result = append(result, track)
	}
	return result, nil
}

func (r *planningTrackRepoStub) FindByIDForShare(ctx context.Context, id int64) (*model.PlanningTrack, error) {
	return r.FindByID(ctx, id)
}

func (r *planningTrackRepoStub) UpdateIfActive(_ context.Context, track *model.PlanningTrack) (bool, error) {
	stored, ok := r.tracks[track.ID]
	if !ok || stored.ArchivedAt != nil {
		return false, nil
	}
	r.tracks[track.ID] = track
	return true, nil
}

func (r *planningTrackRepoStub) UpdateSortOrders(context.Context, []int64) error {
	return nil
}

func (r *planningTrackRepoStub) UpdateColumns(_ context.Context, track *model.PlanningTrack, _ ...string) (int64, error) {
	r.tracks[track.ID] = track
	return 1, nil
}

func TestPlanningTrackServiceCreateAndUpdateValidation(t *testing.T) {
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

func TestPlanningTrackAssignmentRejectsArchivedExceptExistingReference(t *testing.T) {
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
	track := &model.PlanningTrack{
		Name: "Früh", Color: "#83CD2D",
	}
	repo := newPlanningTrackRepoStub()
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
}
