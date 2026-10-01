package architecture

import (
	"runtime/debug"
	"sync"
	"testing"
)

// repositoryGraphLoad admits one full load of the real backend at a time. Each
// load type-checks the whole module from source and holds about 2 GB, whether
// it runs in the test binary or in a spawned architecture CLI. Parallel tests
// would otherwise stack these peaks on top of each other.
var repositoryGraphLoad sync.Mutex

// holdRepositoryGraphLoad serializes the calling test with every other test
// that loads the real backend graph, until the test and its subtests finish.
// Releasing returns the heap of an in-process load to the OS, so the next
// holder does not stack on memory this process still keeps mapped.
func holdRepositoryGraphLoad(t *testing.T) {
	t.Helper()
	repositoryGraphLoad.Lock()
	t.Cleanup(func() {
		debug.FreeOSMemory()
		repositoryGraphLoad.Unlock()
	})
}
