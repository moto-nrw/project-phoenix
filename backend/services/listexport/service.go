package listexport

import (
	"fmt"
	"strings"
	"time"
)

// RendererService renders list/record documents to PDF, XLSX, and DOCX files.
type RendererService struct{}

// NewService creates the document renderer.
func NewService() *RendererService {
	return &RendererService{}
}

func (s *RendererService) Render(doc Document, format Format, filenameBase string) (File, error) {
	base := safeFilename(filenameBase)
	if base == "" {
		base = "export"
	}
	switch format {
	case FormatPDF:
		data, err := renderPDFDesigned(doc)
		if err != nil {
			return File{}, err
		}
		return File{Data: data, ContentType: "application/pdf", Filename: base + ".pdf"}, nil
	case FormatDOCX:
		data, err := renderDOCX(doc)
		if err != nil {
			return File{}, err
		}
		return File{Data: data, ContentType: "application/vnd.openxmlformats-officedocument.wordprocessingml.document", Filename: base + ".docx"}, nil
	case FormatXLSX:
		data, err := renderXLSX(doc)
		if err != nil {
			return File{}, err
		}
		return File{Data: data, ContentType: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", Filename: base + ".xlsx"}, nil
	default:
		return File{}, fmt.Errorf("unsupported export format %q", format)
	}
}

func DefaultColumnsForPreset(preset Preset) []ColumnID {
	switch preset {
	case PresetOGSCompact:
		return []ColumnID{ColumnName, ColumnSchoolClass, ColumnGroup, ColumnCareDays, ColumnDeparture, ColumnPlannedPickup}
	case PresetClassRoster:
		return []ColumnID{ColumnName, ColumnSchoolClass, ColumnGroup, ColumnEnrollmentSummary, ColumnCareDays, ColumnWeeklyMonday, ColumnWeeklyTuesday, ColumnWeeklyWednesday, ColumnWeeklyThursday, ColumnWeeklyFriday, ColumnDeparture}
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
		ColumnContactPhone:      {ID: ColumnContactPhone, Label: "Telefonnummer"},
		ColumnGuardianContacts:  {ID: ColumnGuardianContacts, Label: "Erziehungsberechtigte"},
		ColumnBirthday:          {ID: ColumnBirthday, Label: "Geburtstag"},
		ColumnAge:               {ID: ColumnAge, Label: "Alter"},
	}
}

func ResolveColumns(ids []ColumnID, preset Preset) []Column {
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

func GeneratedAtLabel(t time.Time) string {
	if t.IsZero() {
		t = time.Now()
	}
	return t.Format("02.01.2006 15:04")
}

func safeFilename(input string) string {
	input = strings.TrimSpace(strings.ToLower(input))
	replacer := strings.NewReplacer(
		"ä", "ae", "ö", "oe", "ü", "ue", "ß", "ss",
		" ", "-", "/", "-", "\\", "-", ":", "-", ";", "-",
	)
	input = replacer.Replace(input)
	var b strings.Builder
	lastDash := false
	for _, r := range input {
		ok := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
		if ok {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}
