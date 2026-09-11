package notifications

import "github.com/moto-nrw/project-phoenix/localization"

const (
	ParentAppointmentPublished  = "published"
	ParentAppointmentUpdated    = "updated"
	ParentAppointmentCancelled  = "cancelled"
	ParentAppointmentReminder   = "reminder"
	ParentAnnouncementPublished = "announcement"
	ParentPollPublished         = "poll"
	ParentPollReminder          = "poll_reminder"
	// ParentCareCancelled is the push for a system-authored cancellation
	// notice (#2601). Deliberately names the event: a family should know a
	// block fell out before opening the app.
	ParentCareCancelled = "care_cancelled"
)

func ParentAnnouncementCopy(locale, kind string) (string, string) {
	switch localization.NormalizeLocale(locale) {
	case "en":
		switch kind {
		case ParentPollPublished:
			return "New poll", "A school is asking for your response in the parent portal."
		case ParentPollReminder:
			return "Reminder: poll open", "A response for your child is still missing in the parent portal."
		case ParentCareCancelled:
			return "Care cancelled", "The after-school care has cancelled one of your child's sessions."
		default:
			return "New parent announcement", "A new announcement is available in the parent portal."
		}
	case "ru":
		switch kind {
		case ParentPollPublished:
			return "Новый опрос", "Школа просит вас ответить в родительском портале."
		case ParentPollReminder:
			return "Напоминание: опрос открыт", "В родительском портале ещё нет ответа за вашего ребёнка."
		case ParentCareCancelled:
			return "Занятие отменено", "Группа продлённого дня отменила одно из занятий вашего ребёнка."
		default:
			return "Новое объявление для родителей", "В родительском портале доступно новое объявление."
		}
	case "sq":
		switch kind {
		case ParentPollPublished:
			return "Anketë e re", "Një shkollë kërkon përgjigjen tuaj në portalin e prindërve."
		case ParentPollReminder:
			return "Kujtesë: anketa është e hapur", "Në portalin e prindërve mungon ende një përgjigje për fëmijën tuaj."
		case ParentCareCancelled:
			return "Kujdesi anulohet", "Kujdesi pas shkollës ka anuluar një nga takimet e fëmijës suaj."
		default:
			return "Njoftim i ri për prindërit", "Një njoftim i ri është i disponueshëm në portalin e prindërve."
		}
	default:
		switch kind {
		case ParentPollPublished:
			return "Neue Umfrage", "Eine Schule bittet um Ihre Rückmeldung im Elternportal."
		case ParentPollReminder:
			return "Erinnerung: Umfrage offen", "Für Ihr Kind fehlt noch eine Rückmeldung im Elternportal."
		case ParentCareCancelled:
			return "Betreuung fällt aus", "Die OGS hat einen Betreuungstermin Ihres Kindes abgesagt."
		default:
			return "Neue Elternmitteilung", "Eine neue Mitteilung ist im Elternportal verfügbar."
		}
	}
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
		return "Ihre Anfrage zur Abholzeit"
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
		return "Your pickup time request"
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
		return "Ваш запрос об изменении времени, когда ребёнка забирают"
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
		return "Kërkesa juaj për orarin e marrjes"
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

func ParentMessageCopy(locale string) (string, string) {
	switch localization.NormalizeLocale(locale) {
	case "en":
		return "New message from the OGS", "You have a new message in the parent portal."
	case "ru":
		return "Новое сообщение от продлёнки", "У вас новое сообщение в родительском портале."
	case "sq":
		return "Mesazh i ri nga OGS-ja", "Keni një mesazh të ri në portalin e prindërve."
	default:
		return "Neue Nachricht der OGS", "Sie haben eine neue Nachricht im Elternportal."
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
	default:
		return parentAppointmentCopyDE(kind)
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
