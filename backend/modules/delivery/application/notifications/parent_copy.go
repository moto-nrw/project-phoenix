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
	// ParentAnnouncementReminder is the scheduled second delivery of an
	// announcement (#3162). Generic like every parent push: the wording waits
	// in the portal.
	ParentAnnouncementReminder = "announcement_reminder"
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
		case ParentAnnouncementReminder:
			return "Reminder: parent announcement", "Your after-school care reminds you of an announcement in the parent portal."
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
		case ParentAnnouncementReminder:
			return "Напоминание: объявление", "Продлёнка напоминает вам об объявлении в родительском портале."
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
		case ParentAnnouncementReminder:
			return "Kujtesë: njoftim për prindërit", "Kujdesi pas shkollës ju kujton një njoftim në portalin e prindërve."
		case ParentCareCancelled:
			return "Kujdesi anulohet", "Kujdesi pas shkollës ka anuluar një nga takimet e fëmijës suaj."
		default:
			return "Njoftim i ri për prindërit", "Një njoftim i ri është i disponueshëm në portalin e prindërve."
		}
	case "pl":
		switch kind {
		case ParentPollPublished:
			return "Nowa ankieta", "Szkoła prosi o Państwa odpowiedź w portalu dla rodziców."
		case ParentPollReminder:
			return "Przypomnienie: ankieta otwarta", "W portalu dla rodziców brakuje jeszcze odpowiedzi za Państwa dziecko."
		case ParentAnnouncementReminder:
			return "Przypomnienie: ogłoszenie dla rodziców", "OGS przypomina o ogłoszeniu w portalu dla rodziców."
		case ParentCareCancelled:
			return "Opieka odwołana", "OGS odwołała jeden z terminów opieki Państwa dziecka."
		default:
			return "Nowe ogłoszenie dla rodziców", "W portalu dla rodziców jest nowe ogłoszenie."
		}
	case "tr":
		switch kind {
		case ParentPollPublished:
			return "Yeni anket", "Bir okul veli portalında yanıtınızı bekliyor."
		case ParentPollReminder:
			return "Hatırlatma: anket açık", "Veli portalında çocuğunuz için bir yanıt hâlâ eksik."
		case ParentAnnouncementReminder:
			return "Hatırlatma: veli duyurusu", "OGS, veli portalındaki bir duyuruyu hatırlatıyor."
		case ParentCareCancelled:
			return "Bakım iptal edildi", "OGS, çocuğunuzun bakım günlerinden birini iptal etti."
		default:
			return "Veliler için yeni duyuru", "Veli portalında yeni bir duyuru var."
		}
	case "uk":
		switch kind {
		case ParentPollPublished:
			return "Нове опитування", "Школа просить вас відповісти в батьківському порталі."
		case ParentPollReminder:
			return "Нагадування: опитування відкрите", "У батьківському порталі ще немає відповіді за вашу дитину."
		case ParentAnnouncementReminder:
			return "Нагадування: оголошення для батьків", "OGS нагадує вам про оголошення в батьківському порталі."
		case ParentCareCancelled:
			return "Догляд скасовано", "OGS скасувала один із днів догляду вашої дитини."
		default:
			return "Нове оголошення для батьків", "У батьківському порталі є нове оголошення."
		}
	default:
		switch kind {
		case ParentPollPublished:
			return "Neue Umfrage", "Eine Schule bittet um Ihre Rückmeldung im Elternportal."
		case ParentPollReminder:
			return "Erinnerung: Umfrage offen", "Für Ihr Kind fehlt noch eine Rückmeldung im Elternportal."
		case ParentAnnouncementReminder:
			return "Erinnerung: Elternmitteilung", "Ihre OGS erinnert Sie an eine Mitteilung im Elternportal."
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

func parentRequestSubjectPL(requestType string) string {
	switch requestType {
	case "care_schedule":
		return "Państwa prośbę o zmianę godzin opieki"
	case "pickup_change":
		return "Państwa prośbę o zmianę godziny odbioru"
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
		return "Teslim alma saati talebiniz"
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
		return "Ваш запит щодо часу, коли забирають дитину"
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

func ParentMessageCopy(locale string) (string, string) {
	switch localization.NormalizeLocale(locale) {
	case "en":
		return "New message from the OGS", "You have a new message in the parent portal."
	case "ru":
		return "Новое сообщение от продлёнки", "У вас новое сообщение в родительском портале."
	case "sq":
		return "Mesazh i ri nga OGS-ja", "Keni një mesazh të ri në portalin e prindërve."
	case "pl":
		return "Nowa wiadomość od OGS", "Mają Państwo nową wiadomość w portalu dla rodziców."
	case "tr":
		return "OGS'den yeni mesaj", "Veli portalında yeni bir mesajınız var."
	case "uk":
		return "Нове повідомлення від OGS", "У вас нове повідомлення в батьківському порталі."
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
