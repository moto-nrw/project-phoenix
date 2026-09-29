package application

import "github.com/moto-nrw/project-phoenix/modules/enrollment"

// RequestReads is the per-account read state of the admin queue (#3778); nil
// when the composition bound none. The owner keeps it; the decision flow only
// carries it to the adapters that serve the queue.
func (d *Decisions) RequestReads() enrollment.RequestReads {
	return d.deps.Reads
}
