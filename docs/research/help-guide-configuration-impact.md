# Konfigurationsabhängige Oberfläche in moto — Grundlage für die Überarbeitung der Hilfe

Stand: 30.08.2026
Repositories: `project-phoenix` (Backend + Frontend), `PyrePortal` (Kiosk)
Zweck: Ermitteln, bei welchen schul- oder mandantenbezogenen Einstellungen die Hilfe unterschiedliche
Texte, Schritte, Bilder oder eigene Varianten braucht.
Verwandte Dokumente: `docs/adr/0009-hilfe-bleibt-oeffentlich-die-app-verweist.md`,
`docs/hilfebereich-umbau-plan.md` (Abschnitt 6 und D5).

Alle Aussagen sind gegen Quellcode und Tests geprüft. Vermutungen sind ausdrücklich als solche
gekennzeichnet. Es wurde kein Anwendungscode und keine Hilfe geändert.

---

## 1. Kurzfazit

1. Registriert sind heute **162 Einstellungen** (163 Key-Konstanten in `backend/models/config/keys.go`, davon eine ohne Registrierung), nicht die in ADR 0009 genannten „rund 86" — die Zahl ist seit dem ADR gewachsen und sollte dort nachgezogen werden.
2. Von diesen 162 verändern nur rund **25 sichtbar Oberfläche oder Ablauf**; der Rest sind Aufbewahrungsfristen, Scheduler-Parameter, Formularinhalte der Anmeldung und Abrechnungsschlüssel ohne Anleitungsbezug.
3. Der wichtigste Verteiler ist `GET /auth/tenant/resolve`: er liefert genau **15 aufgelöste Einstellungen** als Mandanten-Metadaten an jedes Mitarbeitenden-Gerät, damit auch Personen ohne `config:read` Navigation und Seiten korrekt sehen (`backend/api/auth/tenant_handlers.go:141-157`).
4. `attendance.nfc_enabled` (Standard **aus**, nur moto-Team) entscheidet, ob es das Tablet-Kapitel für eine Schule überhaupt gibt: ohne NFC verschwinden Navigationseinträge, zwei Bereiche der Datenverwaltung und der komplette Reiter `Geräte` mit neun Einstellungen.
5. `operations.presence_mode` (Standard **detailliert**, nur moto-Team) ist der einzige Schalter, der Inhalte nicht nur weglässt, sondern **falsch macht**: im Binär-Modus blendet die Web-App Räume und Aktivitäten aus, führt Betreuungskräfte nach der Anmeldung auf eine andere Startseite und ersetzt Raumnamen durch „Anwesend/Abwesend/Schulhof".
6. **Wichtige Korrektur zum Cross-Repo-Vertrag:** PyrePortal wertet `presence_mode` heute **nicht aus** — es gibt keinen einzigen `binary`-Zweig in der Kiosk-App (`PyrePortal/src/services/api.ts:787` ist die einzige Fundstelle außerhalb von Tests), der in `CLAUDE.md` beschriebene Kiosk-Zweig ist geplant, aber nicht umgesetzt, und eine Anleitung darf ihn nicht als bestehend beschreiben.
7. Die Ziel-Buttons am Tablet („Wohin geht …?") stehen im Standard **anders als in der heutigen Anleitung**: `checkout.schulhof_enabled` und `checkout.wc_enabled` sind ab Werk **aus**, der Schulhof-Button braucht zusätzlich einen vorhandenen Raum „Schulhof" — die Anleitung beschreibt heute alle vier Ziele als vorhanden.
8. Für die Top 3 ist `operations.group_mode` maßgeblich: Die Einstellung ändert die Startseite nach der Anmeldung und blendet `Meine Gruppen` und `Gruppenzugriff` aus. Die ähnlich benannte Einstellung `operations.care_concept` ist davon unabhängig und wirkt nur nachrangig auf den Schulhof-Reiter und spontane Aktivitäten in der Aktuellen Aufsicht.
9. Neben den Einstellungen wirken **Rollen und Berechtigungen** wie Konfiguration: mehrere Navigationseinträge hängen an `config:read`, `admin:*` oder Spezialrechten, und das Eltern-Portal kombiniert Schul-Einstellung **und** individuelle Sorgeberechtigten-Rechte zu einem Merkmalssatz je Kind.
10. Für die Anleitung heißt das: eine kleine, kuratierte Variantenachse (NFC, Anwesenheits-Modus, Organisationsmodell) statt einer Filterung über alle Einstellungen — genau die Linie aus ADR 0009, aber mit anderen Grenzen als dort angenommen.

---

## 2. Top 3 der einflussreichsten Einstellungen

Aufgenommen wurde nur, was mindestens einen nachgewiesenen sichtbaren Konsumenten hat.
Rollen und Berechtigungen stehen bewusst nicht in dieser Liste; sie folgen in Abschnitt 4.4.

### Rangliste

| Rang | Konfigurationsfaktor | Technischer Schlüssel | Standard | Wer ändert |
|---|---|---|---|---|
| 1 | Anwesenheit über NFC-Geräte erfassen | `attendance.nfc_enabled` | `false` | nur moto-Team (`operator_only`) |
| 2 | Anwesenheits-Modus | `operations.presence_mode` | `detailed` | nur moto-Team (`operator_only`) |
| 3 | Arbeit mit festen Gruppen | `operations.group_mode` | `fixed_groups` | Schul-Admin (`config:update`) |

---

### Rang 1 — `attendance.nfc_enabled` („Anwesenheit über NFC-Geräte erfassen")

| Feld | Inhalt |
|---|---|
| **Mögliche Werte / Standard** | `true` / `false`; Standard **`false`** (`backend/services/config/defaults/operations.go:375-388`) |
| **Wer ändert** | Nur das moto-Team. `AccessPolicy: AccessOperatorOnly`, Schreibrecht `config:manage`; für Schul-Admins im Schema unsichtbar (`backend/services/config/defaults/operations.go:381-384`, Filterlogik `backend/services/config/schema_builder.go:39-57`) |
| **Betroffene Nutzer und Portale** | Alle Mitarbeitenden im Tenant-Portal; die gesamte Kiosk-App PyrePortal |
| **Sichtbarer Unterschied** | Aus: Navigationseinträge `Aktivitäten` und in der Datenverwaltung `Aktivitäten` + `Geräte` verschwinden; wer die Adressen direkt aufruft, bekommt eine 404-Seite; der Einstellungsreiter `Geräte` ist leer, weil alle neun Geräte-Einstellungen von diesem Schalter abhängen. An: NFC-Navigation, Geräteverwaltung, Armbandzuweisung und der komplette Tablet-Ablauf existieren. |
| **Route-Schutz** | `frontend/src/components/tenant/nfc-mode-guard.tsx:13-19` (`notFound()`), angewandt auf `/activities` (`frontend/src/app/[tenant]/(protected)/activities/page.tsx:54-58`), `/database/activities` (`.../database/activities/page.tsx:35-39`) und `/database/devices` (`.../database/devices/page.tsx:58-62`) |
| **Belegkette** | Registry → `backend/api/auth/tenant_handlers.go:145` (im Batch) → `:231` (`resolved.nfcEnabled`) → JSON `nfc_enabled` (`backend/api/auth/types.go:42-45`, gesetzt `backend/api/auth/tenant_handlers.go:109`) → `frontend/src/lib/tenant-api.ts:202` und `frontend/src/app/[tenant]/layout.tsx:72` → `useNFCEnabled()` (`frontend/src/lib/tenant-context.tsx:213-216`) → **Navigation** `frontend/src/components/dashboard/sidebar.tsx:326-330` (`NFC_ONLY_HREFS`), angewandt `:519` und `:569`; **mobile Navigation** `frontend/src/components/dashboard/mobile-bottom-nav.tsx:612-614`; **Datenverwaltung** `frontend/src/app/[tenant]/(protected)/database/page.tsx:184` und `:222`; **Startseite** `frontend/src/app/[tenant]/(protected)/dashboard/page.tsx:166-169` |
| **Abhängige Einstellungen (`depends_on`)** | Neun: `checkout.raumwechsel_enabled`, `checkout.schulhof_enabled`, `checkout.wc_enabled`, `checkout.daily_checkout_from_all_rooms_enabled`, `checkin.activity_capacity_details_enabled`, `checkin.room_capacity_details_enabled` (`backend/services/config/defaults/devices.go:19,33,47,61,81,95`), `security.ogs_device_pin` (`backend/services/config/defaults/security.go:22`), `operations.student_daily_checkout_time` und `operations.per_student_checkout_enabled` (`backend/services/config/defaults/operations.go:68,82`) |
| **Betroffene Hilfe-Themen** | `nfcQuickstartChapters` vollständig; `nfcChapters` vollständig (11 Kapitel); `guideEntryPoints` Karten 3 und 4; in `appChapters` die Schritte `aktivitaeten` und `datenverwaltung`; in `setupChapters` der `go-live-check` |
| **Empfohlene Behandlung** | Eigene Variante auf Anleitungsebene. Die beiden NFC-Einstiege auf `/help` als „nur für Einrichtungen mit NFC-Tablets" kennzeichnen (ADR 0009: kennzeichnen, nicht verstecken). In der Alltags-Anleitung dort, wo NFC-Navigation beschrieben wird, einen bedingten Hinweis setzen. |
| **Bewertung** | Sichtbarkeitsreichweite **sehr hoch** (zwei Navigationsebenen, zwei Datenverwaltungs-Bereiche, ein kompletter Einstellungsreiter, eine ganze zweite App). Ablaufabweichung **vollständig** (mit NFC scannen Kinder selbst; ohne NFC erfasst das Team in der Web-App). Zielgruppenbreite **alle**. Fehlerrisiko **sehr hoch**: eine Schule ohne NFC bekommt heute zwei Einstiegskarten für Geräte, die sie nicht besitzt. Kombinationswirkung **hoch**: neun Folge-Einstellungen. |

---

### Rang 2 — `operations.presence_mode` („Anwesenheits-Modus")

| Feld | Inhalt |
|---|---|
| **Mögliche Werte / Standard** | `detailed` („Detailliert (Räume & Aktivitäten)") oder `binary` („Binär (nur An-/Abwesend)"); Standard **`detailed`** (`backend/services/config/defaults/operations.go:303-321`) |
| **Wer ändert** | Nur das moto-Team (`AccessOperatorOnly`, `config:manage`). Zusätzliche Sperre: ein Wechsel wird abgelehnt, solange am selben Tag noch ein Kind eingecheckt ist — `ErrPresenceModeSwitchBlocked`, Umgehung nur mit `force` und Warn-Log (`backend/services/config/operator_settings_service.go:88-100, 165-190`) |
| **Betroffene Nutzer und Portale** | Alle Mitarbeitenden im Tenant-Portal; indirekt Eltern (Statusanzeige); Kiosk **heute nicht** (siehe unten) |
| **Sichtbarer Unterschied im Tenant-Portal** | Binär: Navigationseinträge `Räume` und `Aktivitäten` verschwinden, ihre Seiten liefern 404; Betreuungskräfte landen nach der Anmeldung auf `/students/search` statt `/ogs-groups`; auf `Meine Gruppen` erscheint ein zusätzlicher Schnell-Anwesenheitsmodus (Pille mobil, FAB auf dem Tablet, Inline-Button auf dem Desktop); Ortsangaben lauten nur noch „Anwesend", „Abwesend" oder „Schulhof" statt Raumnamen und „Unterwegs". |
| **Belegkette Backend** | Registry → `backend/api/auth/tenant_handlers.go:143, 247-250` → JSON `presence_mode` → Auflösung/Fallback `backend/services/config/presence.go:15-33` (leer oder Fehler → `detailed`) → Ortsauflösung `backend/services/active/location_snapshot.go:56-73` und `:142-165` (Binär-Labels), Lade-Kurzschluss `backend/api/common/student_locations.go:44-72`, Einzelfall `backend/api/students/response_helpers.go:307-325`; Umzüge/Transit sind im Binärmodus No-Ops (`backend/services/active/transit_service.go:81, 218, 364`) |
| **Belegkette Frontend** | `frontend/src/lib/tenant-api.ts:200` → `usePresenceMode()` (`frontend/src/lib/tenant-context.tsx:201-204`) → **Navigation** `frontend/src/components/dashboard/sidebar.tsx:336` (`BINARY_HIDDEN_HREFS = ["/rooms","/activities"]`), angewandt `:570` und `:1146`; **Route-Schutz** `frontend/src/components/tenant/binary-mode-guard.tsx:28-33` (`notFound()`), angewandt auf `/rooms` (`.../rooms/page.tsx:729-733`), `/activities` (`.../activities/page.tsx:55-57`) und `/active-supervisions` (`.../active-supervisions/page.tsx:532-551`); **Startseite nach Anmeldung** `frontend/src/lib/redirect-utils.ts:36-53`; **Gruppenseite** `frontend/src/app/[tenant]/(protected)/ogs-groups/page.tsx:182-183, 1102-1112, 1162-1172, 1184, 1192`; **Startseiten-Karten** `frontend/src/app/[tenant]/(protected)/dashboard/page.tsx:169-170` |
| **Kiosk (wichtig)** | PyrePortal liest `presence_mode` **nicht aus**. Einzige Fundstelle außerhalb von Tests ist die Typdeklaration `PyrePortal/src/services/api.ts:787`. Es gibt keine Verzweigung, keine versteckte Raumauswahl, keinen Zwei-Button-Ablauf. Die Scan-Seite ist per Route-Guard sogar an eine bestehende Sitzung mit Aktivität **und** Raum gebunden (`PyrePortal/src/App.tsx:57, 143-149`). Serverseitig wird der Scan dagegen kurzgeschlossen (`backend/api/iot/checkin/handlers.go:305-313, 466-545`) und liefert `room_name: ""`, kein `visit_id` und `daily_checkout_available: false`. Der offene UX-Zweig ist im Backend kommentiert (`backend/api/iot/checkin/handlers.go:466-472`). |
| **Betroffene Hilfe-Themen** | `appChapters`: `home`, `meine-gruppen`, `aktuelle-aufsicht`, `raeume`, `aktivitaeten`, `kindersuche`, `kinderdetailansicht`, `tagesauswertung`; `setupChapters`: `raeume-anlegen`, `aktivitaeten-anlegen`, `go-live-check`; `nfcChapters`: `nfc-aufsicht-starten`, `nfc-ein-auschecken`, `nfc-einstellungen-betrieb` |
| **Empfohlene Behandlung** | Eigene Variante für die Web-App-Themen (andere Schritte, andere Bilder, anderer Einstiegspfad nach der Anmeldung). Für das Tablet **vorerst keine** Binär-Variante schreiben, sondern die Umsetzungslage klären (Abschnitt 6, Frage 1). |
| **Bewertung** | Sichtbarkeitsreichweite **sehr hoch**. Ablaufabweichung **vollständig**, inklusive der ersten Seite nach der Anmeldung. Zielgruppenbreite **alle Betreuungskräfte und die Leitung**. Fehlerrisiko **sehr hoch**: `guide-data.ts` nennt „Schulhof" 23-mal, „Toilette" 7-mal, „Raumwechsel" 5-mal und den Anwesenheits-Modus selbst nur dreimal (gemessen am 30.08.2026 über die ganze Datei; ADR 0009 nennt für den 29.08.2026 niedrigere Zahlen, weil dort nur die NFC-Kapitel gezählt wurden). Kombinationswirkung **hoch** (mit NFC und mit dem Schulhof-Schalter). |

---

### Rang 3 — `operations.group_mode` („Arbeit mit festen Gruppen")

| Feld | Inhalt |
|---|---|
| **Mögliche Werte / Standard** | `fixed_groups` („Feste Gruppen") oder `open_care` („Offene Betreuung ohne feste Gruppen"); Standard **`fixed_groups`** (`backend/services/config/defaults/operations.go:391-409`) |
| **Wer ändert** | Schul-Admin, `config:update`, Reiter `Betrieb`, Kategorie `organisation` |
| **Betroffene Nutzer und Portale** | Alle Betreuungskräfte im Tenant-Portal |
| **Sichtbarer Unterschied** | `open_care`: Betreuungskräfte landen nach der Anmeldung auf `/students/search` statt `/ogs-groups`; das Akkordeon `Meine Gruppen` und der Eintrag `Gruppenzugriff` (`/substitutions`) verschwinden aus der Navigation; ein direkter Aufruf von `/ogs-groups` leitet auf `Alle Kinder` um; auf der Startseite entfällt die Gruppen-Kachel. |
| **Route-Schutz** | `frontend/src/components/tenant/open-care-mode-guard.tsx:19-26` (`redirect` auf `/students/search`), angewandt in `frontend/src/app/[tenant]/(protected)/ogs-groups/page.tsx:1241-1243` |
| **Belegkette `group_mode`** | `backend/api/auth/tenant_handlers.go:152, 251-253, 290-300` → JSON `group_mode` (`backend/api/auth/types.go:83`) → `useOpenCareGroupMode()` (`frontend/src/lib/tenant-context.tsx:287-290`) → **Startseite nach Anmeldung** `frontend/src/lib/redirect-utils.ts:53` und `frontend/src/components/auth/smart-redirect.tsx:22-35` → **Startseiten-Karten** `frontend/src/app/[tenant]/(protected)/dashboard/page.tsx:167, 171-175, 514` → **Navigation** `frontend/src/components/dashboard/sidebar.tsx:147, 1027, 1205-1207` und `frontend/src/components/dashboard/mobile-bottom-nav.tsx:643, 665` |
| **Betroffene Hilfe-Themen** | `appChapters`: `home`, `meine-gruppen`, `kindersuche`; `setupChapters`: `gruppen-anlegen`; `appChapters`: `einstellungen-zustaendigkeit` |
| **Empfohlene Behandlung** | Bedingter Hinweis plus abweichender erster Schritt („Nach der Anmeldung sehen Sie …"). `operations.care_concept` ist keine Variante dieses Faktors und wird nur als nachrangiger Hinweis in Abschnitt 3.6 behandelt. |
| **Bewertung** | Sichtbarkeitsreichweite **mittel bis hoch** (Startseite nach Anmeldung, Startseiten-Karten und Navigation). Ablaufabweichung **hoch** — der Einstieg in den Arbeitstag ist ein anderer. Zielgruppenbreite **alle Betreuungskräfte**. Fehlerrisiko **hoch**: der erste Satz jeder Alltagsanleitung stimmt sonst nicht. Kombinationswirkung **hoch**: `group_mode` kombiniert mit `presence_mode`, weil beide auf `/students/search` führen. |

---

## 3. Entscheidungs-Matrix für die Anleitung

### 3.1 `attendance.nfc_enabled`

- **Wenn `attendance.nfc_enabled = false`** (Standard), sieht die Person: keinen Navigationseintrag `Aktivitäten`, in der Datenverwaltung keine Kacheln `Aktivitäten` und `Geräte`, und im Reiter `Geräte` der Einstellungen keine Optionen.
  → Die Anleitung braucht: **ein eigenes Thema entfällt.** Beide NFC-Einstiegskarten auf `/help` werden als „nur mit NFC-Tablets" gekennzeichnet; in `datenverwaltung` und `aktivitaeten` steht ein bedingter Hinweis.
- **Wenn `attendance.nfc_enabled = true`**, sieht die Person stattdessen: die NFC-Navigation, die Geräteverwaltung, die Armbandzuweisung und den vollständigen Reiter `Geräte`.
  → Die Anleitung braucht: die heutigen NFC-Kapitel unverändert als Bestand, aber intern nach den Regeln 3.3 verzweigt.

### 3.2 `operations.presence_mode`

- **Wenn `presence_mode = binary`**, sieht die Person in der Web-App: keine Einträge `Räume` und `Aktivitäten` (direkte Aufrufe enden auf 404), nach der Anmeldung als Betreuungskraft `Alle Kinder` statt `Meine Gruppen`, auf `Meine Gruppen` einen zusätzlichen Schnell-Anwesenheitsmodus, und als Ort nur „Anwesend", „Abwesend" oder „Schulhof".
  → Die Anleitung braucht: **eigene Varianten** der Themen `home`, `meine-gruppen`, `kindersuche`, `aktuelle-aufsicht`; die Themen `raeume`, `aktivitaeten`, `raeume-anlegen`, `aktivitaeten-anlegen` gelten dort nicht und werden gekennzeichnet; **andere Bilder** für `meine-gruppen` (Schnell-Anwesenheit) und die Kinderkarten (Statusabzeichen).
- **Wenn `presence_mode = detailed`** (Standard), sieht die Person stattdessen: Räume, Aktivitäten, Raumverlauf und die Ortsangabe „Unterwegs" beim Wechsel.
  → Die Anleitung braucht: den heutigen Text.

### 3.3 Ziel-Buttons am Tablet — ein Bild, benannte Bedingungen

Der Dialog erscheint nur nach einem Auschecken (`action = "checked_out"`) und nur, wenn mindestens
ein Ziel übrig bleibt.

**Empfohlene Umsetzung: ein einziges Bild plus eine konkrete Bildunterschrift, dazu drei ausdrücklich
benannte Bedingungen.** Keine eigenen Varianten dieses Themas.

Als Bild bleibt der heutige `nfc-auschecken.webp` mit allen vier Zielen — also die *reichste*, nicht
die Standard-Variante. Begründung ist die Asymmetrie: Wer vier Ziele im Bild sieht und nur eines hat,
erkennt sein Bild als Teilmenge und braucht nur die Unterschrift. Wer eines im Bild sieht und vier
hat, muss raten, was `Schulhof` und `Toilette` tun. Ein Standard-Bild würde die zweite, schlechtere
Lage erzeugen.

Die Bildunterschrift darf konkret sein, weil die Menge der Ziele geschlossen ist (vier fest
verdrahtete Einträge, `PyrePortal/src/pages/ActivityScanningPage.tsx:460-513`) — also die vier Namen
nennen statt „je nach Einrichtung verschieden" zu schreiben. Sinngemäß: *„Welche dieser Ziele
erscheinen, legt Ihre OGS unter `Einstellungen` → `Geräte` fest. `Schulhof` und `Toilette` sind im
Auslieferungszustand ausgeschaltet."*

#### Was die Bildunterschrift abdeckt

Diese Unterschiede lassen nur Elemente weg; die Schritte bleiben identisch, und niemand wird zu einer
falschen Handlung geführt.

- **Wenn `checkout.raumwechsel_enabled = true`** (Standard), sieht die Person `Raumwechsel`. Der Button wählt **keinen** Raum aus, sondern schließt den Dialog; das Kind scannt im Zielraum erneut (`PyrePortal/src/hooks/pages/useCheckoutDestination.ts:36-40`).
- **Wenn `checkout.schulhof_enabled = true` und ein Raum „Schulhof" existiert**, sieht die Person zusätzlich den orangefarbenen Button `Schulhof`; nach dem Tippen erscheint „Viel Spaß auf dem Schulhof, {Vorname}!" (`PyrePortal/src/pages/ActivityScanningPage.tsx:477-487`, `PyrePortal/src/services/checkoutDestinationService.ts:45`).
- **Wenn `checkout.wc_enabled = true` und ein Raum „WC" oder „Toilette" existiert**, sieht die Person zusätzlich `Toilette`. Die Beschriftung lautet immer „Toilette", auch wenn der Raum „WC" heißt (`PyrePortal/src/services/apiErrors.ts:234, 248-251`). Das gehört in die Unterschrift oder in einen Halbsatz, weil sonst Raumliste und Tablet-Beschriftung auseinanderlaufen.
- **Wenn drei oder vier Ziele sichtbar sind**, wechselt das Modal auf ein 2×2-Raster und wird größer; bei einem oder zwei Zielen steht eine einzeilige Reihe (`PyrePortal/src/pages/ActivityScanningPage.tsx:524-525, 883-892`). Ein Halbsatz in der Unterschrift, kein zweites Bild.

#### Was einen eigenen Satz braucht — die Unterschrift reicht nicht

In diesen drei Fällen würde ein allgemeiner Hinweis („die Ziele unterscheiden sich") die Person in
eine Sackgasse führen, die `.claude/rules/verstaendlichkeit.md` ausdrücklich verbietet.

- **`nach Hause` ist kein Ein/Aus-Schalter.** Es hängt an der Abmeldezeit und am Raum: erst nach `operations.student_daily_checkout_time` (leer = jederzeit) beziehungsweise, bei `operations.per_student_checkout_enabled`, ab der individuellen Abholzeit minus Vorlauf; und dann entweder in jedem Raum oder nur im eigenen Gruppenraum und auf dem Schulhof (`backend/services/iot/checkin/checkout_gate.go:163-190, 207-244`).
  → Die Anleitung braucht: **einen eigenen Satz mit der Bedingung**, nicht den Hinweis auf Konfigurierbarkeit. Wer `nach Hause` nicht sieht, schließt sonst nicht auf eine Einstellung, sondern auf einen Defekt.
- **Sind alle drei Ziele aus, gibt es den Dialog überhaupt nicht.** Die Person sieht direkt „Du bist aus diesem Raum abgemeldet." (`PyrePortal/src/pages/ActivityScanningPage.tsx:517-522, 1112-1115`; Test `PyrePortal/src/pages/ActivityScanningPage.test.tsx:725-753`).
  → Die Anleitung braucht: **einen eigenen Satz**, weil das kein reduziertes Bild ist, sondern ein anderer Bildschirm. Eine Unterschrift am Vier-Ziele-Bild trägt hier nicht.
- **Einstellung an, Raum fehlt** — wurde der Raum „Schulhof" (oder „WC"/„Toilette") gelöscht oder umbenannt, verschwindet der Button stillschweigend; das Tablet meldet nichts, der Fall steht nur im Log (`PyrePortal/src/hooks/pages/useActivityScanningPage.ts:286-297, 300-308`).
  → Die Anleitung braucht: **einen Eintrag in der Fehlerbehebung** („Schulhof-Button fehlt trotz eingeschalteter Einstellung → Raum prüfen"), nicht eine Bildunterschrift.

#### Reichweite dieser Technik

„Ein Bild plus benannte Bedingungen" trägt hier, weil auf einem sonst gleichen Bildschirm nur
Elemente fehlen. Für Rang 1 und Rang 2 trägt sie **nicht**: dort verschwinden ganze Seiten (404
beziehungsweise Weiterleitung), der Einstiegspfad nach der Anmeldung ist ein anderer, und im
Binär-Modus stimmen die Ortsbezeichnungen nicht mehr. Eine Bildunterschrift kann nicht auffangen,
dass jemand auf einer 404-Seite steht.

### 3.4 Kombination NFC × Anwesenheits-Modus

Die vom Auftrag genannten Kombinationen wurden geprüft. Nur diese erzeugen im Code tatsächlich unterschiedliche Oberflächen:

| Kombination | Erzeugt eine eigene Oberfläche? | Beleg |
|---|---|---|
| **NFC aus** | **Ja** — kein Tablet-Kapitel, keine NFC-Navigation, leerer Geräte-Reiter | `sidebar.tsx:326-330,519,569`; `database/page.tsx:184,222`; `defaults/devices.go:19,33,47,61,81,95` |
| **NFC an + detaillierte Anwesenheit** | **Ja** — der heute dokumentierte Normalfall (Aufsicht starten, Raum wählen, Zieldialog) | `PyrePortal/src/App.tsx:57,143-149`; `ActivityScanningPage.tsx:453-525` |
| **NFC an + binäre Anwesenheit + Schulhof** | **Nein, nicht am Tablet.** Der Kiosk verzweigt nicht nach `presence_mode`; er zeigt weiterhin Aktivitäts- und Raumauswahl und im Zieldialog `Raumwechsel`. In der Web-App unterscheidet sich der Fall dagegen sehr wohl (3.2). | `PyrePortal/src/services/api.ts:787` als einzige Fundstelle; `backend/api/iot/checkin/handlers.go:466-472` (offener UX-Zweig) |
| **NFC an + binäre Anwesenheit ohne Schulhof** | **Nein, nicht am Tablet** — identisch zur Zeile darüber | wie oben |
| **NFC aus + binäre Anwesenheit** | **Ja** — die schlankste Variante: Web-App ohne Räume, Aktivitäten und Tablet; Betreuungskräfte starten auf `Alle Kinder` und arbeiten über den Schnell-Anwesenheitsmodus | `sidebar.tsx:336,569-570`; `redirect-utils.ts:53`; `ogs-groups/page.tsx:1102-1112` |

Zusätzlich real, aber im Auftrag nicht genannt:

- **`presence_mode = binary` und ein Kind auf dem Schulhof**: Der Zustand `on_yard` ist im Datenmodell vorhanden und wird gelesen, aber **nirgends geschrieben**. Der Kommentar am Modell sagt ausdrücklich, dass der Schreibpfad aus PyrePortal noch fehlt und `on_yard` in der Produktion unerreichbar ist (`backend/models/active/attendance.go:26-33`). Das Label „Schulhof" in der Binär-Variante ist damit heute nicht erreichbar.

### 3.5 `group_mode` × `presence_mode` (Startseite nach der Anmeldung)

- **Wenn `presence_mode = binary` ODER `group_mode = open_care`**, landet eine Betreuungskraft nach der Anmeldung auf `Alle Kinder` (`/students/search`).
  → Die Anleitung braucht: einen abweichenden ersten Schritt in jedem Alltags-Thema, das mit „Öffnen Sie `Meine Gruppen`" beginnt.
- **Sonst** landet sie auf `Meine Gruppen`, bei laufender eigener Aufsicht ohne Gruppen auf `Aktuelle Aufsicht`; eine reine Leitung landet auf `Home`.
  → Beleg: `frontend/src/lib/redirect-utils.ts:36-80`.

### 3.6 Nachrangig: `care_concept` × `attendance.web_spontaneous_activities_enabled`

- **Wenn `care_concept = open_rooms` UND `web_spontaneous_activities_enabled = true`** (beides Standard), sieht die Person in `Aktuelle Aufsicht` den Banner „Spontane Aktivität starten" und den Schulhof-Reiter.
- **Wenn `care_concept = fixed_schedule`**, sieht die Person weder Banner noch Schulhof-Reiter — der zweite Schalter wird gar nicht mehr ausgewertet und ist in den Einstellungen ausgeblendet (`backend/services/config/defaults/operations.go:453-467`, `backend/services/supervisiondashboard/service.go:539-544`).
  → Die Anleitung braucht: einen bedingten Abschnitt im Thema `aktuelle-aufsicht`.

---

## 4. Vollständige Bestandsaufnahme

### 4.0 Vorbemerkung: wie eine Einstellung überhaupt in die Oberfläche kommt

Es gibt vier Wege. Wer die Hilfe schreibt, muss sie auseinanderhalten, weil sie unterschiedlich
zuverlässig sind:

1. **Mandanten-Metadaten aus `/auth/tenant/resolve`** — 15 aufgelöste Einstellungen, für **jede** angemeldete Person lesbar, auch ohne `config:read`. Server-Abruf `frontend/src/app/[tenant]/layout.tsx:43-87`, Client-Abruf `frontend/src/lib/tenant-api.ts:176-221`, Auswahl-Hooks `frontend/src/lib/tenant-context.tsx:201-310`. Das ist der einzige Kanal, auf den sich Navigation und Route-Schutz stützen dürfen.
2. **Einstellungs-Schema (`/api/settings/schema`)** — nur für Personen mit `config:read`. Das Frontend liest daraus namentlich genau **sechs** Schlüssel: `timetable.enabled`, `operations.parent_news_enabled`, `operations.meal_plan_enabled`, `guardians.parent_invite_mode`, `timetable.day_start_time`/`day_end_time` und `enrollment.enabled` (`frontend/src/lib/settings-api.ts:65-81`, `frontend/src/lib/hooks/use-settings-schema.ts:13-22`). **Folge für die Anleitung:** Einträge, die an diesem Kanal hängen — `Essensplan` und `Elternmitteilungen` — sind für Betreuungskräfte ohne `config:read` unsichtbar, auch wenn die Schule sie eingeschaltet hat.
3. **Feld in einer normalen API-Antwort** — z. B. `attendance_log_enabled` und `feedback_enabled` am Kind (`frontend/src/lib/hooks/use-student-data.ts:244-264`), `capabilities.web_spontaneous_activities_enabled` am Aufsichts-Dashboard, `staff_upload_enabled` an der Dateiablage, `ChildFeatures` je Kind im Eltern-Portal.
4. **Serverseitig, ohne jede Frontend-Sichtbarkeit** — alle übrigen rund 140 Einstellungen. Sie erscheinen nur im automatisch erzeugten Einstellungsformular.

Zwei gegenläufige Vorsichtsmuster sind bewusst gesetzt: Navigation prüft `!== false` (der Eintrag
bleibt sichtbar, solange das Schema lädt), Route-Schutz prüft `=== false` (die Seite bleibt sichtbar,
solange das Schema lädt) — `frontend/src/components/dashboard/sidebar.tsx:477-482` gegen
`frontend/src/components/timetable/betreuungsplan-view.tsx:536-537`.

### 4.1 Registrierte Mandanten-Einstellungen

162 Definitionen aus 154 `config.Register`-Aufrufen (zwei davon in Schleifen: sieben Lohnarten, drei Einheiten) in `backend/services/config/defaults/*.go`. Gruppiert nach Wirkung auf die Anleitung.

#### A — Verändern Oberfläche oder Ablauf sichtbar (25)

| Schlüssel | Standard | Sichtbarer Effekt | Beleg |
|---|---|---|---|
| `attendance.nfc_enabled` | `false` | Navigation, Datenverwaltung, Geräte-Reiter, Tablet | `sidebar.tsx:326-330,519,569` |
| `operations.presence_mode` | `detailed` | Navigation, 404-Schutz, Startseite nach Anmeldung, Ortslabels | `sidebar.tsx:336,570`; `binary-mode-guard.tsx:28-33` |
| `attendance.web_enabled` | `true` | Alle An-/Abmelde-Aktionen der Web-App | `api/common/attendance_web.go:17-36` |
| `operations.group_mode` | `fixed_groups` | Startseite nach Anmeldung, Startseiten-Karten | `redirect-utils.ts:53`; `dashboard/page.tsx:171-175` |
| `operations.care_concept` | `open_rooms` | Spontan-Banner und Schulhof-Reiter in Aktueller Aufsicht | `supervisiondashboard/service.go:539-551` |
| `attendance.web_spontaneous_activities_enabled` | `true` | dito, zweite Bedingung | `api/timetable/operations.go:448-469` |
| `operations.operational_overview_scope` | `own` | Welche Räume in Aktueller Aufsicht erscheinen | `auth/authorize/operational_overview.go:71-91` |
| `checkout.raumwechsel_enabled` | `true` | Tablet-Button `Raumwechsel` | `ActivityScanningPage.tsx:467-476` |
| `checkout.schulhof_enabled` | `false` | Tablet-Button `Schulhof` + legt den Raum an | `ActivityScanningPage.tsx:477-487`; `facilities/settings_sideeffects.go:13-20` |
| `checkout.wc_enabled` | `false` | Tablet-Button `Toilette` + legt den Raum an | `ActivityScanningPage.tsx:488-500`; `facilities/settings_sideeffects.go:22-29` |
| `checkout.daily_checkout_from_all_rooms_enabled` | `true` | Wo `nach Hause` erscheint | `iot/checkin/checkout_gate.go:174-190` |
| `operations.student_daily_checkout_time` | `""` | Ab wann `nach Hause` erscheint | `iot/checkin/checkout_gate.go:150-160` |
| `operations.per_student_checkout_enabled` | `false` | Zeitschranke je Kind statt global | `iot/checkin/checkout_gate.go:106-137` |
| `operations.per_student_checkout_delta_minutes` | `15` | Vorlauf vor der Abholzeit | `iot/checkin/checkout_gate.go:140-148` |
| `checkin.room_capacity_details_enabled` | `true` | „Turnhalle ist voll (30/30 …)" statt allgemeinem Text | `api/iot/checkin/workflow.go:61-66`; `PyrePortal/src/services/apiErrors.ts:269-282` |
| `checkin.activity_capacity_details_enabled` | `false` | dito für Aktivitäten | `api/iot/checkin/workflow.go:70-75` |
| `security.ogs_device_pin` | `"1234"` | PIN-Bildschirm am Tablet | `api/iot/api.go:79-94`; `PyrePortal/src/pages/PinPage.tsx:21-27,207` |
| `feedback.enabled` | `false` | Tages-Feedback nach dem Auschecken | `api/iot/checkin/attendance_handlers.go:215`; `PyrePortal/src/hooks/pages/useActivityScanningPage.ts:536-546` |
| `gdpr.attendance_log_enabled` | `false` | Navigationseintrag `Tagesauswertung`; im Kind-Detail wird der Button `Anwesenheitsprotokoll` mit „Für Ihre Schule deaktiviert" abgeschaltet; die Raum-Historie zeigt eine eigene Meldung | `sidebar.tsx:576`; `components/students/student-detail-components.tsx:1056,1075-1088`; `components/rooms/room-detail-content.tsx:167,241,328`; `app/[tenant]/(protected)/day-log/page.tsx:497-498`; `api/rooms/api.go:473` |
| `display.enabled` | `false` | Navigationseintrag `Info-Displays`; direkter Aufruf ergibt 404 | `sidebar.tsx:573`; `components/tenant/display-mode-guard.tsx:13-19` angewandt in `app/[tenant]/(protected)/info-displays/page.tsx:47-49`; `api/display/api.go:116` |
| `operations.staff_messaging_enabled` | `false` | Navigationseintrag `Team-Chat` | `sidebar.tsx:578` |
| `operations.meal_plan_enabled` | `true` | Eintrag `Essensplan` (zusätzlich `config:read` nötig) | `sidebar.tsx:474-476,548-552` |
| `operations.parent_news_enabled` | `true` | Eintrag `Elternmitteilungen`; Eltern-Portal | `sidebar.tsx:471-472,545` |
| `timetable.enabled` | `true` | Akkordeon `Planung` schrumpft auf `Kalenderzeiträume` und `Abrechnung`; Betreuungsplan, Vertretung, Dienstplan und Tageslisten zeigen einen ausdrücklichen Abschalt-Zustand; auf der Exporte-Seite entfallen zwei Exportkarten | `sidebar.tsx:481-482,498-502`; `betreuungsplan-view.tsx:537,1298-1307`; `vertretung-view.tsx:278,601-603`; `dienstplan-view.tsx:148,272-274`; `app/[tenant]/(protected)/lists/page.tsx:721,2051-2059`; `app/[tenant]/(protected)/database/exports/page.tsx:100-101` |
| `operations.student_photos_enabled` | `false` | Kinderfotos auf Kartenlisten, im Kind-Detail und im Stammdaten-Formular; das Ausschalten löscht die Fotos und wird mit einem Warndialog bestätigt | `services/users/student_photo_service.go:327,430`; `frontend/src/lib/hooks/use-student-photos-enabled.ts:7-19`; `components/students/compact-student-card.tsx:49,62`; `components/settings/settings-field.tsx:44-52` |

#### B — Ändern Inhalt oder Text, aber nicht die Struktur (rund 20)

`operations.emergency_list_health_info` (Gesundheitsspalte der Notfallliste, `services/emergency/service.go:158`), `operations.birthday_display_enabled` und `…_include_staff` (Geburtstagskarte auf Home, `services/users/birthday_service.go:129,138`), `tracking.indicators_enabled` + `tracking.indicator_1..3` (Haken auf den Kinderkarten; **das Frontend kennt keinen eigenen Schalter** — bei ausgeschalteter Einstellung liefert das Backend eine leere Indikatorliste und die Karten rendern nichts: `api/active/tracking_handlers.go:40-47`, `services/ogsgrouplive/service.go:739`, Renderbedingung `frontend/src/app/[tenant]/(protected)/ogs-groups/page.tsx:1054-1063`), `gdpr.attendance_visible_days` und `gdpr.room_detail_visible_days` (wie weit der Verlauf zurückreicht, `api/students/attendance_history_handlers.go:140-141`), `timetable.show_expected_children_count`, `timetable.children_per_staff_ratio`, `timetable.day_start_time`/`day_end_time`, `timetable.slot_list_short_day_cutoff`/`long_day_cutoff` (Tageslisten, `services/slotlists/service.go:261-270`), `operations.status_flag_clear_time`, `operations.sick_clear_mode`, `operations.excused_clear_mode`, `operations.require_pickup_offering_review`, `files.staff_upload_enabled` und `files.max_storage_mb` (`services/filestore/service.go:197-200`), `notifications.*` (neun Schalter; sie erlauben Benachrichtigungen, verordnen sie aber nicht), `reminders.*` (sechs Schalter, entscheiden welche Erinnerungen an der Glocke erscheinen), `calendar.appointment_reminder_*`.

#### C — Betreffen nur das Eltern-Portal (14)

`operations.parent_sick_note_enabled`, `parent_sick_requires_approval`, `parent_excused_requires_approval`, `parent_notes_enabled`, `parent_pickup_change_enabled`, `parent_master_data_edit_enabled`, `parent_master_data_request_enabled`, `parent_guardian_management_enabled`, `parent_care_pickup_request_enabled`, `parent_care_mode_request_enabled`, `parent_message_staff_name_visible`, `parent_news_enabled`, `guardians.parent_invite_mode`, `guardians.parent_can_remove`. Alle laufen über einen gemeinsamen Merkmalssatz je Kind (`backend/services/parent/parent_write_service.go:593-716`), der Schul-Einstellung **und** individuelle Sorgeberechtigten-Rechte verrechnet. Für die Hilfe erst relevant, wenn die Eltern-Hilfe entsteht (ADR 0009, Strang C2).

#### D — Anmeldung (Online-Formular), 33 Einstellungen

`enrollment.*` (inklusive der sechs Rechtstext-Blöcke und der Captcha-Schlüssel). Sie verändern das öffentliche Anmeldeformular und die Prüfoberfläche, nicht die Alltags-Navigation. `enrollment.enabled` (Standard `false`) blendet **keinen** Navigationseintrag aus — der Bereich `Anmeldungen` ist rein rollen-gesteuert; die Einstellung ändert nur den Checklisten-Zustand und die Erreichbarkeit des öffentlichen Formulars (`frontend/src/components/enrollment/admin-enrollments-list.tsx:281-296,575-583`; `backend/services/enrollment/request_service.go:3152-3154`). **Das weicht von der Annahme in ADR 0009 ab**, dort ist `enrollment.enabled` als Navigations-Schalter aufgeführt.

#### E — Ohne sichtbare Wirkung auf die Anleitung (rund 70)

Untersucht und bewusst ausgeschlossen:

- **Aufbewahrungsfristen**: `gdpr.time_tracking_retention_days`, `gdpr.student_change_log_retention_days`, `gdpr.pwa_usage_retention_days`, `gdpr.staff_message_retention_days`, `gdpr.privacy_consent_retention_days`, `gdpr.timetable_retention_days`, `feedback.data_retention_days`, `enrollment.rejected_retention_days`. Wirken erst nach Wochen und ohne Bildschirmänderung.
- **Scheduler und Systemtakte** (alle `operator_only` oder Reiter `System`): `operations.session_end_*`, `operations.session_cleanup_*`, `operations.session_inactivity_timeout_minutes`, `operations.student_activation_interval_minutes`, `gdpr.data_cleanup_*`, `enrollment.outbox_*`, `enrollment.status_token_ttl_days`, `invitations.guardian_token_expiry_hours`, `iot.device_online_window_minutes`, `timetable.materialization_*`.
- **Abrechnung**: die zwölf `payroll.*`-Einstellungen. Sie erscheinen gar nicht auf der Einstellungsseite — der Reiter `abrechnung` wird dort herausgefiltert und auf einer eigenen Seite gepflegt (`frontend/src/components/settings/settings-page.tsx:330`).
- **Sicherheit**: `security.mfa_*` und `security.account_lockout_*` verändern den Anmeldeablauf, aber nicht die Alltagsoberfläche. **Hinweis:** die MFA-Kette ist für die Anleitung durchaus relevant, gehört aber zum Thema Anmeldung, nicht zur Konfigurationsvarianz der Alltagsansichten.
- **Standort**: `operations.federal_state` steuert nur den Feiertagskalender der Zeiterfassung.
- **Zeiterfassung**: `operations.time_tracking_account_start_date`, `…_enforce_planned_start`, `…_require_deviation_reason`, `…_deviation_tolerance_minutes`, `tracking.auto_checkout_*` — sie ändern Regeln, nicht Bildschirme; für das Thema `zeiterfassung` genügt ein Satz.
- **Tote Konstante**: `operations.parent_care_arrival_request_enabled` (`backend/models/config/keys.go:146`) ist **nicht registriert** und hat außerhalb eines Tests keinen Konsumenten. Er kann weder gesetzt noch gelesen werden.

### 4.2 Schul- und Organisationskonfiguration außerhalb der Registry

| Faktor | Sichtbarer Effekt | Beleg |
|---|---|---|
| `platform.schools.settings` (JSONB), Schlüssel `loginImageUrl` / `schoolLogoUrl` | Bild auf der Anmeldeseite und Logo im Kopf jeder E-Mail; gepflegt über den handgeschriebenen Reiter `Personalisierung` | `backend/models/platform/school.go:22`; `backend/services/enrollment/email_branding.go:12-33`; `backend/services/auth/guardian_invitation_service.go:237-249`; `frontend/src/components/settings/settings-page.tsx` (Personalisierungs-Reiter) |
| `platform.schools.name` / `slug` / `subdomain` | Adresse des Portals, Anzeigename, Schulname auf dem Tablet-Startbildschirm | `backend/models/platform/school.go:16-18`; `PyrePortal/src/services/schoolName.ts:38-52` |
| `platform.schools.active` / `hidden` / `deleted_at` | Ein inaktiver oder gelöschter Mandant wird bei `/auth/tenant/resolve` als „nicht gefunden" behandelt | `backend/api/auth/tenant_handlers.go:55-59` |
| `enrollment.phases`-Spalten | Pro Anmeldephase: Anmeldefenster, Formularvorlage, Auswahlmodus der Betreuungsangebote, Überlaufregel, Sichtbarkeit von Ablehnungsgründen, zulässige Klassen und Klassenstufen | `backend/models/enrollment/phase.go:127-215` |
| Räume, Gruppen, Aktivitäten, Kategorien | Ohne angelegte Räume ist am Tablet keine Aufsicht startbar; ohne zugewiesene Aktivitäten erscheint „Sie haben derzeit keine zugewiesenen Aktivitäten." | `PyrePortal/src/App.tsx:57,143-149`; `PyrePortal/src/pages/CreateActivityPage.tsx:25-27` |
| Raumkapazität | Erzeugt die 409-Meldung „Raum voll" | `backend/api/iot/checkin/workflow.go:57-66` |
| Gruppenraum eines Kindes | Entscheidet bei ausgeschaltetem „Nach Hause in jedem Raum", wo `nach Hause` erscheint | `backend/services/iot/checkin/checkout_gate.go:207-220` |

### 4.3 Geräte- und NFC-Konfiguration

| Faktor | Sichtbarer Effekt | Beleg |
|---|---|---|
| `GET /api/iot/config` | Liefert genau `presence_mode`, `checkout.{raumwechsel,schulhof,wc}_enabled`, `checkout.daily_checkout_time`, `feedback.enabled` | `backend/api/iot/config.go:17-39`; `PyrePortal/src/services/api.ts:786-798` |
| Abrufzeitpunkt | Einmal beim Betreten der Scan-Seite, kein Polling: Änderungen greifen erst, wenn die Aufsicht die Seite neu betritt | `PyrePortal/src/hooks/pages/useActivityScanningPage.ts:318-337` |
| Fehlerfall des Abrufs | Alle drei Ziel-Buttons erscheinen (Muster `!== false`), ohne Hinweis auf dem Tablet | `PyrePortal/src/hooks/pages/useActivityScanningPage.ts:332-335` |
| Geräte-API-Schlüssel | Bestimmt Mandant und Schule des Tablets; fehlt er, erscheint „API-Schlüssel ungültig …" | `PyrePortal/src/platform/webAdapterBase.ts:108-117`; `PyrePortal/src/services/apiErrors.ts:96-97`; `backend/api/iot/config.go:44-54` |
| `iot.devices.room_id` | **Ohne sichtbare Wirkung auf dem Tablet**: die Raumliste ist ungefiltert | `backend/models/iot/device.go:38`; `backend/api/iot/data/handlers.go:145-181` |
| Zwei PIN-Wege | Entweder die gemeinsame OGS-Geräte-PIN oder eine persönliche Konto-PIN mit `X-Staff-ID`; heute meldet PyrePortal sich mit der gemeinsamen PIN an (`staffName: 'OGS Device'`) | `backend/auth/device/device_auth.go:248-265` (gemeinsame PIN) und `:268-306` (persönliche PIN); `PyrePortal/src/pages/PinPage.tsx:207-216` |
| Kiosk-Ziel (GKT / Wedge / Browser) | Unterschiedliche NFC-Hardware und Fehlertexte; **Raspberry Pi/Balena und Tauri sind stillgelegt** | `PyrePortal/CLAUDE.md:33-41` |

### 4.4 Rollen und Berechtigungen (nicht Teil der Top 3)

| Faktor | Sichtbarer Effekt | Beleg |
|---|---|---|
| Systemrolle `admin` | Blendet Datenverwaltung, Einstellungen, Anmeldungen, Statistik ein | `frontend/src/components/dashboard/sidebar.tsx:440,595-600` |
| `hideForAdmin` | Eine reine Leitung sieht bestimmte Einträge **nicht** — die Leitung ist keine Obermenge der Betreuungskraft | `frontend/src/components/dashboard/sidebar.tsx:568` |
| `config:read` | Ohne dieses Recht fehlen die Einträge `Essensplan` und `Elternmitteilungen`, weil ihr Zustand nur aus dem Einstellungs-Schema kommt | `frontend/src/components/dashboard/sidebar.tsx:464-476,545-552` |
| `admin:*` | Alleiniges Recht zum Verfassen von Elternmitteilungen | `frontend/src/components/dashboard/sidebar.tsx:448` |
| `grade_transitions:read` | Eintrag `Jahrgangswechsel` | `frontend/src/components/dashboard/sidebar.tsx:522-525` |
| `guardians:financial` | Eintrag `Bankverbindungen` | `frontend/src/components/dashboard/sidebar.tsx:546` |
| `users:absence` (zusammen mit `users:read`) | Krank-/Entschuldigt-Aktionen auch für Kinder, die man sonst nicht bearbeiten darf | `backend/auth/authorize/student_absence.go:30-60`; `backend/api/students/types.go:213-219` |
| `users:checkin` | An-/Abmelden über die Web-App | `backend/api/students/api.go:313,317` |
| Systemrolle `lehrkraft` | Nur Klassenansicht und Nachrichten; die operative Sicht bleibt immer `own` | `backend/auth/authorize/operational_overview.go:27-43`; `backend/database/migrations/001015278_lehrkraft_role.go:58-75` |
| Sorgeberechtigten-Rechte (`parent_portal.*`) | Pro Kind unterschiedlich; verrechnen sich mit den Schul-Einstellungen zu einem Merkmalssatz | `backend/services/parent/parent_write_service.go:693-716`; `.claude/rules/guardian-parent-permissions.md` |
| `display:read` / `display:manage` | Eintrag `Info-Displays` zusätzlich zur Einstellung | `frontend/src/components/dashboard/sidebar.tsx:154` |
| `config:read` + `users:read` (beide) | Eintrag `Statistik` | `frontend/src/components/dashboard/sidebar.tsx:198, 591-596` |
| `calendar:own` | Eintrag `Mein Kalender` | `frontend/src/components/dashboard/sidebar.tsx:135, 584-590` |
| `config:manage` | Einzige Planungsseite, die auch Nicht-Admins sehen: `Abrechnung` | `frontend/src/lib/planning-navigation.ts:77`; `frontend/src/components/dashboard/sidebar.tsx:491-497` |
| Zusammengesetzte Regeln | `Anfragen` fasst mehrere Rechte zusammen (`users:update`, `users:absence`+`users:read`, `vacation:approve`) — als einfache Permission nicht ausdrückbar | `frontend/src/lib/change-request-access.ts:86-90`; `frontend/src/components/dashboard/sidebar.tsx:567` |

**Mechanik:** Berechtigungen kommen aus den JWT-Claims in die Sitzung (`frontend/src/server/auth/shared.ts:764`) und werden mit reinen Funktionen geprüft — `hasPermission`, `hasRole`, `isAdmin`, `hasEffectiveAdminScope`, `isCaregiver` in `frontend/src/lib/auth-utils.ts:6-100`, inklusive der Wildcard-Regeln (`admin:*`, `*:*`). Einen Berechtigungs-Context oder `usePermissions`-Hook gibt es nicht.

### 4.5 Portal-spezifische Unterschiede

| Portal | Besonderheit für die Hilfe | Beleg |
|---|---|---|
| Tenant-Portal (`{slug}.…`) | Bekommt 15 aufgelöste Einstellungen als Metadaten, damit auch Personen ohne `config:read` die richtige Navigation sehen | `backend/api/auth/tenant_handlers.go:141-157` |
| Schul-Portal „moto schule" | Nur Klassenansicht und Nachrichten; operative Sicht immer `own`; verlinkt heute auf die vollständige OGS-Anleitung | `backend/auth/authorize/operational_overview.go:27-43`; ADR 0009 |
| Eltern-Portal | Hat **keinen** Hilfebereich. Die Navigation kennt genau **zwei** Schalter — `Neuigkeiten` und `Essensplan` (`frontend/src/components/parent/shell/parent-nav-items.ts:29,88-103`, aufgelöst in `frontend/src/components/parent/shell/parent-shell.tsx:36-47`) —, und beide sind **über alle verknüpften Schulen aggregiert**: eine einzige Schule mit eingeschaltetem Feature lässt den Eintrag erscheinen (`frontend/src/lib/hooks/use-parent-news-enabled.ts:27-52`, `frontend/src/lib/hooks/use-parent-meal-plan-enabled.ts:11-30`). Alles andere entscheidet der Merkmalssatz je Kind aus Einstellung × Sorgeberechtigten-Recht × Betreuungsende. | `backend/services/parent/parent_write_service.go:593-716`; ADR 0009 |
| Operator-Portal | Intern; von der Verständlichkeitsregel ausgenommen und für die Hilfe nicht relevant | `.claude/rules/verstaendlichkeit.md` |
| Kiosk (PyrePortal) | Eigene deutsche Texte, die auf Backend-Fehlerzeichenketten gemappt sind | `PyrePortal/src/services/apiErrors.ts` |

### 4.6 Datenzustände, die wie eine Einstellung wirken

| Zustand | Sichtbarer Effekt | Beleg |
|---|---|---|
| Kein Raum „Schulhof" vorhanden | Schulhof-Button am Tablet fehlt trotz eingeschalteter Einstellung, ohne Meldung | `PyrePortal/src/hooks/pages/useActivityScanningPage.ts:286-297` |
| Kein Raum „WC"/„Toilette" | Toiletten-Button fehlt entsprechend | `PyrePortal/src/hooks/pages/useActivityScanningPage.ts:300-308` |
| Kind hat keine Gruppe | Bei ausgeschaltetem „Nach Hause in jedem Raum" erscheint `nach Hause` nie | `backend/services/iot/checkin/checkout_gate.go:88-93` |
| Betreuung des Kindes beendet | Im Eltern-Portal gehen **alle** Schreibfunktionen aus, Lesefunktionen bleiben | `backend/services/parent/parent_write_service.go:688-701` |
| Keine laufende Aufsicht | `Aktuelle Aufsicht` zeigt einen Leerzustand, dessen Text sich nach `nfcEnabled` richtet („an einem Terminal" vs. „in der Web-App") | `frontend/src/components/active-supervisions/states.tsx:71,93` |
| Keine zugewiesenen Aktivitäten | Tablet zeigt „Sie haben derzeit keine zugewiesenen Aktivitäten." | `PyrePortal/src/pages/CreateActivityPage.tsx:25-27` |
| Offene Anwesenheit am selben Tag | Der Wechsel des Anwesenheits-Modus wird abgelehnt | `backend/services/config/operator_settings_service.go:165-190` |

### 4.7 Env-Fallbacks bestehender Einstellungen

Die Registry kennt **keine** Umgebungsvariablen; ein `Resolve*` liefert bei fehlendem Override den Registry-Standard. Nur Konsumenten, die vorher `HasTenantOverride` fragen, greifen auf eine Variable zurück. Real vorhanden:

| Einstellung | Env-Fallback | Beleg |
|---|---|---|
| `operations.student_daily_checkout_time` | `STUDENT_DAILY_CHECKOUT_TIME` | `backend/services/iot/checkin/checkout_gate.go:28-31` |
| `security.ogs_device_pin` | `OGS_DEVICE_PIN` (nur wenn kein Settings-Service da ist) | `backend/auth/device/device_auth.go:235-246` |
| `enrollment.require_captcha`, `…captcha_secret_key`, `…captcha_site_key` | `ENROLLMENT_REQUIRE_CAPTCHA`, `ENROLLMENT_CAPTCHA_SECRET_KEY`, `ENROLLMENT_CAPTCHA_SITE_KEY` | `backend/services/enrollment/captcha_service.go:83,141,151` |
| `invitations.guardian_token_expiry_hours` | eigene Variable | `backend/services/auth/guardian_invitation_service.go:139` |
| Scheduler-Einstellungen | `CLEANUP_SCHEDULER_ENABLED`, `SESSION_END_SCHEDULER_ENABLED`, `SESSION_CLEANUP_*`, `STATUS_FLAG_CLEAR_ENABLED` u. a. | `backend/services/scheduler/scheduler.go:733-738,1160-1165,1272-1290,1696` |

**Nicht verwechseln:** Registry-Standard ≠ API-Fallback. `GET /api/iot/config` antwortet bei einem Auflösungsfehler mit `raumwechsel/schulhof/wc = true`, obwohl die Registry-Standards `true/false/false` lauten (`backend/api/iot/config.go:58-73` gegen `backend/services/config/defaults/devices.go:8-63`). Für die Anleitung ist der **Registry-Standard** maßgeblich. Dasselbe Muster bei `checkout.daily_checkout_from_all_rooms_enabled`: Registry `true`, Code-Fallback bei Lesefehler `false` (`backend/services/iot/checkin/checkout_gate.go:191-205`).

---

## 5. Auswirkungen auf die bestehende Hilfe

Legende: **kein Unterschied** · **bedingter Hinweis** · **abweichende Schritte** · **anderes Bild** · **eigenes Thema/Variante**

### 5.1 `guideEntryPoints` (`guide-data.ts:120-165`)

| Karte | Auslöser | Bedarf |
|---|---|---|
| „Ersteinrichtung" | `presence_mode`, `nfc_enabled` | **bedingter Hinweis** — die Reihenfolge Räume → Gruppen → Aktivitäten gilt im Binär-Modus nicht |
| „Die App im Alltag" | `presence_mode`, `group_mode` | **bedingter Hinweis** auf der Karte, Varianten in den Themen |
| „NFC Erste Schritte" | `nfc_enabled` | **bedingter Hinweis** — als „nur mit NFC-Tablets" kennzeichnen, nicht verstecken (ADR 0009) |
| „NFC & Tablets" | `nfc_enabled` | **bedingter Hinweis**, gleiche Begründung |

**Querschnitts-Befund für alle Themen mit Wegbeschreibung:** Die mobile Navigation
(`frontend/src/components/dashboard/mobile-bottom-nav.tsx`) enthält die Einträge `Info-Displays`,
`Tagesauswertung` und `Dateien` **überhaupt nicht** — sie existieren nur in der Desktop-Seitenleiste.
Jede Anleitung, die „öffnen Sie in der Seitenleiste …" schreibt, ist auf dem Telefon für diese drei
Themen falsch. Betrifft `info-displays-erstellen`, `info-displays-verwalten`, `tagesauswertung` und
`dateiablage`.

### 5.2 `setupChapters` (`guide-data.ts:234-571`)

| Thema | Auslöser | Bedarf |
|---|---|---|
| `konto-erstellen`, `passkey-einrichten` | `security.mfa_mode` | **bedingter Hinweis** (zusätzlicher Schritt bei aktiver Zwei-Faktor-Pflicht) |
| `app-zum-home-bildschirm`, `…-android` | — | **kein Unterschied** |
| `mitarbeitende-anlegen` | Rollenmodell | **bedingter Hinweis** (welche Rolle welchen Bereich sieht) |
| `raeume-anlegen` | `presence_mode` | **eigenes Thema/Variante** — im Binär-Modus gibt es die Seite `Räume` nicht |
| `gruppen-anlegen` | `group_mode` | **bedingter Hinweis** |
| `aktivitaeten-anlegen` | `presence_mode`, `nfc_enabled` | **eigenes Thema/Variante** — Eintrag entfällt in beiden Fällen |
| `kinder-importieren`, `kind-manuell-anlegen` | `enrollment.collect_*`, `enrollment.grade_level_max` | **bedingter Hinweis** |
| `betreuungszeiten-pflegen`, `…-mehrere-kinder` | `operations.require_pickup_offering_review`, `enrollment.care_offerings_enabled` | **bedingter Hinweis** |
| `go-live-check` | `nfc_enabled`, `presence_mode` | **abweichende Schritte** — die Checkliste prüft heute Dinge, die es je nach Konfiguration nicht gibt |

### 5.3 `appChapters` (`guide-data.ts:573-2388`)

| Thema | Auslöser | Bedarf |
|---|---|---|
| `home` | `presence_mode`, `group_mode`, `nfc_enabled`, `birthday_display_enabled` | **abweichende Schritte** + **anderes Bild** (Zahl der Info-Karten ändert sich) |
| `kindersuche` | `presence_mode` (Ortslabels), `student_photos_enabled`, `tracking.indicators_enabled` | **bedingter Hinweis** + **anderes Bild** |
| `kinderdetailansicht` | `gdpr.attendance_log_enabled`, `feedback.enabled`, `presence_mode` | **bedingter Hinweis** |
| `meine-gruppen` | `presence_mode` | **eigenes Thema/Variante** — im Binär-Modus zusätzlicher Schnell-Anwesenheitsmodus mit eigener Bedienung |
| `aktuelle-aufsicht` | `operational_overview_scope`, `care_concept`, `web_spontaneous_activities_enabled`, `presence_mode`, `nfc_enabled` | **abweichende Schritte** — dichteste Stelle der ganzen Anleitung; vier Schalter treffen hier zusammen |
| `tagesauswertung` | `gdpr.attendance_log_enabled` (Standard aus) | **bedingter Hinweis** — der Eintrag fehlt im Auslieferungszustand |
| `statistik`, `abwesenheiten` | `sick_clear_mode`, `excused_clear_mode`, `status_flag_clear_time` | **bedingter Hinweis** |
| `notfall` | `operations.emergency_list_health_info` | **bedingter Hinweis** — die Seite darf keine Gesundheitsspalte versprechen, die das PDF nicht hat |
| `erinnerungen` | die sechs `reminders.*` (alle Standard aus) | **bedingter Hinweis** |
| `aktivitaeten` | `nfc_enabled`, `presence_mode` | **eigenes Thema/Variante** — Eintrag entfällt in beiden Fällen |
| `raeume` | `presence_mode` | **eigenes Thema/Variante** |
| `team-chat` | `operations.staff_messaging_enabled` (Standard aus) | **bedingter Hinweis** |
| `mitarbeiter`, `mitarbeiter-admin-profil` | Rollen | **bedingter Hinweis** |
| `lehrkraft-klassenansicht`, `lehrkraft-aufsichten`, `lehrkraft-nachrichten` | Portal `moto schule`, `operational_overview_scope` | **bedingter Hinweis** — die Freigabe „Sicht auf alle Räume" gilt für Lehrkräfte nicht |
| `abrechnung-vorbereiten` | `payroll.*` (alle leer im Standard) | **bedingter Hinweis** |
| `listen-aus-betreuungsslots` (Tageslisten) | `timetable.slot_list_*_cutoff` | **bedingter Hinweis** (Listennamen und Zuschnitt sind schulabhängig) |
| `zeiterfassung`, `zeiterfassung-urlaub-historie` | `time_tracking_*`, `tracking.auto_checkout_*`, `federal_state` | **bedingter Hinweis** |
| `dateiablage` | `files.staff_upload_enabled` (Standard aus), `files.max_storage_mb` | **bedingter Hinweis** |
| `anmeldungen-*`, `anmeldephasen`, `betreuungsangebote`, `anmeldeformulare` | `enrollment.*` (33 Einstellungen, plus Spalten je Phase) | **bedingter Hinweis**; die Rechtstext-Blöcke sind alle standardmäßig aus |
| `eltern-app-ueberblick`, `eltern-anfragen`, `elternmitteilungen`, `nachrichten` | die 14 `parent_*`-Einstellungen | **bedingter Hinweis** — was Eltern sehen, ist je Schule **und je Sorgeberechtigtem** verschieden. Zusätzlich: `elternmitteilungen` verlangt `admin:*` **und** `operations.parent_news_enabled`; `nachrichten` blendet bei ausgeschaltetem `operations.parent_notes_enabled` den Button „Neue Nachricht" aus (`frontend/src/app/[tenant]/(protected)/messages/page.tsx:126,160,173`) |
| `essensplan` | `operations.meal_plan_enabled` **und** `config:read` bzw. Admin | **bedingter Hinweis** — eine Betreuungskraft ohne `config:read` sieht den Eintrag nie, auch bei eingeschaltetem Essensplan (`frontend/src/components/dashboard/sidebar.tsx:464-476, 548-552`) |
| `info-displays-*` | `display.enabled` (Standard aus) | **bedingter Hinweis** |
| `einstellungen-ueberblick` | Zugriffsrichtlinien | **abweichende Schritte** — welche Reiter erscheinen, hängt davon ab, was für die Schule sichtbar ist |
| `einstellungen-sicht-auf-alle-raeume` | `operational_overview_scope` | **kein Unterschied** — das Thema ist korrekt; nur der Lehrkraft-Zusatz fehlt |
| `einstellungen-zustaendigkeit` | Zugriffsrichtlinien | **abweichende Schritte** — die Liste der „vom moto-Team betreuten" Einstellungen ist unvollständig (siehe Abschnitt 6, Frage 5) |
| `benachrichtigungen-auswaehlen`, `push-benachrichtigungen-aktivieren` | `notifications.*` | **bedingter Hinweis** |

### 5.4 `nfcChapters` (`guide-data.ts:2390-2996`) und `nfcQuickstartChapters` (`:173-232`)

| Thema | Auslöser | Bedarf |
|---|---|---|
| gesamter Bestand | `attendance.nfc_enabled` | **eigenes Thema/Variante** auf Anleitungsebene: gilt nur für Schulen mit NFC |
| `nfc-tablet-anmelden`, `mit-geraete-pin-anmelden` | `security.ogs_device_pin` | **bedingter Hinweis** — Standard-PIN `1234`, muss geändert werden |
| `nfc-geraete-pruefen` | `iot.device_online_window_minutes` | **kein Unterschied** |
| `nfc-aufsicht-auswahl`, `nfc-aufsicht-team-raum`, `nfc-aufsicht-bestaetigen` | `presence_mode` (**nur theoretisch**, siehe Abschnitt 6) | heute **kein Unterschied**; die Kapitel bleiben in beiden Modi korrekt |
| `nfc-kinder-einchecken` | `presence_mode` | **bedingter Hinweis** — im Binär-Modus lautet der Text „Willkommen, {Vorname}!" und die Raumzeile ist irreführend |
| `nfc-kinder-auschecken` | `checkout.raumwechsel/schulhof/wc_enabled` | **bedingter Hinweis** — Bild bleibt (`nfc-auschecken.webp`, alle vier Ziele), neue konkrete Bildunterschrift; zusätzlich **ein eigener Satz** für den Fall „alle Ziele aus" (dann fehlt der Dialog ganz). Siehe 3.3 |
| `nfc-kinder-nach-hause` | `daily_checkout_from_all_rooms_enabled`, `student_daily_checkout_time`, `per_student_checkout_enabled`, Schulhof-Raum | **abweichende Schritte** — `nach Hause` ist kein Schalter, sondern eine Bedingung; sie muss benannt werden, nicht verallgemeinert |
| `nfc-einstellungen-geraete` | die sechs Geräte-Einstellungen | **kein Unterschied** — das Thema ist korrekt und aktuell |
| `nfc-einstellungen-betrieb` | `presence_mode`, `student_daily_checkout_time` | **abweichende Schritte** — der Hinweis „Der Anwesenheits-Modus verändert grundlegend, wie das Tablet arbeitet" stimmt für die Web-App, für das Tablet heute nicht |
| `nfc-fehler-erkennung-netzwerk` | fehlender Schulhof-/WC-Raum, fehlgeschlagener Config-Abruf | **abweichende Schritte** — zwei belegte, heute undokumentierte Störungsbilder; der fehlende Raum ist der Gegenpart zur Bildunterschrift aus 3.3 |
| `nfc-lieferumfang`, `nfc-aufstellen-schritte` | Kiosk-Ziel (GKT / Wedge) | **bedingter Hinweis** — Raspberry Pi ist stillgelegt |

---

## 6. Unsicherheiten und offene Fragen

### Belegte Widersprüche, die eine Produktentscheidung brauchen

1. **Der Binär-Modus existiert am Tablet nicht.** Backend und `project-phoenix/CLAUDE.md` beschreiben einen Kiosk-Zweig (Raumauswahl aus, Raumwechsel/WC aus, Zwei- gegen Drei-Button-Modal); PyrePortal setzt ihn nicht um (`PyrePortal/src/services/api.ts:787` ist die einzige Fundstelle außerhalb von Tests; der offene Zweig ist in `backend/api/iot/checkin/handlers.go:466-472` kommentiert).
   **Frage an Produkt:** Soll die Anleitung den heutigen Ist-Zustand beschreiben, oder wird der Kiosk-Zweig vor der Überarbeitung umgesetzt? Bis zur Antwort darf zu Binär + Tablet nichts geschrieben werden. Betrifft direkt Arbeitspaket **D5** in `docs/hilfebereich-umbau-plan.md`, das heute vom Gegenteil ausgeht.
2. **Im Binär-Modus zeigt das Tablet fachlich falsche Texte.** Nach einem Check-in erscheint „Du bist jetzt in diesem Raum" (Rückfalltext, weil `room_name` leer ist, `PyrePortal/src/pages/ActivityScanningPage.tsx:612-616`), und nach einem Check-out erscheint „Wohin geht {Vorname}?" mit `Raumwechsel`, obwohl es keine Räume gibt.
   **Frage:** Ist das ein zu meldender Fehler? Für die Hilfe ist es keiner — er lässt sich nicht wegformulieren.
3. **Der Schulhof-Zustand `on_yard` ist nicht erreichbar.** Lese- und Anzeigepfad existieren, der Schreibpfad aus PyrePortal fehlt (`backend/models/active/attendance.go:26-33`).
   **Frage:** Gibt es die Kombination „binäre Anwesenheit + Schulhof" als Zustand überhaupt schon, oder beschreibt der Prototyp (`frontend/src/components/help/prototype/prototype-data.ts:290-300`) eine geplante Zukunft?
4. **`project-phoenix/CLAUDE.md` ist beim Ökosystem veraltet.** Dort steht „Raspberry Pi kiosk app (Tauri + React)" und moto-balenaOS als aktiver Deploy-Weg; PyrePortal nennt beide Ziele stillgelegt und den Tauri-Quellcode als entfernt (`PyrePortal/CLAUDE.md:26,41`). Die NFC-Anleitung sollte keine Pi-Hardware beschreiben, bevor das geklärt ist.
   **Frage an das Team:** Welche Hardware liegt heute tatsächlich im Versandkarton (Kapitel `nfc-lieferumfang`)?
5. **Das Thema `einstellungen-zustaendigkeit` ist unvollständig.** Es nennt als „vom moto-Team betreut" nur die Web-Anwesenheit und den Anwesenheits-Modus. Tatsächlich sind **21 Einstellungen** `operator_only`: `operations.presence_mode`, `attendance.web_enabled`, `attendance.nfc_enabled`, `operations.federal_state`, `operations.session_end_enabled`/`_time`/`_timeout_minutes`, `operations.session_cleanup_enabled`/`_interval_minutes`, `operations.session_abandoned_threshold_minutes`, `operations.session_inactivity_timeout_minutes`, `iot.device_online_window_minutes`, `enrollment.bookings_authoritative`, `enrollment.captcha_secret_key`, `enrollment.outbox_max_attempts`, `enrollment.outbox_worker_interval_seconds`, `enrollment.status_token_ttl_days`, `invitations.guardian_token_expiry_hours`, `timetable.materialization_enabled`/`_weekday`/`_weeks_ahead`. Umgekehrt sind **13 Einstellungen** `admin_only` und damit für das moto-Team unsichtbar: `security.ogs_device_pin` und die zwölf `payroll.*`.
   **Frage:** Soll das Thema die vollständige Liste nennen oder nur die für Schulen spürbaren?
6. **ADR 0009 nennt vier Kandidaten-Schalter; die Messung stützt nur drei.** `attendance.nfc_enabled`, `display.enabled` und `operations.presence_mode` steuern tatsächlich die Navigation; `enrollment.enabled` **nicht** — der Bereich `Anmeldungen` ist rein rollen-gesteuert (`frontend/src/components/dashboard/sidebar.tsx`, kein `enrollment`-Gate). Umgekehrt steuern vier weitere Schalter die Navigation, die im ADR fehlen: `gdpr.attendance_log_enabled`, `operations.staff_messaging_enabled`, `operations.meal_plan_enabled` und `timetable.enabled`.
   **Frage:** ADR 0009 und `docs/hilfebereich-umbau-plan.md` §6 entsprechend nachziehen? Auch die dort genannte Zahl „rund 86 Einstellungen" liegt heute bei 162.
7. **Drei Bereiche haben keinen mobilen Einstieg.** `Info-Displays`, `Tagesauswertung` und `Dateien` stehen nur in der Desktop-Seitenleiste; in `frontend/src/components/dashboard/mobile-bottom-nav.tsx` kommen sie nicht vor — weder in der Hauptleiste noch im Überlauf-Menü.
   **Frage an Produkt:** Ist das Absicht? Falls ja, muss die Anleitung es sagen; falls nein, ist es ein Fehler und die Anleitung sollte nicht darum herumformulieren.
8. **`Essensplan` und `Elternmitteilungen` hängen an `config:read`.** Beide Einträge werden aus dem Einstellungs-Schema gelesen, und dieses Schema holt das Frontend nur für Personen mit `config:read` oder Admin-Rolle (`frontend/src/components/dashboard/sidebar.tsx:464-476`). Eine Betreuungskraft sieht den Essensplan also nie, auch wenn die Schule ihn eingeschaltet hat.
   **Frage:** Gewollt oder Nebenwirkung? Falls gewollt, gehört es in die Anleitung; falls nicht, sollten die beiden Schalter wie die anderen über `/auth/tenant/resolve` reisen.
9. **Drei deklarierte Felder haben keinen Konsumenten.** `TenantSettings.primaryColor` (`frontend/src/lib/tenant-api.ts:15`), `ChildFeatures.request_submit_enabled` und `ChildFeatures.has_open_change_request` (`frontend/src/lib/parent-api.ts:173,186`) werden nirgends gerendert. Für die Anleitung ohne Folgen, für die Aufräumarbeit relevant.
10. **Screenshot-Referenz: welche Konfiguration zeigen wir?** Für den Auschecken-Dialog ist die Antwort in 3.3 vorgeschlagen — die *reichste* Variante plus konkrete Bildunterschrift, weil eine Teilmenge wiedererkennbar ist, eine Obermenge aber nicht erklärbar. `nfc-auschecken.webp` bleibt damit unverändert.
   **Frage an das Team:** Gilt dieses Prinzip als Regel für alle 77 Bilder, oder ist es eine Einzelfallentscheidung? Die Regel funktioniert nur dort, wo Konfiguration Elemente **weglässt**; wo sie ganze Seiten oder Abläufe austauscht (Rang 1 und 2), braucht es weiterhin eigene Bilder. Die Entscheidung gehört vor Arbeitspaket D3.

### Vermutungen (ausdrücklich als solche gekennzeichnet)

- **Vermutung:** Die meisten Pilotschulen laufen mit `presence_mode = detailed` und `nfc_enabled = true`, also im heute dokumentierten Fall. Belegt ist das nicht — dazu müsste man `config.setting_values` je Mandant auswerten (`backend/cmd/settings.go` bietet dafür `settings overrides`). Empfehlung: vor der inhaltlichen Überarbeitung einmal messen, welche Werte real gesetzt sind. Das ersetzt jede Schätzung darüber, welche Variante überhaupt jemand liest.
- **Vermutung:** `operations.group_mode` und `operations.care_concept` werden verwechselt, weil beide im selben Reiter, in derselben Kategorie `organisation` und direkt untereinander stehen. Der Beschreibungstext von `group_mode` grenzt bereits gegen „Sicht auf alle Räume" ab (`backend/services/config/defaults/operations.go:394-396`), aber nicht gegen `care_concept`. Das ist genau das Muster „Zwillingsüberschriften" aus `.claude/rules/verstaendlichkeit.md`.
- **Vermutung:** Der Hinweis in `nfc-einstellungen-geraete`, `Schulhof` und `Toilette` legten „automatisch einen passenden Raum an", ist korrekt (`backend/services/facilities/settings_sideeffects.go:13-29`), aber unvollständig: Wird die Einstellung später ausgeschaltet, bleibt der Raum bestehen. Der umgekehrte Fall (Einstellung an, Raum gelöscht) lässt den Button stillschweigend verschwinden. Nicht abschließend geprüft, ob es dafür einen Schutz gibt.

### Methodische Grenzen dieser Recherche

- Geprüft wurde der Stand von `development` am 30.08.2026, ohne die Anwendung zu starten. Die Aussagen stützen sich auf Quellcode und Tests, nicht auf Bildschirmaufnahmen.
- Die Bewertung „sichtbarer Effekt" bezieht sich auf Tenant-Portal, Eltern-Portal und Kiosk. Das Operator-Portal wurde nicht betrachtet.
- Für die 33 `enrollment.*`-Einstellungen wurde die Wirkung auf das öffentliche Formular nur stichprobenartig verfolgt; für eine Anmeldungs-Anleitung wäre eine eigene Recherche nötig.
