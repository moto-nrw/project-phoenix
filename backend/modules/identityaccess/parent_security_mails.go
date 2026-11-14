package identityaccess

import (
	"fmt"
	"strings"
)

// The parents portal speaks the language the parent chose (#7051), so the
// sign-in code and the notice of a remembered device go out in it as well.
// The languages are the parents portal's; any other locale reads German.

// parentCodeCopy is the text of a parents-portal sign-in code.
type parentCodeCopy struct {
	Subject, Title, Greeting, Intro, CodeLabel, Validity, RequestFrom, Tip, Security string
}

// parentDeviceCopy is the text of the notice of a remembered device.
type parentDeviceCopy struct {
	Subject, Title, Greeting, Intro, DeviceLabel, IPLabel, TimeLabel, NotYouTitle, NotYou, Recovery string
}

// parentResetCopy is the notice of a reset of the code, as a parent reads it.
type parentResetCopy struct {
	Subject, Title, Greeting, Body, Again, NotYouTitle, NotYou string
}

// parentCopy is one language of the parents' security mails. FooterText and
// PoweredByLabel are the footer of an automatic mail, as the parents
// portal's other mails write it: the sentence and the label beside the moto
// logo.
type parentCopy struct {
	Code                       parentCodeCopy
	Device                     parentDeviceCopy
	Reset                      parentResetCopy
	FooterText, PoweredByLabel string
}

// parentCopies is every text of the parents' security mails, by language.
// Code: %[1]d is the validity of the code in minutes, %[2]d the days a
// device stays remembered. Device: %[1]d is the days the device stays
// remembered. Reset: %[1]s is the moment of the reset.
var parentCopies = map[string]parentCopy{
	"de": {
		Code: parentCodeCopy{
			Subject:     "Ihr Anmeldecode für moto",
			Title:       "Ihr Anmeldecode",
			Greeting:    "Hallo,",
			Intro:       "jemand meldet sich gerade mit Ihrer E-Mail-Adresse im Eltern-Portal von moto an. Geben Sie diesen Code auf der Anmeldeseite ein:",
			CodeLabel:   "Ihr Code",
			Validity:    "Der Code gilt %[1]d Minuten und nur ein einziges Mal.",
			RequestFrom: "Anfrage von IP-Adresse",
			Tip:         "Tipp: Auf Ihrem eigenen Gerät können Sie nach dem Code „Ja, merken und anmelden“ wählen. Dann fragt moto dort %[2]d Tage lang nicht nach einem Code.",
			Security:    "Sie haben sich nicht angemeldet? Dann löschen Sie diese E-Mail. An Ihrem Konto ändert sich nichts. moto fragt nie am Telefon, im Chat oder per E-Mail nach diesem Code.",
		},
		Device: parentDeviceCopy{
			Subject:     "moto hat sich ein neues Gerät gemerkt",
			Title:       "Neues gemerktes Gerät",
			Greeting:    "Hallo,",
			Intro:       "Sie haben sich gerade im Eltern-Portal angemeldet und „Ja, merken und anmelden“ gewählt. Auf diesem Gerät fragt moto %[1]d Tage lang nicht nach einem Code.",
			DeviceLabel: "Gerät",
			IPLabel:     "IP-Adresse",
			TimeLabel:   "Zeitpunkt",
			NotYouTitle: "Das waren nicht Sie?",
			NotYou:      "Ändern Sie sofort Ihr Passwort. Entfernen Sie danach das Gerät im Eltern-Portal unter „Einstellungen → Anmeldung“.",
			Recovery:    "Sie kommen nicht mehr in Ihr Konto? Wenden Sie sich an Ihre Schule. Das moto-Team kann dann die Anmeldung mit Code zurücksetzen.",
		},
		Reset: parentResetCopy{
			Subject: "Ihre Anmeldung mit Code wurde zurückgesetzt", Title: "Ihre Anmeldung mit Code wurde zurückgesetzt", Greeting: "Hallo,",
			Body:        "Am %[1]s wurde die Anmeldung mit Code für Ihr moto-Konto zurückgesetzt. Ihre gemerkten Geräte sind gelöscht. Sie melden sich jetzt wieder nur mit Ihrem Passwort an.",
			Again:       "Im Eltern-Portal schalten Sie die Anmeldung mit Code unter „Einstellungen → Anmeldung“ wieder ein.",
			NotYouTitle: "Haben Sie das nicht beauftragt?",
			NotYou:      "Ändern Sie sofort Ihr Passwort: Wählen Sie auf der Anmeldeseite „Passwort vergessen?“. Melden Sie sich danach bei Ihrer Schule.",
		},
		FooterText: "Diese E-Mail wurde automatisch versendet.", PoweredByLabel: "Unterstützt von",
	},
	"en": {
		Code: parentCodeCopy{
			Subject:     "Your moto sign-in code",
			Title:       "Your sign-in code",
			Greeting:    "Hello,",
			Intro:       "someone is signing in to the moto parents portal with your email address right now. Enter this code on the sign-in page:",
			CodeLabel:   "Your code",
			Validity:    "The code is valid for %[1]d minutes and can be used only once.",
			RequestFrom: "Request from IP address",
			Tip:         "Tip: on your own device you can choose “Yes, remember and sign in” after the code. moto will then not ask for a code on that device for %[2]d days.",
			Security:    "You did not sign in? Then delete this email. Nothing changes in your account. moto never asks for this code by phone, chat or email.",
		},
		Device: parentDeviceCopy{
			Subject:     "moto remembered a new device",
			Title:       "New remembered device",
			Greeting:    "Hello,",
			Intro:       "you just signed in to the parents portal and chose “Yes, remember and sign in”. moto will not ask for a code on this device for %[1]d days.",
			DeviceLabel: "Device",
			IPLabel:     "IP address",
			TimeLabel:   "Time",
			NotYouTitle: "This was not you?",
			NotYou:      "Change your password right away. Then remove the device in the parents portal under “Settings → Sign-in”.",
			Recovery:    "You can no longer get into your account? Contact your school. The moto team can then reset the sign-in with a code.",
		},
		Reset: parentResetCopy{
			Subject: "Your sign-in with a code was reset", Title: "Your sign-in with a code was reset", Greeting: "Hello,",
			Body:        "On %[1]s, sign-in with a code was reset for your moto account. Your remembered devices have been deleted. You now sign in with your password only.",
			Again:       "In the parents portal you can turn sign-in with a code back on under “Settings → Sign-in”.",
			NotYouTitle: "You did not ask for this?",
			NotYou:      "Change your password right away: on the sign-in page choose “Forgot your password?”. Then contact your school.",
		},
		FooterText: "This email was sent automatically.", PoweredByLabel: "Powered by",
	},
	"ru": {
		Code: parentCodeCopy{
			Subject:     "Ваш код для входа в moto",
			Title:       "Ваш код для входа",
			Greeting:    "Здравствуйте!",
			Intro:       "Кто-то сейчас входит в родительский портал moto с вашим адресом электронной почты. Введите этот код на странице входа:",
			CodeLabel:   "Ваш код",
			Validity:    "Код действует %[1]d минут и только один раз.",
			RequestFrom: "Запрос с IP-адреса",
			Tip:         "Совет: на своём устройстве после ввода кода можно выбрать «Да, запомнить и войти». Тогда moto не будет запрашивать код на этом устройстве %[2]d дней.",
			Security:    "Вы не входили? Тогда удалите это письмо. В вашей учётной записи ничего не изменится. moto никогда не спрашивает этот код по телефону, в чате или по электронной почте.",
		},
		Device: parentDeviceCopy{
			Subject:     "moto запомнил новое устройство",
			Title:       "Новое запомненное устройство",
			Greeting:    "Здравствуйте!",
			Intro:       "Вы только что вошли в родительский портал и выбрали «Да, запомнить и войти». На этом устройстве moto не будет запрашивать код %[1]d дней.",
			DeviceLabel: "Устройство",
			IPLabel:     "IP-адрес",
			TimeLabel:   "Время",
			NotYouTitle: "Это были не вы?",
			NotYou:      "Сразу смените пароль. Затем удалите устройство в родительском портале в разделе «Настройки → Вход».",
			Recovery:    "Вы больше не можете войти в учётную запись? Обратитесь в свою школу. Команда moto сможет сбросить вход с кодом.",
		},
		Reset: parentResetCopy{
			Subject: "Вход с кодом сброшен", Title: "Вход с кодом сброшен", Greeting: "Здравствуйте!",
			Body:        "%[1]s вход с кодом для вашей учётной записи moto был сброшен. Ваши запомненные устройства удалены. Теперь вы входите только с паролем.",
			Again:       "В родительском портале вход с кодом можно снова включить в разделе «Настройки → Вход».",
			NotYouTitle: "Вы этого не заказывали?",
			NotYou:      "Сразу смените пароль: на странице входа выберите «Забыли пароль?». Затем обратитесь в свою школу.",
		},
		FooterText: "Это письмо отправлено автоматически.", PoweredByLabel: "При поддержке",
	},
	"sq": {
		Code: parentCodeCopy{
			Subject:     "Kodi juaj i hyrjes në moto",
			Title:       "Kodi juaj i hyrjes",
			Greeting:    "Përshëndetje,",
			Intro:       "dikush po hyn tani në portalin e prindërve të moto me adresën tuaj të emailit. Shkruani këtë kod në faqen e hyrjes:",
			CodeLabel:   "Kodi juaj",
			Validity:    "Kodi vlen %[1]d minuta dhe vetëm një herë.",
			RequestFrom: "Kërkesë nga adresa IP",
			Tip:         "Këshillë: në pajisjen tuaj mund të zgjidhni pas kodit „Po, mbaje mend dhe hyr“. Atëherë moto nuk kërkon kod në atë pajisje për %[2]d ditë.",
			Security:    "Nuk keni hyrë ju? Atëherë fshijeni këtë email. Në llogarinë tuaj nuk ndryshon asgjë. moto nuk e kërkon kurrë këtë kod me telefon, në chat ose me email.",
		},
		Device: parentDeviceCopy{
			Subject:     "moto mbajti mend një pajisje të re",
			Title:       "Pajisje e re e mbajtur mend",
			Greeting:    "Përshëndetje,",
			Intro:       "sapo hytë në portalin e prindërve dhe zgjodhët „Po, mbaje mend dhe hyr“. Në këtë pajisje moto nuk kërkon kod për %[1]d ditë.",
			DeviceLabel: "Pajisja",
			IPLabel:     "Adresa IP",
			TimeLabel:   "Koha",
			NotYouTitle: "Nuk ishit ju?",
			NotYou:      "Ndryshoni menjëherë fjalëkalimin. Pastaj hiqeni pajisjen në portalin e prindërve te „Cilësimet → Hyrja“.",
			Recovery:    "Nuk hyni dot më në llogari? Drejtohuni shkollës suaj. Ekipi i moto mund ta rivendosë hyrjen me kod.",
		},
		Reset: parentResetCopy{
			Subject: "Hyrja me kod u rivendos", Title: "Hyrja me kod u rivendos", Greeting: "Përshëndetje,",
			Body:        "Më %[1]s hyrja me kod për llogarinë tuaj moto u rivendos. Pajisjet tuaja të mbajtura mend u fshinë. Tani hyni vetëm me fjalëkalimin tuaj.",
			Again:       "Në portalin e prindërve mund ta aktivizoni përsëri hyrjen me kod te „Cilësimet → Hyrja“.",
			NotYouTitle: "Nuk e keni kërkuar ju këtë?",
			NotYou:      "Ndryshoni menjëherë fjalëkalimin: në faqen e hyrjes zgjidhni „Keni harruar fjalëkalimin?“. Pastaj drejtohuni shkollës suaj.",
		},
		FooterText: "Ky email u dërgua automatikisht.", PoweredByLabel: "Mundësuar nga",
	},
	"pl": {
		Code: parentCodeCopy{
			Subject:     "Państwa kod logowania do moto",
			Title:       "Państwa kod logowania",
			Greeting:    "Dzień dobry,",
			Intro:       "ktoś właśnie loguje się do portalu dla rodziców moto Państwa adresem e-mail. Proszę wpisać ten kod na stronie logowania:",
			CodeLabel:   "Państwa kod",
			Validity:    "Kod jest ważny %[1]d minut i tylko jeden raz.",
			RequestFrom: "Żądanie z adresu IP",
			Tip:         "Wskazówka: na własnym urządzeniu mogą Państwo po wpisaniu kodu wybrać „Tak, zapamiętaj i zaloguj”. Wtedy moto nie poprosi o kod na tym urządzeniu przez %[2]d dni.",
			Security:    "To nie Państwo się logują? Proszę usunąć tę wiadomość. Na Państwa koncie nic się nie zmieni. moto nigdy nie pyta o ten kod przez telefon, czat ani e-mail.",
		},
		Device: parentDeviceCopy{
			Subject:     "moto zapamiętało nowe urządzenie",
			Title:       "Nowe zapamiętane urządzenie",
			Greeting:    "Dzień dobry,",
			Intro:       "właśnie zalogowali się Państwo do portalu dla rodziców i wybrali „Tak, zapamiętaj i zaloguj”. Na tym urządzeniu moto nie poprosi o kod przez %[1]d dni.",
			DeviceLabel: "Urządzenie",
			IPLabel:     "Adres IP",
			TimeLabel:   "Czas",
			NotYouTitle: "To nie byli Państwo?",
			NotYou:      "Proszę od razu zmienić hasło. Następnie proszę usunąć urządzenie w portalu dla rodziców w sekcji „Ustawienia → Logowanie”.",
			Recovery:    "Nie mają Państwo już dostępu do konta? Proszę skontaktować się ze szkołą. Zespół moto może wtedy zresetować logowanie z kodem.",
		},
		Reset: parentResetCopy{
			Subject: "Logowanie z kodem zostało zresetowane", Title: "Logowanie z kodem zostało zresetowane", Greeting: "Dzień dobry,",
			Body:        "Dnia %[1]s logowanie z kodem dla Państwa konta moto zostało zresetowane. Zapamiętane urządzenia zostały usunięte. Teraz logują się Państwo tylko hasłem.",
			Again:       "W portalu dla rodziców mogą Państwo ponownie włączyć logowanie z kodem w sekcji „Ustawienia → Logowanie”.",
			NotYouTitle: "Nie zlecali Państwo tego?",
			NotYou:      "Proszę od razu zmienić hasło: na stronie logowania proszę wybrać „Nie pamiętają Państwo hasła?”. Następnie proszę skontaktować się ze szkołą.",
		},
		FooterText: "Ten e-mail został wysłany automatycznie.", PoweredByLabel: "Obsługiwane przez",
	},
	"tr": {
		Code: parentCodeCopy{
			Subject:     "moto giriş kodunuz",
			Title:       "Giriş kodunuz",
			Greeting:    "Merhaba,",
			Intro:       "şu anda biri e-posta adresinizle moto veli portalına giriş yapıyor. Bu kodu giriş sayfasına girin:",
			CodeLabel:   "Kodunuz",
			Validity:    "Kod %[1]d dakika geçerlidir ve yalnızca bir kez kullanılabilir.",
			RequestFrom: "İstek IP adresi",
			Tip:         "İpucu: Kendi cihazınızda koddan sonra „Evet, hatırla ve giriş yap“ seçeneğini seçebilirsiniz. moto o cihazda %[2]d gün boyunca kod sormaz.",
			Security:    "Giriş yapan siz değil misiniz? Bu e-postayı silin. Hesabınızda hiçbir şey değişmez. moto bu kodu asla telefonla, sohbette veya e-postayla sormaz.",
		},
		Device: parentDeviceCopy{
			Subject:     "moto yeni bir cihazı hatırladı",
			Title:       "Yeni hatırlanan cihaz",
			Greeting:    "Merhaba,",
			Intro:       "veli portalına giriş yaptınız ve „Evet, hatırla ve giriş yap“ seçeneğini seçtiniz. moto bu cihazda %[1]d gün boyunca kod sormaz.",
			DeviceLabel: "Cihaz",
			IPLabel:     "IP adresi",
			TimeLabel:   "Zaman",
			NotYouTitle: "Bu siz değil miydiniz?",
			NotYou:      "Hemen şifrenizi değiştirin. Ardından cihazı veli portalında „Ayarlar → Giriş“ bölümünden kaldırın.",
			Recovery:    "Hesabınıza artık giremiyor musunuz? Okulunuza başvurun. moto ekibi kodla girişi sıfırlayabilir.",
		},
		Reset: parentResetCopy{
			Subject: "Kodla girişiniz sıfırlandı", Title: "Kodla girişiniz sıfırlandı", Greeting: "Merhaba,",
			Body:        "%[1]s tarihinde moto hesabınız için kodla giriş sıfırlandı. Hatırlanan cihazlarınız silindi. Artık yalnızca şifrenizle giriş yaparsınız.",
			Again:       "Veli portalında kodla girişi „Ayarlar → Giriş“ bölümünden yeniden açabilirsiniz.",
			NotYouTitle: "Bunu siz istemediniz mi?",
			NotYou:      "Hemen şifrenizi değiştirin: Giriş sayfasında „Şifrenizi mi unuttunuz?“ seçeneğini seçin. Ardından okulunuza başvurun.",
		},
		FooterText: "Bu e-posta otomatik olarak gönderildi.", PoweredByLabel: "Destekleyen",
	},
	"uk": {
		Code: parentCodeCopy{
			Subject:     "Ваш код для входу в moto",
			Title:       "Ваш код для входу",
			Greeting:    "Вітаємо!",
			Intro:       "Хтось зараз входить у батьківський портал moto з вашою адресою електронної пошти. Введіть цей код на сторінці входу:",
			CodeLabel:   "Ваш код",
			Validity:    "Код дійсний %[1]d хвилин і лише один раз.",
			RequestFrom: "Запит з IP-адреси",
			Tip:         "Порада: на власному пристрої після коду можна вибрати «Так, запам’ятати й увійти». Тоді moto не запитуватиме код на цьому пристрої %[2]d днів.",
			Security:    "Це були не ви? Тоді видаліть цей лист. У вашому обліковому записі нічого не зміниться. moto ніколи не запитує цей код телефоном, у чаті чи електронною поштою.",
		},
		Device: parentDeviceCopy{
			Subject:     "moto запам’ятав новий пристрій",
			Title:       "Новий запам’ятований пристрій",
			Greeting:    "Вітаємо!",
			Intro:       "Ви щойно увійшли в батьківський портал і вибрали «Так, запам’ятати й увійти». На цьому пристрої moto не запитуватиме код %[1]d днів.",
			DeviceLabel: "Пристрій",
			IPLabel:     "IP-адреса",
			TimeLabel:   "Час",
			NotYouTitle: "Це були не ви?",
			NotYou:      "Негайно змініть пароль. Потім видаліть пристрій у батьківському порталі в розділі «Налаштування → Вхід».",
			Recovery:    "Ви більше не можете увійти в обліковий запис? Зверніться до своєї школи. Команда moto зможе скинути вхід із кодом.",
		},
		Reset: parentResetCopy{
			Subject: "Вхід із кодом скинуто", Title: "Вхід із кодом скинуто", Greeting: "Вітаємо!",
			Body:        "%[1]s вхід із кодом для вашого облікового запису moto було скинуто. Ваші запам’ятовані пристрої видалено. Тепер ви входите лише з паролем.",
			Again:       "У батьківському порталі вхід із кодом можна знову увімкнути в розділі «Налаштування → Вхід».",
			NotYouTitle: "Ви цього не замовляли?",
			NotYou:      "Негайно змініть пароль: на сторінці входу виберіть «Забули пароль?». Потім зверніться до своєї школи.",
		},
		FooterText: "Цей лист надіслано автоматично.", PoweredByLabel: "За підтримки",
	},
}

// parentMFAResetMail tells an account that chose a language in the parents
// portal, in that language, that its code was reset (#7051). Like the staff
// notice it carries no link.
func parentMFAResetMail(message MFAResetByAdminMail, origin string) Mail {
	lang := parentLocale(message.Locale)
	text := parentCopies[lang].Reset
	return Mail{
		Subject:  text.Subject,
		Template: "parent-mfa-reset.html",
		Content: withFooter(lang, map[string]any{
			"Lang":        lang,
			"LogoURL":     trimmedLogoURL(origin),
			"Title":       text.Title,
			"Greeting":    text.Greeting,
			"Body":        fmt.Sprintf(text.Body, message.ResetAt),
			"Again":       text.Again,
			"NotYouTitle": text.NotYouTitle,
			"NotYou":      text.NotYou,
		}),
	}
}

// withFooter adds the footer of the language to a mail's content.
func withFooter(lang string, content map[string]any) map[string]any {
	text := parentCopies[lang]
	content["FooterText"], content["PoweredByLabel"] = text.FooterText, text.PoweredByLabel
	return content
}

// parentLocale picks the copy's language: the parent's, or German.
func parentLocale(locale string) string {
	locale = strings.ToLower(strings.TrimSpace(locale))
	if _, ok := parentCopies[locale]; ok {
		return locale
	}
	return "de"
}

// parentMFACodeMail is the parents portal's sign-in code in the parent's
// language. Like every code mail it carries no link.
func parentMFACodeMail(message MFACodeMail, origin string) Mail {
	lang := parentLocale(message.Locale)
	text := parentCopies[lang].Code
	return Mail{
		Subject:  text.Subject,
		Template: "parent-mfa-email-code.html",
		Content: withFooter(lang, map[string]any{
			"Lang":        lang,
			"LogoURL":     trimmedLogoURL(origin),
			"Code":        message.Code,
			"RequestIP":   message.RequestIP,
			"Title":       text.Title,
			"Greeting":    text.Greeting,
			"Intro":       text.Intro,
			"CodeLabel":   text.CodeLabel,
			"Validity":    fmt.Sprintf(text.Validity, message.ExpiryMinutes, message.TrustedDeviceDays),
			"RequestFrom": text.RequestFrom,
			"Tip":         tipFor(message, text.Tip),
			"Security":    text.Security,
		}),
	}
}

func tipFor(message MFACodeMail, tip string) string {
	if !message.TrustedDeviceEnabled || message.TrustedDeviceDays <= 0 {
		return ""
	}
	return fmt.Sprintf(tip, 0, message.TrustedDeviceDays)
}

// parentTrustedDeviceMail tells a parent, in the portal's language, that the
// parents portal remembered a device.
func parentTrustedDeviceMail(message TrustedDeviceMail, origin string) Mail {
	lang := parentLocale(message.Locale)
	text := parentCopies[lang].Device
	return Mail{
		Subject:  text.Subject,
		Template: "parent-trusted-device-added.html",
		Content: withFooter(lang, map[string]any{
			"Lang":        lang,
			"LogoURL":     trimmedLogoURL(origin),
			"DeviceLabel": message.DeviceLabel,
			"IPAddress":   message.RequestIP,
			"AddedAt":     message.AddedAt,
			"Title":       text.Title,
			"Greeting":    text.Greeting,
			"Intro":       fmt.Sprintf(text.Intro, message.TrustedDays),
			"DeviceName":  text.DeviceLabel,
			"IPName":      text.IPLabel,
			"TimeName":    text.TimeLabel,
			"NotYouTitle": text.NotYouTitle,
			"NotYou":      text.NotYou,
			"Recovery":    text.Recovery,
		}),
	}
}
