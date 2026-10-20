package application

import (
	"context"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/facilities"
	"github.com/moto-nrw/project-phoenix/modules/facilities/internal/domain"
	"github.com/moto-nrw/project-phoenix/modules/facilities/internal/ports"
)

type seamRecorder interface {
	FindRoom(context.Context, int64) (facilities.Room, error)
	FindRoomByName(context.Context, string) (facilities.Room, error)
	FindToiletRoom(context.Context, int64) (facilities.Room, error)
	ListRooms(context.Context, facilities.RoomFilter) ([]facilities.Room, error)
	ListRoomsPage(context.Context, facilities.RoomFilter, int, int) (facilities.RoomPage, error)
	ListRoomsByID(context.Context, []int64) ([]facilities.Room, error)
	LockRoomsByID(context.Context, []int64) ([]facilities.Room, error)
	CreateRoom(context.Context, facilities.CreateRoom) (facilities.Room, error)
	UpdateRoom(context.Context, facilities.UpdateRoom) (facilities.Room, error)
	DeleteRoom(context.Context, int64) error
	ObserveRejection(string, time.Duration, error)
}
type seamStore struct{ e seamRecorder }

func (s seamStore) Create(ctx context.Context, v facilities.CreateRoom) (facilities.Room, domain.OperationStats, error) {
	r, err := s.e.CreateRoom(ctx, v)
	return r, domain.OperationStats{}, err
}
func (s seamStore) Update(ctx context.Context, v facilities.UpdateRoom) (facilities.Room, domain.OperationStats, error) {
	r, err := s.e.UpdateRoom(ctx, v)
	return r, domain.OperationStats{}, err
}
func (s seamStore) Delete(ctx context.Context, id int64) (domain.OperationStats, error) {
	return domain.OperationStats{}, s.e.DeleteRoom(ctx, id)
}
func (s seamStore) FindByID(ctx context.Context, id int64, lock string) (facilities.Room, bool, domain.OperationStats, error) {
	if lock != "" {
		return facilities.Room{ID: id, Name: "Igelraum"}, true, domain.OperationStats{}, nil
	}
	r, err := s.e.FindRoom(ctx, id)
	return r, err == nil, domain.OperationStats{}, err
}
func (s seamStore) FindByName(context.Context, string) (facilities.Room, bool, domain.OperationStats, error) {
	return facilities.Room{}, false, domain.OperationStats{}, nil
}
func (s seamStore) FindToilet(ctx context.Context, id int64) (facilities.Room, bool, domain.OperationStats, error) {
	r, err := s.e.FindToiletRoom(ctx, id)
	return r, err == nil, domain.OperationStats{}, err
}
func (s seamStore) List(ctx context.Context, f facilities.RoomFilter) ([]facilities.Room, domain.OperationStats, error) {
	r, err := s.e.ListRooms(ctx, f)
	return r, domain.OperationStats{}, err
}
func (s seamStore) ListPage(ctx context.Context, f facilities.RoomFilter, o, l int) ([]facilities.Room, int, domain.OperationStats, error) {
	r, err := s.e.ListRoomsPage(ctx, f, o, l)
	return r.Rooms, r.Total, domain.OperationStats{}, err
}
func (s seamStore) ListByIDs(ctx context.Context, ids []int64) ([]facilities.Room, domain.OperationStats, error) {
	r, err := s.e.ListRoomsByID(ctx, ids)
	return r, domain.OperationStats{}, err
}
func (s seamStore) LockByIDs(ctx context.Context, ids []int64) ([]facilities.Room, domain.OperationStats, error) {
	r, err := s.e.LockRoomsByID(ctx, ids)
	return r, domain.OperationStats{}, err
}

type seamTransaction struct{}

func (seamTransaction) RunWrite(ctx context.Context, fn func(context.Context) error) error {
	return fn(ctx)
}
func (seamTransaction) AcquireLock(context.Context, string) error { return nil }
func newTestModule(e seamRecorder) *Service {
	if e == nil {
		return NewRooms(nil, seamTransaction{}, func(ports.Observation) {})
	}
	return New(seamStore{e}, seamTransaction{}, func(context.Context) error { return nil }, UnusedRooms{}, func(o ports.Observation) {
		if o.Err != nil {
			e.ObserveRejection(o.Operation, o.Duration, o.Err)
		}
	})
}
func newTestRooms(e seamRecorder) *Service {
	if e == nil {
		return NewRooms(nil, seamTransaction{}, func(ports.Observation) {})
	}
	return NewRooms(seamStore{e}, seamTransaction{}, func(ports.Observation) {})
}

var _ facilities.Catalog = (*Service)(nil)
var _ facilities.Capability = (*Service)(nil)
