package enrollment

import (
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

// WriteSelectionDateForTest exposes the clamp shared by the read and write
// paths. The parity test supplies their common clock snapshot (#2185).
func WriteSelectionDateForTest(phase *enrollmentModels.Phase, today timezone.Date) timezone.Date {
	return offeringSelectionDateOn(phase, today)
}
