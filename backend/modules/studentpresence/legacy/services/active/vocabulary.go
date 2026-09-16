package active

import "github.com/moto-nrw/project-phoenix/models/base"

// The shared persistence vocabulary the retained service still speaks. The
// package names the embedded model row, the list options and the repository
// error contract it depends on so its own tests build them without reaching
// past the package (#3214); every entry goes with the retained code that uses it.
type (
	Model         = base.Model
	QueryOptions  = base.QueryOptions
	Pagination    = base.Pagination
	DatabaseError = base.DatabaseError
)

var (
	ErrNotFound = base.ErrNotFound
	IsNoRows    = base.IsNoRows
)
