package defaults

import "github.com/moto-nrw/project-phoenix/models/config"

func init() {
	for index, report := range []struct{ key, label string }{
		{config.KeyParentSickReportsEnabled, "Krankmeldungen durch Eltern"},
		{config.KeyParentExcusedReportsEnabled, "Entschuldigungen durch Eltern"},
	} {
		config.Register(config.Definition{
			Key: report.key, Label: report.label,
			Description: "Elternmeldungen ändern den Tagesstatus, melden aber kein Kind an.",
			Type:        config.FieldBoolean, Default: true,
			ReadPermission: "config:read", WritePermission: "config:manage",
			Tab: "operations", Category: "elternmeldungen", SortOrder: 6 + index,
		})
	}
	config.Register(config.Definition{
		Key:             config.KeyParentAbsenceReviewScope,
		Label:           "Wer bearbeitet diese Elternanfragen?",
		Description:     "Gilt nur für Krankmeldungen und Entschuldigungen durch Eltern.",
		Type:            config.FieldSelect,
		Default:         config.ParentAbsenceReviewScopeInherit,
		ReadPermission:  "config:read",
		WritePermission: "config:manage",
		Tab:             "operations",
		Category:        "elternmeldungen",
		SortOrder:       8,
		Options: &config.SelectOptions{Static: []config.SelectOption{
			{Label: "Bisherige Freigabe", Value: config.ParentAbsenceReviewScopeInherit},
			{Label: "Admins", Value: config.ParentAbsenceReviewScopeAdmins},
			{Label: "Admins und zuständige Gruppenleitungen", Value: config.ParentAbsenceReviewScopeGroupLeaders},
			{Label: "Berechtigtes Team", Value: config.ParentAbsenceReviewScopeAllStaff},
		}},
	})
	config.Register(config.Definition{
		Key:             config.KeyStudentAbsenceEditScope,
		Label:           "Wer darf Kinder krank oder entschuldigt melden?",
		Description:     "Gilt auch für mehrere Tage und das Zurücknehmen einer Meldung. Elternmeldungen werden getrennt freigegeben.",
		Type:            config.FieldSelect,
		Default:         config.StudentAbsenceEditScopeAllStaff,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "sehen-und-bearbeiten",
		SortOrder:       5,
		Options: &config.SelectOptions{Static: []config.SelectOption{
			{Label: "Nur OGS-Admins", Value: config.StudentAbsenceEditScopeAdmins},
			{Label: "Das ganze Team", Value: config.StudentAbsenceEditScopeAllStaff},
		}},
	})
	config.Register(config.Definition{
		Key:             config.KeyAttendanceEditScope,
		Label:           "Wo darf das Team Kinder an- und abmelden?",
		Description:     "Überall geht nur, wenn das Team alle Gruppen und Blöcke sieht. Die Auswahl ändert keine Aufsicht und keinen Betreuungsplan.",
		Type:            config.FieldSelect,
		Default:         config.AttendanceEditScopeOwn,
		ReadPermission:  "config:read",
		WritePermission: "config:update",
		Tab:             "operations",
		Category:        "sehen-und-bearbeiten",
		SortOrder:       4,
		Options: &config.SelectOptions{Static: []config.SelectOption{
			{Label: "Eigene Zuständigkeiten", Value: config.AttendanceEditScopeOwn},
			{Label: "Überall", Value: config.AttendanceEditScopeAllStaff},
		}},
	})
}
