package ports

import "context"

// CareContractConclusion concludes the care contracts of the schools that
// conclude at the acceptance (Weg b, #4680, ADR 0088 section 6) over every
// Träger and returns how many it concluded. It opens its own units: the
// tenant transaction of each school it reads and a Träger transaction per
// contract. The root binds the Betreuungsvertrag's conclusion.
type CareContractConclusion func(ctx context.Context) (concluded int, err error)
