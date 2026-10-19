package api

import (
	"time"
)

// Identity & Access owns the password reset flows (#2722). This file binds
// the reset link mail to the Delivery dispatcher with the portal hosts the
// root knows, and serves the retained auth service's consumer-owned port
// over the public module.

// cleanupResetExpiry configures the reset flows of the maintenance roots,
// which only clean up and never issue a link.
const cleanupResetExpiry = time.Hour
