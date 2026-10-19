package notifications

import "github.com/moto-nrw/project-phoenix/modules/delivery/internal/adapters/localization"

const (
	ParentAppointmentPublished  = "published"
	ParentAppointmentUpdated    = "updated"
	ParentAppointmentCancelled  = "cancelled"
	ParentAppointmentReminder   = "reminder"
	ParentAnnouncementPublished = "announcement"
	ParentPollPublished         = "poll"
	ParentPollReminder          = "poll_reminder"
	// ParentAnnouncementReminder is the scheduled second delivery of an
	// announcement (#3162). Generic like every parent push: the wording waits
	// in the portal.
	ParentAnnouncementReminder = "announcement_reminder"
	// ParentCareCancelled is the push for a system-authored cancellation
	// notice (#2601). Deliberately names the event: a family should know a
	// block fell out before opening the app.
	ParentCareCancelled = "care_cancelled"
)

// IsSupportedLocale reports whether locale names one of the languages the
// guardian portal and the parent copy below are offered in.
func IsSupportedLocale(locale string) bool {
	return localization.IsSupported(locale)
}

func ParentRequestDecisionCopy(locale, requestType, requestStatus string) (string, string) {
	switch localization.NormalizeLocale(locale) {
	case "en":
		subject := parentRequestSubjectEN(requestType)
		if requestStatus == "abgelehnt" {
			return "Request rejected", subject + " was rejected."
		}
		return "Request approved", subject + " was approved."
	case "ru":
		subject := parentRequestSubjectRU(requestType)
		if requestStatus == "abgelehnt" {
			return "Запрос отклонён", subject + " отклонён."
		}
		return "Запрос одобрен", subject + " одобрен."
	case "sq":
		subject := parentRequestSubjectSQ(requestType)
		if requestStatus == "abgelehnt" {
			return "Kërkesa u refuzua", subject + " u refuzua."
		}
		return "Kërkesa u miratua", subject + " u miratua."
	case "pl":
		// The subject is in the accusative: "OGS odrzuciła Państwa prośbę".
		// That keeps the verb form independent of the subject's gender.
		subject := parentRequestSubjectPL(requestType)
		if requestStatus == "abgelehnt" {
			return "Prośba odrzucona", "OGS odrzuciła " + subject + "."
		}
		return "Prośba zaakceptowana", "OGS zaakceptowała " + subject + "."
	case "tr":
		subject := parentRequestSubjectTR(requestType)
		if requestStatus == "abgelehnt" {
			return "Talep reddedildi", subject + " reddedildi."
		}
		return "Talep onaylandı", subject + " onaylandı."
	case "uk":
		subject := parentRequestSubjectUK(requestType)
		if requestStatus == "abgelehnt" {
			return "Запит відхилено", subject + " відхилено."
		}
		return "Запит схвалено", subject + " схвалено."
	default:
		subject := parentRequestSubjectDE(requestType)
		if requestStatus == "abgelehnt" {
			return "Anfrage abgelehnt", subject + " wurde abgelehnt."
		}
		return "Anfrage genehmigt", subject + " wurde genehmigt."
	}
}

func parentRequestSubjectDE(requestType string) string {
	switch requestType {
	case "care_schedule":
		return "Ihre Anfrage zu den Betreuungszeiten"
	case "pickup_change":
		return "Ihre Anfrage zur Abholung"
	case "master_data":
		return "Ihre Anfrage zu den Stammdaten"
	case "excused_absence":
		return "Ihre Abmeldung"
	case "sick_absence":
		return "Ihre Krankmeldung"
	default:
		return "Ihre Anfrage"
	}
}

func parentRequestSubjectEN(requestType string) string {
	switch requestType {
	case "care_schedule":
		return "Your care schedule request"
	case "pickup_change":
		return "Your pickup request"
	case "master_data":
		return "Your master data request"
	case "excused_absence":
		return "Your absence notice"
	case "sick_absence":
		return "Your sick note"
	default:
		return "Your request"
	}
}

func parentRequestSubjectRU(requestType string) string {
	switch requestType {
	case "care_schedule":
		return "Ваш запрос об изменении времени продлёнки"
	case "pickup_change":
		return "Ваш запрос о том, как ребёнок уходит домой"
	case "master_data":
		return "Ваш запрос об изменении основных данных"
	case "excused_absence":
		return "Ваше уведомление об отсутствии"
	case "sick_absence":
		return "Ваше уведомление о болезни"
	default:
		return "Ваш запрос"
	}
}

func parentRequestSubjectSQ(requestType string) string {
	switch requestType {
	case "care_schedule":
		return "Kërkesa juaj për orarin e kujdesit"
	case "pickup_change":
		return "Kërkesa juaj për marrjen e fëmijës"
	case "master_data":
		return "Kërkesa juaj për të dhënat bazë"
	case "excused_absence":
		return "Njoftimi juaj për mungesën"
	case "sick_absence":
		return "Njoftimi juaj për sëmundjen"
	default:
		return "Kërkesa juaj"
	}
}

func parentRequestSubjectPL(requestType string) string {
	switch requestType {
	case "care_schedule":
		return "Państwa prośbę o zmianę godzin opieki"
	case "pickup_change":
		return "Państwa prośbę o zmianę odbioru"
	case "master_data":
		return "Państwa prośbę o zmianę danych podstawowych"
	case "excused_absence":
		return "Państwa zgłoszenie nieobecności"
	case "sick_absence":
		return "Państwa zgłoszenie choroby"
	default:
		return "Państwa prośbę"
	}
}

func parentRequestSubjectTR(requestType string) string {
	switch requestType {
	case "care_schedule":
		return "Bakım saatleri talebiniz"
	case "pickup_change":
		return "Teslim alma talebiniz"
	case "master_data":
		return "Temel bilgiler talebiniz"
	case "excused_absence":
		return "Mazeret bildiriminiz"
	case "sick_absence":
		return "Hastalık bildiriminiz"
	default:
		return "Talebiniz"
	}
}

func parentRequestSubjectUK(requestType string) string {
	switch requestType {
	case "care_schedule":
		return "Ваш запит щодо годин догляду"
	case "pickup_change":
		return "Ваш запит щодо того, як дитина йде додому"
	case "master_data":
		return "Ваш запит щодо основних даних"
	case "excused_absence":
		return "Ваше повідомлення про відсутність"
	case "sick_absence":
		return "Ваше повідомлення про хворобу"
	default:
		return "Ваш запит"
	}
}

func ParentAppointmentCopy(locale, kind string) (string, string) {
	switch localization.NormalizeLocale(locale) {
	case "en":
		return parentAppointmentCopyEN(kind)
	case "ru":
		return parentAppointmentCopyRU(kind)
	case "sq":
		return parentAppointmentCopySQ(kind)
	case "pl":
		return parentAppointmentCopyPL(kind)
	case "tr":
		return parentAppointmentCopyTR(kind)
	case "uk":
		return parentAppointmentCopyUK(kind)
	default:
		return parentAppointmentCopyDE(kind)
	}
}

func parentAppointmentCopyPL(kind string) (string, string) {
	switch kind {
	case ParentAppointmentUpdated:
		return "Termin zmieniony", "Termin dla Państwa został zmieniony."
	case ParentAppointmentCancelled:
		return "Termin odwołany", "Termin dla Państwa został odwołany."
	case ParentAppointmentReminder:
		return "Przypomnienie o terminie", "Wkrótce mają Państwo termin."
	default:
		return "Nowy termin", "Wpisano dla Państwa nowy termin."
	}
}

func parentAppointmentCopyTR(kind string) (string, string) {
	switch kind {
	case ParentAppointmentUpdated:
		return "Randevu değişti", "Sizin için bir randevu değiştirildi."
	case ParentAppointmentCancelled:
		return "Randevu iptal edildi", "Sizin için bir randevu iptal edildi."
	case ParentAppointmentReminder:
		return "Randevu hatırlatması", "Yakında bir randevunuz var."
	default:
		return "Yeni randevu", "Sizin için yeni bir randevu eklendi."
	}
}

func parentAppointmentCopyUK(kind string) (string, string) {
	switch kind {
	case ParentAppointmentUpdated:
		return "Подію змінено", "Подію для вас змінено."
	case ParentAppointmentCancelled:
		return "Подію скасовано", "Подію для вас скасовано."
	case ParentAppointmentReminder:
		return "Нагадування про подію", "Незабаром відбудеться подія для вас."
	default:
		return "Нова подія", "Для вас додано нову подію."
	}
}

func parentAppointmentCopyDE(kind string) (string, string) {
	switch kind {
	case ParentAppointmentUpdated:
		return "Termin geändert", "Ein Termin für Sie wurde geändert."
	case ParentAppointmentCancelled:
		return "Termin abgesagt", "Ein Termin für Sie wurde abgesagt."
	case ParentAppointmentReminder:
		return "Terminerinnerung", "Ein Termin für Sie steht bald an."
	default:
		return "Neuer Termin", "Für Sie wurde ein neuer Termin eingetragen."
	}
}

func parentAppointmentCopyEN(kind string) (string, string) {
	switch kind {
	case ParentAppointmentUpdated:
		return "Appointment changed", "An appointment for you has been changed."
	case ParentAppointmentCancelled:
		return "Appointment cancelled", "An appointment for you has been cancelled."
	case ParentAppointmentReminder:
		return "Appointment reminder", "You have an appointment coming up soon."
	default:
		return "New appointment", "A new appointment has been added for you."
	}
}

func parentAppointmentCopyRU(kind string) (string, string) {
	switch kind {
	case ParentAppointmentUpdated:
		return "Событие изменено", "Событие для вас было изменено."
	case ParentAppointmentCancelled:
		return "Событие отменено", "Событие для вас было отменено."
	case ParentAppointmentReminder:
		return "Напоминание о событии", "Скоро начнётся событие, на которое вы приглашены."
	default:
		return "Новое событие", "Для вас добавлено новое событие."
	}
}

func parentAppointmentCopySQ(kind string) (string, string) {
	switch kind {
	case ParentAppointmentUpdated:
		return "Takimi u ndryshua", "Një takim për ju është ndryshuar."
	case ParentAppointmentCancelled:
		return "Takimi u anulua", "Një takim për ju është anuluar."
	case ParentAppointmentReminder:
		return "Kujtesë për takim", "Së shpejti keni një takim."
	default:
		return "Takim i ri", "Është shtuar një takim i ri për ju."
	}
}
