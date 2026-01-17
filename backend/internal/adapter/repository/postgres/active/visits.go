package active

import (
	"github.com/moto-nrw/project-phoenix/internal/adapter/repository/postgres/base"
	activeDomain "github.com/moto-nrw/project-phoenix/internal/core/domain/active"
	activePort "github.com/moto-nrw/project-phoenix/internal/core/port/active"
	"github.com/uptrace/bun"
)

// Table name constants (S1192 - avoid duplicate string literals)
const (
	tableActiveVisits            = "active.visits"
	tableExprActiveVisitsAsVisit = `active.visits AS "visit"`
)

// VisitRepository implements activePort.VisitRepository interface
// It's split across multiple files for readability:
// - visits.go: Constructor and struct definition
// - visits_lifecycle.go: Create, EndVisit, TransferVisitsFromRecentSessions
// - visits_query.go: All Find/Get query methods
// - visits_retention.go: Cleanup and retention-related methods
type VisitRepository struct {
	*base.Repository[*activeDomain.Visit]
	db *bun.DB
}

// NewVisitRepository creates a new VisitRepository
func NewVisitRepository(db *bun.DB) activePort.VisitRepository {
	return &VisitRepository{
		Repository: base.NewRepository[*activeDomain.Visit](db, tableActiveVisits, "Visit"),
		db:         db,
	}
}
