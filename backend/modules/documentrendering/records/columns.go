package records

import "time"

func DefaultColumnsForPreset(preset Preset) []ColumnID {
	switch preset {
	case PresetOGSCompact:
		return []ColumnID{ColumnName, ColumnSchoolClass, ColumnGroup, ColumnCareDays, ColumnDeparture, ColumnPlannedPickup}
	case PresetClassRoster:
		// Shared columns first; retain the live source's existing extra
		// columns rather than removing content while aligning the lists.
		return append(ClassRosterColumnIDs(), ColumnGroup, ColumnEnrollmentSummary, ColumnCareDays, ColumnDeparture)
	case PresetDailyPlanning:
		return []ColumnID{ColumnName, ColumnSchoolClass, ColumnGroup, ColumnDailyStatus, ColumnPlannedArrival, ColumnPlannedPickup, ColumnDailyNotes}
	case PresetAttendanceSnapshot:
		return []ColumnID{ColumnName, ColumnSchoolClass, ColumnGroup, ColumnCurrentLocation, ColumnPlannedPickup}
	case PresetPickupList:
		return []ColumnID{ColumnName, ColumnSchoolClass, ColumnGroup, ColumnDeparture, ColumnPlannedPickup, ColumnDailyNotes}
	case PresetBlankChecklist:
		return []ColumnID{ColumnName, ColumnSchoolClass, ColumnGroup}
	case PresetBirthdayList:
		return []ColumnID{ColumnName, ColumnSchoolClass, ColumnGroup, ColumnBirthday, ColumnAge}
	case PresetStaffBirthdayList:
		return []ColumnID{ColumnName, ColumnBirthday}
	default:
		return []ColumnID{ColumnName, ColumnSchoolClass, ColumnGroup, ColumnWeeklyMonday, ColumnWeeklyTuesday, ColumnWeeklyWednesday, ColumnWeeklyThursday, ColumnWeeklyFriday}
	}
}

// ColumnCatalog is the set of columns a caller-chosen column list may resolve
// to. ColumnHealthInfo is deliberately NOT in it: the child's health note is
// printed only by the two lists built for it, the Notfallliste (#2609, always
// with the health column since #3732) and the Gesundheitsliste
// (HealthListColumns, #3323). Listing it here would let any other export
// request print Art. 9 data.
func ColumnCatalog() map[ColumnID]Column {
	return map[ColumnID]Column{
		ColumnName:              {ID: ColumnName, Label: "Name"},
		ColumnSchoolClass:       {ID: ColumnSchoolClass, Label: "Klasse"},
		ColumnGroup:             {ID: ColumnGroup, Label: "Gruppe"},
		ColumnEnrollmentSummary: {ID: ColumnEnrollmentSummary, Label: "Betreuungs-/Anmeldestatus"},
		ColumnCareDays:          {ID: ColumnCareDays, Label: "Betreuungstage"},
		ColumnWeeklyMonday:      {ID: ColumnWeeklyMonday, Label: "Montag"},
		ColumnWeeklyTuesday:     {ID: ColumnWeeklyTuesday, Label: "Dienstag"},
		ColumnWeeklyWednesday:   {ID: ColumnWeeklyWednesday, Label: "Mittwoch"},
		ColumnWeeklyThursday:    {ID: ColumnWeeklyThursday, Label: "Donnerstag"},
		ColumnWeeklyFriday:      {ID: ColumnWeeklyFriday, Label: "Freitag"},
		ColumnPlannedArrival:    {ID: ColumnPlannedArrival, Label: "Geplante Ankunft"},
		ColumnPlannedPickup:     {ID: ColumnPlannedPickup, Label: "Geplante Abholung"},
		ColumnDailyStatus:       {ID: ColumnDailyStatus, Label: "Tagesstatus"},
		ColumnDeparture:         {ID: ColumnDeparture, Label: "Geh-/Abholweise"},
		ColumnDailyNotes:        {ID: ColumnDailyNotes, Label: "Tageshinweise"},
		ColumnCurrentLocation:   {ID: ColumnCurrentLocation, Label: "Aktueller Aufenthaltsort"},
		ColumnRoomName:          {ID: ColumnRoomName, Label: "Raum"},
		ColumnRoomStatus:        {ID: ColumnRoomStatus, Label: "Status"},
		ColumnRoomBuilding:      {ID: ColumnRoomBuilding, Label: "Gebäude"},
		ColumnRoomFloor:         {ID: ColumnRoomFloor, Label: "Etage"},
		ColumnRoomActivity:      {ID: ColumnRoomActivity, Label: "Aktivität"},
		ColumnRoomSupervision:   {ID: ColumnRoomSupervision, Label: "Aufsicht"},
		ColumnRoomChildCount:    {ID: ColumnRoomChildCount, Label: "Kinder"},
		ColumnChecklist:         {ID: ColumnChecklist, Label: "Abhaken"},
		ColumnStudentName:       {ID: ColumnStudentName, Label: "Kind"},
		ColumnStudentClass:      {ID: ColumnStudentClass, Label: "Klasse"},
		ColumnStudentGroup:      {ID: ColumnStudentGroup, Label: "Aufsichtsgruppe"},
		ColumnContactName:       {ID: ColumnContactName, Label: "Kontakt"},
		ColumnIBAN:              {ID: ColumnIBAN, Label: "IBAN"},
		ColumnContactPhone:      {ID: ColumnContactPhone, Label: "Telefonnummer"},
		ColumnGuardianContacts:  {ID: ColumnGuardianContacts, Label: "Erziehungsberechtigte"},
		ColumnBirthday:          {ID: ColumnBirthday, Label: "Geburtstag"},
		ColumnAge:               {ID: ColumnAge, Label: "Alter"},
	}
}

func ResolveColumns(ids []ColumnID, preset Preset) []Column {
	if preset == PresetHealthList {
		return HealthListColumns(ids)
	}
	if len(ids) == 0 {
		ids = DefaultColumnsForPreset(preset)
	}
	catalog := ColumnCatalog()
	seen := make(map[ColumnID]struct{}, len(ids))
	columns := make([]Column, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		column, ok := catalog[id]
		if !ok {
			continue
		}
		seen[id] = struct{}{}
		columns = append(columns, column)
	}
	if len(columns) == 0 {
		for _, id := range DefaultColumnsForPreset(PresetOGSWeekly) {
			columns = append(columns, catalog[id])
		}
	}
	return columns
}

// HealthListColumns resolves the columns of the Gesundheitsliste (#3323). The
// caller may drop the class and the group; the name and the health note are
// always printed, because a note without its child, or the list without its
// note, is not the list that was asked for. An empty request means every
// column.
func HealthListColumns(ids []ColumnID) []Column {
	catalog := ColumnCatalog()
	requested := make(map[ColumnID]bool, len(ids))
	for _, id := range ids {
		requested[id] = true
	}
	columns := []Column{catalog[ColumnName]}
	for _, id := range []ColumnID{ColumnSchoolClass, ColumnGroup} {
		if len(ids) == 0 || requested[id] {
			columns = append(columns, catalog[id])
		}
	}
	return append(columns, Column{ID: ColumnHealthInfo, Label: "Gesundheitsinformationen"})
}

func GeneratedAtLabel(t time.Time) string {
	if t.IsZero() {
		t = time.Now()
	}
	return t.Format("02.01.2006 15:04")
}
