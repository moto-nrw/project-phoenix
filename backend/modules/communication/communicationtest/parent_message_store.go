package communicationtest

import parentStore "github.com/moto-nrw/project-phoenix/modules/communication/parentstore"

// NewParentMessageRepository is the parent message store Communication's
// composition binds, for suites that drive parent requests over the test
// database without importing the store itself.
var NewParentMessageRepository = parentStore.NewParentMessageRepository
