---
status: accepted
---

# Export Transfer is the tenth platform module

Export Transfer is a separate platform module because it applies the same
tenant-configured, audited and transport-secured delivery policy to files whose
business meaning remains with their producing domain. It owns transfer
configuration resolution, destination validation, SFTP transport and the
durable transfer journal; it does not own or regenerate the exported content.

Keeping it separate from Delivery avoids granting the existing notification,
mail and realtime packages the SFTP module's persistence and cryptographic
dependencies. Treating it as a workflow was rejected because workflows cannot
own runtime data, while the audit journal must be committed in the caller's
tenant transaction for both successful and failed attempts.
