package services

import "github.com/moto-nrw/project-phoenix/modules/identityaccess"

// The retained People Directory flows report Identity & Access outcomes to
// callers that may not name the owner. The composition root re-exports the
// ones they classify on, so the refusal keeps its identity and its text
// (#3364).

// ErrParentMustUseParentPortal reports a guardian-only account at the tenant
// login, which the portals turn into a redirect message.
var ErrParentMustUseParentPortal = identityaccess.ErrParentMustUseParentPortal
