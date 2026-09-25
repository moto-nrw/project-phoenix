package students

import "github.com/moto-nrw/project-phoenix/modules/requestreview"

// DirectCorrectionResponse is one admin correction to a child's bookings as
// the central history renders it (#2436); the shared request-review
// projection (#2705) owns the shape.
type DirectCorrectionResponse = requestreview.DirectCorrectionResponse
