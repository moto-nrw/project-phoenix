package application

import (
	context "context"
	io "io"

	ports "github.com/moto-nrw/project-phoenix/modules/filestorage/internal/ports"
)

// Observer reports every observation with the public error.
func Observer(observe func(ports.Observation)) func(ports.Observation) {
	return func(observation ports.Observation) {
		observation.Err = MapPublicError(observation.Err)
		observe(observation)
	}
}

type ObjectStore struct {
	Kind    string
	Backend ObjectBackend
}

func (s ObjectStore) Save(ctx context.Context, tenantID int64, storedName string, source io.Reader) (int64, error) {
	return s.Backend.SavePrivate(ctx, s.Kind, tenantID, storedName, source)
}
func (s ObjectStore) Open(ctx context.Context, tenantID int64, storedName string) (ports.Object, error) {
	return s.Backend.OpenPrivate(ctx, s.Kind, tenantID, storedName)
}
func (s ObjectStore) Remove(ctx context.Context, tenantID int64, storedName string) error {
	return s.Backend.RemovePrivate(ctx, s.Kind, tenantID, storedName)
}

type ObjectBackend interface {
	SavePrivate(ctx context.Context, kind string, tenantID int64, storedName string, source io.Reader) (int64, error)
	OpenPrivate(ctx context.Context, kind string, tenantID int64, storedName string) (ports.Object, error)
	RemovePrivate(ctx context.Context, kind string, tenantID int64, storedName string) error
}
