// Package communicationtest names the Communication stores the behavior tests
// of the request domains drive over the test database. It is test support:
// production code never imports it.
package communicationtest

import parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"

// NewParentMessageRepository is the parent message store Communication's
// composition binds, for suites that drive parent requests over the test
// database without importing the store itself.
var NewParentMessageRepository = parentStore.NewParentMessageRepository
