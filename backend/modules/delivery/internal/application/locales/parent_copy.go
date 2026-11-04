package locales

// Existing stored copy-kind protocol values.
const (
	ParentAnnouncementReminder = "announcement_reminder"
	ParentCareCancelled        = "care_cancelled"
	ParentPollPublished        = "poll"
	ParentPollReminder         = "poll_reminder"
)

// ParentAnnouncementCopy is the canonical parent announcement and poll copy.
func ParentAnnouncementCopy(locale, kind string) (string, string) {
	switch Normalize(locale) {
	case "en":
		return parentAnnouncementCopyEN(kind)
	case "ru":
		return parentAnnouncementCopyRU(kind)
	case "sq":
		return parentAnnouncementCopySQ(kind)
	case "pl":
		return parentAnnouncementCopyPL(kind)
	case "tr":
		return parentAnnouncementCopyTR(kind)
	case "uk":
		return parentAnnouncementCopyUK(kind)
	default:
		return parentAnnouncementCopyDE(kind)
	}
}

func parentAnnouncementCopyEN(kind string) (string, string) {
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
}

func parentAnnouncementCopyRU(kind string) (string, string) {
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
}

func parentAnnouncementCopySQ(kind string) (string, string) {
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
}

func parentAnnouncementCopyPL(kind string) (string, string) {
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
}

func parentAnnouncementCopyTR(kind string) (string, string) {
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
}

func parentAnnouncementCopyUK(kind string) (string, string) {
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
}

func parentAnnouncementCopyDE(kind string) (string, string) {
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

// ParentMessageCopy is the canonical generic parent message notice.
func ParentMessageCopy(locale string) (string, string) {
	switch Normalize(locale) {
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

// ParentBillingDocumentCopy is the push that a Träger put a document in the
// inbox of the parents portal (#4671). Like every parent push it names no
// child and no amount; the document waits in the portal.
func ParentBillingDocumentCopy(locale string) (string, string) {
	switch Normalize(locale) {
	case "en":
		return "New document from the provider", "You have a new document in the parent portal."
	case "ru":
		return "Новый документ от организации", "У вас новый документ в родительском портале."
	case "sq":
		return "Dokument i ri nga ofruesi", "Keni një dokument të ri në portalin e prindërve."
	case "pl":
		return "Nowy dokument od organizatora", "Mają Państwo nowy dokument w portalu dla rodziców."
	case "tr":
		return "Kurumdan yeni belge", "Veli portalında yeni bir belgeniz var."
	case "uk":
		return "Новий документ від організації", "У вас новий документ у батьківському порталі."
	default:
		return "Neues Dokument vom Träger", "Sie haben ein neues Dokument im Elternportal."
	}
}
