package compose

import (
	"time"

	"github.com/moto-nrw/project-phoenix/modules/carecontract"
	"github.com/moto-nrw/project-phoenix/modules/carecontract/internal/adapters/postgres"
	"github.com/moto-nrw/project-phoenix/modules/carecontract/internal/adapters/rendering"
	"github.com/moto-nrw/project-phoenix/modules/carecontract/internal/application"
)

// NoticePorts are the ports the root binds for withdrawal and termination
// (#4682): the Träger's confirmation templates and Delivery's outbox.
type NoticePorts = application.NoticePorts

// NewNotices builds withdrawal and termination (#4682): the office operations open their own Träger unit, the parents' run in the parents portal's.
func NewNotices(contracts ContractPorts, notices NoticePorts) carecontract.Notices {
	store, clock := postgres.NewStore(), application.Clock{Now: time.Now}
	return application.NewNotices(store, store, notices, clock,
		application.NewContracts(store, rendering.NewRenderer(), contracts, clock, NewContractVersions()))
}

// NewParentNotices builds the declarations of the parents portal alone,
// in its Träger unit; it needs no letterhead or access log.
func NewParentNotices(notices NoticePorts) carecontract.Notices {
	store := postgres.NewStore()
	return application.NewNotices(store, store, notices, application.Clock{Now: time.Now}, nil)
}

// NewNoticeMailRenderer renders a confirmation mail for Delivery's worker.
func NewNoticeMailRenderer() carecontract.NoticeMailRenderer {
	return application.RenderNoticeMail
}
