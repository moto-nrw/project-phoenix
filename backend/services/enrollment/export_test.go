package enrollment

import (
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
)

// WriteSelectionDateForTest exposes the date UpdateChildOfferings treats as
// "now" when it reads the selection it is about to replace. Only the parity
// test uses it: it needs the write path's own date to prove the read path
// hands the editor exactly that selection (#2185).
func WriteSelectionDateForTest(phase *enrollmentModels.Phase) timezone.Date {
	return currentOfferingSelectionDate(phase)
}
