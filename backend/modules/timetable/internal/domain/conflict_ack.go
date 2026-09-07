package domain

// MaxConflictAcksPerAccount caps how many conflict acknowledgements one
// account can hold per tenant. Fingerprints are opaque to the server, so
// without a cap any plan reader could accumulate unbounded rows (#2151
// review). Acknowledging beyond the cap prunes the oldest rows first; stale
// acknowledgements stop matching once the underlying conflict changes, so
// dropping them is lossless in practice.
const MaxConflictAcksPerAccount = 500
