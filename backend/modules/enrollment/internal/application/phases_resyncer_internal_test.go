package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
)

type phaseRecordsStub struct{ PhaseRecords }
type phaseOfferingsStub struct{ PhaseOfferings }
type sourcedTemplateResyncerStub struct {
	enrollment.SourcedTemplateResyncer
}

func resyncingPhaseDependencies() PhaseDependencies {
	return PhaseDependencies{
		Records: phaseRecordsStub{}, Offerings: phaseOfferingsStub{},
		LockTemplateRecurrence: func(context.Context) error { return nil },
		Resyncer:               sourcedTemplateResyncerStub{},
	}
}

// The phase administration built over the resyncer value resyncs through
// it; a missing collaborator of the resync and detach
// paths is a construction error instead of a logged, saved edit.
func TestNewResyncingPhases_RequiresTheResyncer(t *testing.T) {
	t.Parallel()

	deps := resyncingPhaseDependencies()
	phases, err := NewResyncingPhases(deps)
	require.NoError(t, err)
	require.Equal(t, deps.Resyncer, phases.deps.Resyncer)

	for name, change := range map[string]func(*PhaseDependencies){
		"resyncer":  func(d *PhaseDependencies) { d.Resyncer = nil },
		"records":   func(d *PhaseDependencies) { d.Records = nil },
		"offerings": func(d *PhaseDependencies) { d.Offerings = nil },
		"lock":      func(d *PhaseDependencies) { d.LockTemplateRecurrence = nil },
	} {
		deps := resyncingPhaseDependencies()
		change(&deps)
		_, err := NewResyncingPhases(deps)
		require.ErrorContains(t, err, "are required", name)
	}
}
