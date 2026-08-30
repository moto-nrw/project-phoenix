package scheduler

import (
	"fmt"
	"reflect"
	"strings"
)

// JobID is the stable identity used for worker registration and runtime
// evidence.
type JobID string

// Job is one embedded worker job. Start registers its polling loop with the
// worker lifecycle.
type Job interface {
	ID() JobID
	Start()
}

// Registry owns the complete ordered set of embedded worker jobs.
type Registry struct {
	jobs []Job
	ids  []JobID
}

// NewRegistry validates the worker graph before any job can start.
func NewRegistry(required []JobID, jobs ...Job) (*Registry, error) {
	requiredIDs, err := requiredJobIDSet(required)
	if err != nil {
		return nil, err
	}
	byID := make(map[JobID]Job, len(jobs))
	for index, job := range jobs {
		if isNilJob(job) {
			return nil, fmt.Errorf("nil worker job at index %d", index)
		}
		id := job.ID()
		if _, expected := requiredIDs[id]; !expected {
			return nil, fmt.Errorf("unknown worker job %q", id)
		}
		if _, exists := byID[id]; exists {
			return nil, fmt.Errorf("duplicate worker job %q", id)
		}
		byID[id] = job
	}
	ordered, err := orderRequiredJobs(required, byID)
	if err != nil {
		return nil, err
	}
	return &Registry{jobs: ordered, ids: append([]JobID(nil), required...)}, nil
}

func requiredJobIDSet(required []JobID) (map[JobID]struct{}, error) {
	ids := make(map[JobID]struct{}, len(required))
	for _, id := range required {
		if id == "" {
			return nil, fmt.Errorf("empty required worker job ID")
		}
		if _, exists := ids[id]; exists {
			return nil, fmt.Errorf("duplicate required worker job %q", id)
		}
		ids[id] = struct{}{}
	}
	return ids, nil
}

func orderRequiredJobs(required []JobID, byID map[JobID]Job) ([]Job, error) {
	ordered := make([]Job, 0, len(required))
	missing := make([]string, 0)
	for _, id := range required {
		job, exists := byID[id]
		if !exists {
			missing = append(missing, string(id))
			continue
		}
		ordered = append(ordered, job)
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("missing worker jobs: %s", strings.Join(missing, ", "))
	}
	return ordered, nil
}

func isNilJob(job Job) bool {
	return isNilDependency(job)
}

func isNilDependency(dependency any) bool {
	if dependency == nil {
		return true
	}
	value := reflect.ValueOf(dependency)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

// IDs returns the stable job order used by logs and inventory checks.
func (registry *Registry) IDs() []JobID {
	return append([]JobID(nil), registry.ids...)
}

// Start registers every validated job with the worker lifecycle.
func (registry *Registry) Start() {
	for _, job := range registry.jobs {
		job.Start()
	}
}
