package services

import (
	"log/slog"

	"github.com/moto-nrw/project-phoenix/modules/studentpresence"
	"github.com/moto-nrw/project-phoenix/modules/studentpresence/compose/presenceservice"
)

// AtSchoolCounter counts the students who read "Schule" right now (#3260).
// The day plan it needs lives outside the presence owner.
type AtSchoolCounter = presenceservice.AtSchoolCounter

// NewPresenceOperations binds the composed presence capability to the
// presence operations port the active routes consume. A nil atSchool leaves
// the dashboard's "Zuhause" figure unsplit.
func NewPresenceOperations(service studentpresence.Presence, atSchool AtSchoolCounter, logger *slog.Logger) presenceservice.PresenceOperations {
	operations, err := presenceservice.NewPresenceOperations(service, atSchool, logger)
	if err != nil {
		panic(err)
	}
	return operations
}
