package public

import (
	"example.test/architecture-semantic/database/repositories"
	"example.test/architecture-semantic/models/internalmodel"
	"github.com/uptrace/bun"
)

type Leaky struct {
	bun.BaseModel `bun:"table:alpha.records"`
	Repository    repositories.Repository
	Record        internalmodel.Record
	Filters       map[string]any
}

type Service interface {
	List(map[string]any) ([]Leaky, error)
	Get(int64) (Leaky, error)
	Upsert(Leaky) error
}

type service struct{}

func New() service { return service{} }

func (service) List(map[string]any) error { return nil }

type recursiveService struct{}

func NewRecursive() recursiveService { return recursiveService{} }

func (recursiveService) List() recursiveService { return recursiveService{} }
