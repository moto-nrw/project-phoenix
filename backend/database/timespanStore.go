package database

import (
	"context"
	"github.com/moto-nrw/project-phoenix/models"
	"time"

	"github.com/uptrace/bun"
)

// TimespanStore defines database operations for timespans
type TimespanStore interface {
	CreateTimespan(ctx context.Context, startTime time.Time, endTime *time.Time) (*models.Timespan, error)
	GetTimespan(ctx context.Context, id int64) (*models.Timespan, error)
	UpdateTimespanEndTime(ctx context.Context, id int64, endTime time.Time) error
	DeleteTimespan(ctx context.Context, id int64) error
}

type timespanStore struct {
	db *bun.DB
}

// NewTimespanStore returns a TimespanStore implementation
func NewTimespanStore(db *bun.DB) TimespanStore {
	return &timespanStore{db: db}
}

// CreateTimespan creates a new timespan
func (s *timespanStore) CreateTimespan(ctx context.Context, startTime time.Time, endTime *time.Time) (*models.Timespan, error) {
	timespan := &models.Timespan{
		StartTime: startTime,
		EndTime:   endTime,
		CreatedAt: time.Now(),
	}

	_, err := s.db.NewInsert().
		Model(timespan).
		Exec(ctx)

	if err != nil {
		return nil, err
	}

	return timespan, nil
}

// GetTimespan returns a timespan by ID
func (s *timespanStore) GetTimespan(ctx context.Context, id int64) (*models.Timespan, error) {
	timespan := new(models.Timespan)
	err := s.db.NewSelect().
		Model(timespan).
		Where("id = ?", id).
		Scan(ctx)

	if err != nil {
		return nil, err
	}

	return timespan, nil
}

// UpdateTimespanEndTime updates the end time of a timespan
func (s *timespanStore) UpdateTimespanEndTime(ctx context.Context, id int64, endTime time.Time) error {
	_, err := s.db.NewUpdate().
		Model((*models.Timespan)(nil)).
		Set("endtime = ?", endTime).
		Where("id = ?", id).
		Exec(ctx)

	return err
}

// DeleteTimespan deletes a timespan
func (s *timespanStore) DeleteTimespan(ctx context.Context, id int64) error {
	_, err := s.db.NewDelete().
		Model((*models.Timespan)(nil)).
		Where("id = ?", id).
		Exec(ctx)

	return err
}
