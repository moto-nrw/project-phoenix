package schedule

import (
	"context"
	"errors"
	"fmt"
	"time"

	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	model "github.com/moto-nrw/project-phoenix/models/schedule"
	"github.com/moto-nrw/project-phoenix/tenant"
	"github.com/uptrace/bun"
)

var (
	ErrPlanningTrackNotFound  = errors.New("planning track not found")
	ErrPlanningTrackInvalid   = errors.New("invalid planning track")
	ErrPlanningTrackNameTaken = errors.New("a planning track with this name already exists")
	ErrPlanningTrackArchived  = errors.New("planning track is archived")
)

type PlanningTrackInput struct {
	Name      string
	Color     string
	SortOrder int
}

type PlanningTrackService interface {
	ListPlanningTracks(ctx context.Context) ([]*model.PlanningTrack, error)
	GetPlanningTrack(ctx context.Context, id int64) (*model.PlanningTrack, error)
	CreatePlanningTrack(ctx context.Context, input PlanningTrackInput) (*model.PlanningTrack, error)
	UpdatePlanningTrack(ctx context.Context, id int64, input PlanningTrackInput) (*model.PlanningTrack, error)
	ReorderPlanningTracks(ctx context.Context, ids []int64) error
	ArchivePlanningTrack(ctx context.Context, id int64) (*model.PlanningTrack, error)
	RestorePlanningTrack(ctx context.Context, id int64) (*model.PlanningTrack, error)
	ValidatePlanningTrackAssignment(ctx context.Context, id *int64) error
}

type planningTrackService struct {
	repo model.PlanningTrackRepository
	db   *bun.DB
}

func NewPlanningTrackService(repo model.PlanningTrackRepository, db *bun.DB) PlanningTrackService {
	return &planningTrackService{repo: repo, db: db}
}

func (s *planningTrackService) ListPlanningTracks(ctx context.Context) ([]*model.PlanningTrack, error) {
	return s.repo.ListAll(ctx)
}

func (s *planningTrackService) GetPlanningTrack(ctx context.Context, id int64) (*model.PlanningTrack, error) {
	if id <= 0 {
		return nil, ErrPlanningTrackNotFound
	}
	track, err := s.repo.FindByID(ctx, id)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return nil, ErrPlanningTrackNotFound
		}
		return nil, err
	}
	return track, nil
}

func (s *planningTrackService) CreatePlanningTrack(ctx context.Context, input PlanningTrackInput) (*model.PlanningTrack, error) {
	track := &model.PlanningTrack{Name: input.Name, Color: input.Color, SortOrder: input.SortOrder}
	if err := track.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrPlanningTrackInvalid, err)
	}
	track.SetTenantID(tenant.FromContext(ctx))
	if err := s.repo.Create(ctx, track); err != nil {
		if modelBase.IsUniqueViolation(err) {
			return nil, ErrPlanningTrackNameTaken
		}
		return nil, err
	}
	return track, nil
}

func (s *planningTrackService) UpdatePlanningTrack(ctx context.Context, id int64, input PlanningTrackInput) (*model.PlanningTrack, error) {
	track, err := s.GetPlanningTrack(ctx, id)
	if err != nil {
		return nil, err
	}
	if track.IsArchived() {
		return nil, ErrPlanningTrackArchived
	}
	track.Name, track.Color, track.SortOrder = input.Name, input.Color, input.SortOrder
	if err := track.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrPlanningTrackInvalid, err)
	}
	updated, err := s.repo.UpdateIfActive(ctx, track)
	if err != nil {
		if modelBase.IsUniqueViolation(err) {
			return nil, ErrPlanningTrackNameTaken
		}
		return nil, err
	}
	if !updated {
		return nil, ErrPlanningTrackArchived
	}
	return track, nil
}

func (s *planningTrackService) ReorderPlanningTracks(ctx context.Context, ids []int64) error {
	seen := make(map[int64]struct{}, len(ids))
	for _, id := range ids {
		if id <= 0 {
			return ErrPlanningTrackInvalid
		}
		if _, exists := seen[id]; exists {
			return ErrPlanningTrackInvalid
		}
		seen[id] = struct{}{}
	}
	tenantID := tenant.FromContext(ctx)
	if tenantID <= 0 || s.db == nil {
		return errors.New("planning track service is not configured")
	}
	return tenant.WithTenantTx(ctx, s.db, tenantID, func(txCtx context.Context, _ bun.Tx) error {
		if err := s.repo.UpdateSortOrders(txCtx, ids); err != nil {
			if modelBase.IsNoRows(err) {
				return ErrPlanningTrackNotFound
			}
			return err
		}
		return nil
	})
}

func (s *planningTrackService) ArchivePlanningTrack(ctx context.Context, id int64) (*model.PlanningTrack, error) {
	track, err := s.GetPlanningTrack(ctx, id)
	if err != nil || track.IsArchived() {
		return track, err
	}
	now := time.Now()
	track.ArchivedAt = &now
	if _, err := s.repo.UpdateColumns(ctx, track, "archived_at"); err != nil {
		return nil, err
	}
	return s.GetPlanningTrack(ctx, id)
}

func (s *planningTrackService) RestorePlanningTrack(ctx context.Context, id int64) (*model.PlanningTrack, error) {
	track, err := s.GetPlanningTrack(ctx, id)
	if err != nil || !track.IsArchived() {
		return track, err
	}
	_, err = s.repo.RestoreAtEnd(ctx, track)
	if err != nil {
		if modelBase.IsUniqueViolation(err) {
			return nil, ErrPlanningTrackNameTaken
		}
		return nil, err
	}
	return s.GetPlanningTrack(ctx, id)
}

func (s *planningTrackService) ValidatePlanningTrackAssignment(ctx context.Context, id *int64) error {
	if id == nil {
		return nil
	}
	if *id <= 0 {
		return ErrPlanningTrackNotFound
	}
	track, err := s.repo.FindByIDForShare(ctx, *id)
	if err != nil {
		if modelBase.IsNoRows(err) {
			return ErrPlanningTrackNotFound
		}
		return err
	}
	if track.IsArchived() {
		return ErrPlanningTrackArchived
	}
	return nil
}
