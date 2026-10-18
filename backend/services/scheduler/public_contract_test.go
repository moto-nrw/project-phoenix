package scheduler

import (
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/schedulerruntime/ports"
	"github.com/stretchr/testify/require"
)

type contractJob struct {
	id      ports.JobID
	started *[]ports.JobID
}

func (j contractJob) ID() ports.JobID { return j.id }
func (j contractJob) Start()          { *j.started = append(*j.started, j.id) }
func TestPublicSchedulerContractStartsRequiredJobsInOrderAndOwnsInventory(t *testing.T) {
	t.Parallel()
	required := []ports.JobID{"first", "second"}
	var started []ports.JobID
	registry, err := NewRegistry(required, contractJob{"second", &started}, contractJob{"first", &started})
	require.NoError(t, err)
	require.Equal(t, required, registry.IDs())
	required[0] = "changed"
	exported := registry.IDs()
	exported[0] = "also changed"
	require.Equal(t, []ports.JobID{"first", "second"}, registry.IDs())
	registry.Start()
	require.Equal(t, []ports.JobID{"first", "second"}, started)
	missing, err := NewRegistry([]ports.JobID{"missing"})
	require.EqualError(t, err, "missing worker jobs: missing")
	require.Nil(t, missing)
}
