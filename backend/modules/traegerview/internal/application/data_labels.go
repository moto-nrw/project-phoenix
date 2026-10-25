package application

// dataLabels says in plain German which data an access log entry read or
// changed. The keys are the resource types of audit.data_access_log and the
// change ledgers of the Audit Platform's access trail; both are stored values
// that never change.
var dataLabels = map[string]string{
	"attendance_history":          "Anwesenheit eines Kindes",
	"attendance_day_log":          "Tagesprotokoll einer Gruppe",
	"student_status_day_overview": "Abwesenheiten einer Gruppe",
	"enrollment_phase_export":     "Anmeldungen einer Anmeldephase (Export)",
	"enrollment_student_export":   "Anmeldung eines Kindes (Export)",
	"enrollment_upload_view":      "Nachweis aus einer Anmeldung",
	"staff_financial_view":        "Bank- und Steuerdaten einer Fachkraft",
	"staff_financial_reveal":      "Bank- und Steuerdaten einer Fachkraft (vollständig)",
	"guardian_financial_view":     "Bankverbindung von Eltern",
	"guardian_financial_reveal":   "Bankverbindung von Eltern (vollständig)",
	"guardian_payment_overview":   "Liste der Bankverbindungen",
	"guardian_payment_export":     "Liste der Bankverbindungen (Export)",
	"staff_document_download":     "Dokument einer Fachkraft",
	"student_document_download":   "Dokument eines Kindes",
	"class_day_view":              "Tagesansicht einer Klasse",
	"supervision_student_sheet":   "Abholung und Notfallkontakte eines Kindes",
	"student_health_list_export":  "Gesundheitsliste (Export)",
	"attendance_statistics":       "Statistik der Anwesenheit",
	"time_tracking_export":        "Arbeitszeiten des Personals (Export)",
	"traeger_access_log":          "Zugriffsprotokoll im Träger-Portal",
	"traeger_access_log_export":   "Zugriffsprotokoll im Träger-Portal (Export)",
	"student_record":              "Angaben zu einem Kind",
	"guardian_record":             "Angaben zu Eltern",
	"guardian_bank_details":       "Bankverbindung von Eltern",
	"student_consent":             "Einwilligung für ein Kind",
	"staff_record":                "Angaben zu einer Fachkraft",
	"staff_personnel_number":      "Personalnummer einer Fachkraft",
	"attendance_correction":       "Anwesenheit eines Kindes",
	"student_deletion":            "Kind mit allen Daten gelöscht",
}

// otherData labels a resource type a later feature adds before this list
// knows it.
const otherData = "Sonstige personenbezogene Daten"

// dataLabel says in plain German which data an entry read or changed.
func dataLabel(data string) string {
	if label, ok := dataLabels[data]; ok {
		return label
	}
	return otherData
}
