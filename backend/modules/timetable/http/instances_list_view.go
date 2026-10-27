package timetablehttp

import "fmt"

// Values of the `view` query parameter of GET /instances.
const (
	// instanceListViewFull is the historical projection and stays the
	// default, so every existing consumer keeps its payload.
	instanceListViewFull = "full"
	// instanceListViewSlim drops the per-child attendance rows (#4996). They
	// are about half of a week's payload, and the Betreuungsplan grid renders
	// none of them: the block detail loads its day in full when it opens.
	instanceListViewSlim = "slim"
)

// parseInstanceListView resolves the `view` query parameter. An unknown value
// is rejected instead of silently serving the full payload.
func parseInstanceListView(value string) (slim bool, err error) {
	switch value {
	case "", instanceListViewFull:
		return false, nil
	case instanceListViewSlim:
		return true, nil
	default:
		return false, fmt.Errorf("invalid view %q: must be %q or %q", value, instanceListViewFull, instanceListViewSlim)
	}
}

// slimInstances drops the attendance rows. Counts, student_ids and every
// other field stay, so the grid and its badges render as before.
func slimInstances(instances []enrichedInstance) {
	for i := range instances {
		instances[i].Students = nil
	}
}
