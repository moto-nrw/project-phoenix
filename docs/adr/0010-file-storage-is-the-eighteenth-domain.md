---
status: accepted
---

# File Storage is the eighteenth domain

For [#3034](https://github.com/moto-nrw/project-phoenix/issues/3034), we agreed
that File Storage is a separate eighteenth domain in the canonical backend
module map. It represents a managed school file collection, not just byte
storage. This preserves the existing `file-storage` domain classification in
`backend/architecture/policy.json` and resolves its omission from
[#2580](https://github.com/moto-nrw/project-phoenix/issues/2580).

The classification and boundaries below were confirmed in the design interview.

## Ownership boundaries

File Storage owns file metadata, folders, folder-role and folder-account grants,
quota, and cleanup intents. Cleanup intents survive metadata deletion until
the stored bytes have been removed. This keeps the managed file lifecycle and
its rules within one domain.

Identity & Access supplies account and role facts; File Storage interprets
folder grants within the school boundary. Other domains retain access rules
for their content: Communication decides announcement readership and
editability, and File Storage respects those decisions for attachments.
Storing a file does not grant access to its source content.

Document Rendering remains a platform module that produces output. It does
not own the output's business meaning, audience, or stored lifecycle.
Generating a PDF neither saves it automatically nor grants access. The
requesting domain controls its meaning and audience; File Storage handles
the stored file when explicitly saved. Combining rendering and storage would
mix output generation with school sharing, quota, and cleanup rules.

## Decision-only alignment

#3034 records the decision and aligns the canonical map. It does not change
runtime write ownership, HTTP contracts, storage layout, or ratchet entries.
The File Storage migration remains in
[#2707](https://github.com/moto-nrw/project-phoenix/issues/2707).

The canonical map has eighteen domains and the existing nine platform modules.
Policy validation must pin both owner lists; architecture documentation and
the generated `target.svg` must agree with the executable policy. Generated
diagrams remain temporary artifacts, not committed files. #2580 must record
the same decision before #2707 closes.

The target responsibility above includes `documents.files`,
`documents.folders`, `documents.folder_roles`, `documents.folder_accounts`, and
`documents.file_cleanup`. Their currently missing policy mappings are migration
work, not permission to add mappings or change ratchet entries in #3034.
Existing File Storage ownership of announcement attachments and their cleanup
intents stays unchanged.

## Follow-up

Clarify reusable criteria for choosing a new domain, an existing domain, or a
platform module in a separate follow-up. Those general criteria are not part
of #3034 and have not been agreed in this decision.
