package application

import (
	"context"
	"errors"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
)

type PlanningTrackViews struct {
	tracks timetable.PlanningTrackCapability
}

func NewPlanningTrackViews(tracks timetable.PlanningTrackCapability) *PlanningTrackViews {
	return &PlanningTrackViews{tracks: tracks}
}
func (a *PlanningTrackViews) ListAllPlanningTracks(ctx context.Context) ([]timetable.PlanningTrack, error) {
	return a.tracks.ListPlanningTracks(ctx, timetable.PlanningTrackFilter{Ordered: true})
}

func (a *PlanningTrackViews) GetPlanningTrack(ctx context.Context, id int64) (timetable.PlanningTrack, error) {
	if id <= 0 {
		return timetable.PlanningTrack{}, timetable.ErrPlanningTrackNotFound
	}
	track, err := a.tracks.FindPlanningTrack(ctx, id)
	if errors.Is(err, timetable.ErrPlanningTrackNotFound) {
		return timetable.PlanningTrack{}, timetable.ErrPlanningTrackNotFound
	}
	return track, err
}

func (a *PlanningTrackViews) AddPlanningTrack(ctx context.Context, draft timetable.PlanningTrackDraft) (timetable.PlanningTrack, error) {
	draft, err := draft.Validate()
	if err != nil {
		return timetable.PlanningTrack{}, err
	}
	track, err := a.tracks.CreatePlanningTrack(ctx, planningTrackInputOf(draft))
	return track, planningTrackNameTaken(err)
}

func (a *PlanningTrackViews) EditPlanningTrack(ctx context.Context, id int64, draft timetable.PlanningTrackDraft) (timetable.PlanningTrack, error) {
	current, err := a.GetPlanningTrack(ctx, id)
	if err != nil {
		return timetable.PlanningTrack{}, err
	}
	if current.IsArchived() {
		return timetable.PlanningTrack{}, timetable.ErrPlanningTrackArchived
	}
	draft, err = draft.Validate()
	if err != nil {
		return timetable.PlanningTrack{}, err
	}
	updated, active, err := a.tracks.UpdateActivePlanningTrack(ctx, id, planningTrackInputOf(draft))
	if err != nil {
		return timetable.PlanningTrack{}, planningTrackNameTaken(err)
	}
	if !active {
		return timetable.PlanningTrack{}, timetable.ErrPlanningTrackArchived
	}
	return updated, nil
}

func (a *PlanningTrackViews) ArchivePlanningTrack(ctx context.Context, id int64) (timetable.PlanningTrack, error) {
	track, err := a.GetPlanningTrack(ctx, id)
	if err != nil || track.IsArchived() {
		return track, err
	}
	now := time.Now()
	if _, _, err := a.tracks.SetPlanningTrackArchivedAt(ctx, id, &now); err != nil {
		return timetable.PlanningTrack{}, err
	}
	return a.GetPlanningTrack(ctx, id)
}

func (a *PlanningTrackViews) RestorePlanningTrack(ctx context.Context, id int64) (timetable.PlanningTrack, error) {
	track, err := a.GetPlanningTrack(ctx, id)
	if err != nil || !track.IsArchived() {
		return track, err
	}
	if _, _, err := a.tracks.RestorePlanningTrackAtEnd(ctx, id); err != nil {
		return timetable.PlanningTrack{}, planningTrackNameTaken(err)
	}
	return a.GetPlanningTrack(ctx, id)
}

func (a *PlanningTrackViews) ValidatePlanningTrackAssignment(ctx context.Context, id, allowedArchivedID *int64) error {
	if id == nil {
		return nil
	}
	if *id <= 0 {
		return timetable.ErrPlanningTrackNotFound
	}
	track, err := a.tracks.FindPlanningTrackForShare(ctx, *id)
	if errors.Is(err, timetable.ErrPlanningTrackNotFound) {
		return timetable.ErrPlanningTrackNotFound
	}
	if err != nil {
		return err
	}
	if track.IsArchived() && (allowedArchivedID == nil || *allowedArchivedID != track.ID) {
		return timetable.ErrPlanningTrackArchived
	}
	return nil
}

func planningTrackInputOf(draft timetable.PlanningTrackDraft) timetable.PlanningTrackInput {
	return timetable.PlanningTrackInput{Name: draft.Name, Color: draft.Color, SortOrder: draft.SortOrder}
}

// planningTrackNameTaken turns the owner's active-name conflict into the
// editor's 409 answer.
func planningTrackNameTaken(err error) error {
	if errors.Is(err, timetable.ErrPlanningTrackNameExists) {
		return timetable.ErrPlanningTrackNameTaken
	}
	return err
}
