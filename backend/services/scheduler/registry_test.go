package scheduler

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type registryJob struct {
	id      JobID
	started *int
}

func (job registryJob) ID() JobID { return job.id }

func (job registryJob) Start() {
	if job.started != nil {
		*job.started++
	}
}

func TestNewRegistryAcceptsCompleteJobs(t *testing.T) {
	t.Parallel()

	started := 0
	registry, err := NewRegistry(
		[]JobID{"alpha", "beta"},
		registryJob{id: "alpha", started: &started},
		registryJob{id: "beta", started: &started},
	)

	require.NoError(t, err)
	assert.Equal(t, []JobID{"alpha", "beta"}, registry.IDs())
	registry.Start()
	assert.Equal(t, 2, started)
}

func TestNewRegistryRejectsMissingJob(t *testing.T) {
	t.Parallel()

	_, err := NewRegistry(
		[]JobID{"alpha", "beta"},
		registryJob{id: "alpha"},
	)

	require.ErrorContains(t, err, `missing worker jobs: beta`)
}

func TestNewRegistryRejectsDuplicateJob(t *testing.T) {
	t.Parallel()

	_, err := NewRegistry(
		[]JobID{"alpha"},
		registryJob{id: "alpha"},
		registryJob{id: "alpha"},
	)

	require.ErrorContains(t, err, `duplicate worker job "alpha"`)
}

func TestNewRegistryRejectsDuplicateRequiredID(t *testing.T) {
	t.Parallel()

	_, err := NewRegistry(
		[]JobID{"alpha", "alpha"},
		registryJob{id: "alpha"},
	)

	require.ErrorContains(t, err, `duplicate required worker job "alpha"`)
}

func TestNewRegistryRejectsEmptyRequiredID(t *testing.T) {
	t.Parallel()

	_, err := NewRegistry([]JobID{""}, registryJob{id: ""})

	require.ErrorContains(t, err, "empty required worker job ID")
}

func TestNewRegistryRejectsNilJob(t *testing.T) {
	t.Parallel()

	_, err := NewRegistry(
		[]JobID{"alpha"},
		registryJob{id: "alpha"},
		nil,
	)

	require.ErrorContains(t, err, "nil worker job at index 1")
}

func TestNewRegistryRejectsTypedNilJob(t *testing.T) {
	t.Parallel()

	var job *registryJob
	_, err := NewRegistry([]JobID{"alpha"}, job)

	require.ErrorContains(t, err, "nil worker job at index 0")
}

func TestNewRegistryRejectsUnknownJob(t *testing.T) {
	t.Parallel()

	_, err := NewRegistry(
		[]JobID{"alpha"},
		registryJob{id: "alpha"},
		registryJob{id: "beta"},
	)

	require.ErrorContains(t, err, `unknown worker job "beta"`)
}
