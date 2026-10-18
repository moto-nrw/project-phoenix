// ShiftPlanSyncer is the #1843 bridge between a sick report
// (active.staff_absences) and the planning layer (schedule.staff_shifts +
// schedule.instance_staff). The port is declared on the owner's public
// contract (modules/workforce) and bound by the composition root through
// WithAbsenceShiftPlanSyncer, mirroring the AttendanceSyncer shape. Unlike
// that syncer the contract is FAIL-CLOSED: the linkage is the feature, so an
// error must abort the surrounding absence write (a sick report whose plan
// effects half-applied is exactly the silent side effect #1843 forbids).
package timetracking

import "github.com/moto-nrw/project-phoenix/modules/workforce"

// SickCascadeInput identifies one sick report and the range it covers. Both
// cascade directions take the same shape; the dates are calendar days in
// workforce.DateLayout.
type SickCascadeInput = workforce.SickCascadeInput

// ShiftPlanSyncer cascades a sick report into the plans and back out again.
// All methods run inside the caller's tenant transaction. The shift-plan-sync
// application workflow implements it, because the two writes cross an
// ownership line.
type ShiftPlanSyncer = workforce.ShiftPlanSync
