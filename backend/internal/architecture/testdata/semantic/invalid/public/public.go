package public

import (
	"example.test/architecture-semantic/capability"
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

type factoryResult struct{}

func New() factoryResult { return factoryResult{} }

func (factoryResult) List(map[string]any) error { return nil }

type recursiveFactoryResult struct{}

func NewRecursive() recursiveFactoryResult { return recursiveFactoryResult{} }

func (recursiveFactoryResult) List() recursiveFactoryResult { return recursiveFactoryResult{} }

func NewAnonymous() interface{ List() error } { return nil }

func NewImported() capability.Service { return nil }

type sharedFactoryResult struct{}

type sharedNestedResult struct{}

func NewShared() sharedFactoryResult { return sharedFactoryResult{} }

func (sharedFactoryResult) First() sharedNestedResult { return sharedNestedResult{} }

func (sharedFactoryResult) Second() sharedNestedResult { return sharedNestedResult{} }

func (sharedNestedResult) List() error { return nil }
