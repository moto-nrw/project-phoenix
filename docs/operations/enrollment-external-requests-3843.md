# Externe Abrufe: Online-Anmeldung und Bildkompression

Stand: 23.10.2026, Slice 3843-a, Teil von #3843 / #3846.

## Im Code umgesetzt

- Die öffentliche Captcha-Konfiguration enthält `enabled` und `site_key`.
  Das Backend liefert bei ausgeschaltetem `enrollment.require_captcha` bereits
  einen leeren Site-Key (`backend/modules/enrollment/internal/application/captcha.go`).
  Das Frontend prüft zusätzlich `enabled`, auch bei einem widersprüchlichen
  Ergebnis mit gesetztem Key: kein Widget, kein Turnstile-Skript, keine
  Captcha-Absende-Sperre und kein Captcha-Token im Antrag.
- Bei aktivem Captcha bleibt ein gültiges Token Pflicht. Ablauf und Fehler
  sperren das Absenden erneut. Ein fehlender Site-Key sperrt das Formular
  weiterhin mit Fehlermeldung. Authentifizierte Elternpfade mit `skipCaptcha`
  laden kein Widget; die serverseitige Prüfung bleibt unverändert.
- Die CSP erlaubt `https://challenges.cloudflare.com` ausschließlich für
  `script-src` und `frame-src` auf öffentlichen Mandanten-Phasenformularen
  (`/{tenant}/anmeldung/{phaseId}`, einschließlich Subdomain-Umschreibung).
  Vorschau, Statusseiten, Elternportal und interne Portale behalten die bisherige
  Policy. Der Proxy kennt die Schuleinstellung nicht: Die CSP erlaubt den
  Dienst auf dieser Seite, das Formular entscheidet anhand von `enabled`,
  ob es ihn lädt. Keine Erweiterung von `connect-src` oder `worker-src`.
  Die Phasenauswahl öffnet das Formular als neues Dokument: Eine interne
  Next.js-Navigation würde die strengere CSP der Übersichtsseite behalten.
- `compressAvatar` verwendet `useWebWorker: false`. Damit startet die
  Bibliothek keinen Blob-Worker, der ihre Standard-`libURL` von jsdelivr lädt.
  JPEG-Ausgabe, 512-Pixel-Grenze, Größenlimit und bisheriger Fehler-Fallback
  bleiben erhalten. Das gilt für Profilbilder und die Fotoauswahl für Kinder.

Die CSP-Ausnahme folgt den [Turnstile-CSP-Vorgaben](https://developers.cloudflare.com/turnstile/reference/content-security-policy/).
Der [öffentliche Test-Site-Key](https://developers.cloudflare.com/turnstile/troubleshooting/testing/)
ist kein Produktionsschlüssel. Es wurden keine echten Zugangsdaten verwendet.

## Lokal beobachtet, nicht produktiv nachgewiesen

Prüfung mit Next.js 16.3.8 und agent-browser/Chromium, nur gegen
`http://test-school.localhost:13843/anmeldung/5` und einen lokalen Prüfaufbau.
Ein API-Stub auf `localhost:18443` lieferte ausschließlich synthetische
Schul-/Formulardaten. PostHog und Sentry waren lokal nicht konfiguriert.
Es wurden keine Personendaten eingegeben. HAR-Aufzeichnungen der Anmeldung
enthielten keine Antwortkörper; veröffentlicht werden nur die Ergebnisse
und Screenshots, keine Rohaufzeichnungen oder Sitzungsdaten.

| Fall | Beobachtung |
|---|---|
| Vorher, Captcha an | Browser blockiert Turnstile durch `script-src-elem`; kein Token, Absenden gesperrt. |
| Nachher, Captcha aus, Key absichtlich trotzdem gesetzt | 53 Same-Origin-Abrufe, keine fremde Origin; kein Turnstile-Skript/Widget, keine Captcha-Sperre. |
| Übersicht → Formular, Captcha an | Vorher bleibt dieselbe Dokument-CSP wirksam und blockiert Turnstile. Nachher lädt ein neues Dokument; Test-Token entsteht. |
| Nachher, Captcha an | Same-Origin-Abrufe und ausschließlich `https://challenges.cloudflare.com` als fremde Origin: Turnstile-JavaScript und Challenge-iframe, jeweils erfolgreich geladen. Test-Token vorhanden, Captcha-Sperre aufgehoben. |
| Fotoauswahl, Captcha aus und an | Reale `StudentPhotoSection` mit realem `compressAvatar` in einer temporären lokalen Prüfseite: nur lokale Ressourcen/Blob-Vorschau, kein jsdelivr-Abruf. |

Turnstile lädt JavaScript und iframe-Inhalte. Dabei fallen mindestens
Verbindungsdaten (einschließlich IP-Adresse), Browser-/HTTP-Metadaten, Origin
und öffentlicher Site-Key an. Das Widget erzeugt ein Token; der Antrag sendet
es an die eigene API. Die unveränderte Backend-Verifikation übermittelt Token
und Quell-IP an Cloudflare. Die lokale Prüfung bestätigt das Laden des Widgets,
nicht die vollständigen internen Verarbeitungen oder Orte bei Cloudflare und
nicht die serverseitige Siteverify-Anfrage. Keine Formulareingaben wurden an
Cloudflare gesendet oder als Testdaten benötigt.

### Entscheidung gegen den Kompressions-Worker

Eine lokale `libURL` würde eine zusätzliche, versionsgebundene Auslieferung
und eine Worker-CSP benötigen. Für die auf 512 Pixel begrenzten Avatare ist
Kompression ohne Worker der kleinere Eingriff. Gemessen mit einem synthetischen
JPEG (4096 × 3072 Pixel, 10.420.970 Bytes): 101.179 Bytes als JPEG, etwa
97–183 ms Kompressionsdauer; längster beobachteter Hauptthread-Task 71 ms.
Das ist keine Garantie für langsame Mobilgeräte: Dekodierung und Canvas-Arbeit
können die Oberfläche kurz blockieren. Bei nachgewiesenen Bedienproblemen ist
ein lokal ausgelieferter Worker eine mögliche Folgeänderung, kein Bestandteil
dieses Slices.

## Wiederholung und menschlicher Nachweis

1. Lokal über `scripts/dev-native.sh up frontend` starten. In einem
   synthetischen Schulkontext Captcha aus/an prüfen, zusätzlich aus mit noch
   gesetztem Site-Key. Nur öffentliche Cloudflare-Testkonfiguration verwenden.
   Der hier verwendete API-Stub ersetzt weder Berechtigungs- noch Backendtests.
2. In frischen Browserkontexten die Netzwerkaufzeichnung vor dem Seitenaufruf
   starten. Auf das geladene Formular warten. Aus: kein Cloudflare-Skript oder
   Widget und keine Captcha-Sperre. An: Skript und iframe laden, Token entsteht;
   ohne Token, nach Ablauf oder Fehler bleibt Absenden gesperrt. Nicht nur
   Fehlerfreiheit prüfen, sondern die Anfrage-Origins erfassen.
3. Ein synthetisches Bild über die echte Fotoauswahl komprimieren. Netzwerk
   einschließlich Worker beobachten: keine Anfrage an jsdelivr. Der ergänzende
   lokale Uploadtest benutzt den echten `uploadAvatar`-Client und eine
   abgefangene Same-Origin-Antwort für `POST /api/me/profile/avatar`.
   Er belegt das browserseitige Multipart-Ziel, nicht Speicherung oder Auth.
4. Nach Auslieferung muss ein berechtigter Mensch die tatsächlich ausgelieferte
   Version, Schuleinstellung, CSP und Abrufe bestätigen. Nur eine bereinigte
   Zusammenfassung dokumentieren: Version, Datum, Funktion an/aus, Origins,
   Inhaltsarten, Erfolg/Fehler. Keine Formulardaten, Cookies, Tokens oder
   vollständigen HAR-Dateien in Tickets/PRs veröffentlichen. Diese Agentenarbeit
   greift nicht auf Produktion oder Staging zu.

Der lokale Infrastrukturstart scheiterte am ausgeschöpften Docker-Netzwerkpool.
Deshalb lief nur das native Frontend über `scripts/dev-native.sh frontend pnpm dev`
mit dem lokalen API-Stub. Die temporäre Prüfseite wurde wieder entfernt.

## Offen und AVV/TOM-Abgleich

Der wiederholbare, vollständig abgefangene Browserlauf für alle Portale
steht im [Localhost-Runbook](nutzungsanalyse.md#automatisierter-localhost-nachweis-3843-c).
Er nutzt synthetische API-Antworten und untersucht keine Payload-Inhalte.

AK5 ist im Code erfüllt; die lokale Prüfung deckt Teile von AK1/AK7 ab.
Nicht abgedeckt sind Anmeldung mit echten Konten, alle Portale,
PostHog-/Sentry-Konfiguration und deren transportierte Inhalte (Slice 3843-c),
AK2/AK3/AK4/AK6 sowie der produktive Nachweis.

Textvorschlag, keine Änderung am Legal-Repo:

> Turnstile wird nur bei von der Schule aktiviertem Captcha in der öffentlichen
> Online-Anmeldung geladen. Diese Bedingung ist im Code umgesetzt; die
> tatsächlich ausgelieferte Konfiguration und die Verarbeitung bei Cloudflare
> sind noch zu bestätigen. Die Bildkompression lädt keinen Code von jsdelivr
> mehr. Lokal wurde kein entsprechender Abruf beobachtet; jsdelivr entfällt
> für diesen Pfad erst nach bestätigter produktiver Auslieferung als Empfänger.

Anhang 2 behält Cloudflare für eingeschaltetes Captcha. Empfänger, Datenarten,
Verarbeitungsorte, Verträge und Drittlandgrundlagen bleiben anhand der
Anbieterunterlagen zu bestätigen. Dieser Slice bestätigt keine pauschale
Abschaltung von Analyse, Fehlerdiagnose oder anderen externen Diensten.
