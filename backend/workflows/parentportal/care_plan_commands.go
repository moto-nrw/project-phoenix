package parentportal

// CarePlanCommands shares the caller's tenant unit of work across day locks,
// absence reports and pickup commands.
type CarePlanCommands interface {
	CareExceptions
	GuardianAbsenceReports
	GuardianPickupExceptions
}

func NewCarePlanCommands(exceptions CareExceptions, absences GuardianAbsenceReports, pickups GuardianPickupExceptions) CarePlanCommands {
	return carePlanCommands{exceptions, absences, pickups}
}

type carePlanCommands struct {
	CareExceptions
	GuardianAbsenceReports
	GuardianPickupExceptions
}
