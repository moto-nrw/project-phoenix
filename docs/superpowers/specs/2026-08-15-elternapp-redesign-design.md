# Eltern-App: Neuaufbau der Oberfläche

**Datum:** 2026-08-15
**Branch:** `feat/parent-app-redesign`
**Status:** Entwurf, zur Abstimmung

Diese Spezifikation ist Arbeitsmaterial für genau diesen Umbau und wird am Ende
des Vorhabens gelöscht.

---

## 1. Auftrag

Eltern melden zurück, dass die Eltern-App zu kompliziert und nicht
selbsterklärend ist. Zielgruppe sind Eltern von Grundschul- und
Kita-Kindern, ausdrücklich auch solche mit wenig digitaler Erfahrung und ohne
sichere Deutschkenntnisse. Die App soll für Mobile, Tablet und Desktop nach
gängigen Gestaltungsregeln neu aufgebaut werden: mehr Farbe, größere Schrift,
klarere Wege, und trotzdem hochwertig, so wie man eine Kita-App heute erwartet.

---

## 2. Ist-Zustand: was die Eltern-App kann

Erhoben aus `backend/api/parent/api.go`, also verbindlich und nicht geschätzt.
65 Endpunkte in 15 Funktionsgruppen.

| # | Funktion | Endpunkte | Nutzungsfrequenz |
|---|---|---|---|
| 1 | Nachrichten mit der OGS, ein Verlauf pro Kind | 5 | täglich/wöchentlich |
| 2 | Neuigkeiten, Aushänge, Umfragen | 5 | wöchentlich |
| 3 | Krank- und Abwesenheitsmeldung | 4 | selten, aber dringend |
| 4 | Abholzeit für einen Tag ändern | 3 | wöchentlich |
| 5 | Essensplan | 1 | wöchentlich |
| 6 | Kalender, Termine, Einladungen, ICS-Abo | 6 | monatlich |
| 7 | Reguläre Betreuungszeiten ändern | 3 | 1-2x im Jahr |
| 8 | Betreuungsangebote und AGs | 4 | 1-2x im Jahr |
| 9 | Stammdaten | 4 | selten |
| 10 | Sorgeberechtigte, Kontakt und Abholrecht | 3 | selten |
| 11 | Zweites Elternteil einladen | 3 | einmalig |
| 12 | Neue Anmeldung an einer Schule | 4 | einmalig |
| 13 | Benachrichtigungen und Push | 6 | einmalig |
| 14 | Portalsprache (de/en/ru/sq) | 2 | einmalig |
| 15 | Produktfeedback an moto | 12 | fast nie |

**Die Kernaussage:** Fünf Gruppen decken den Alltag ab, zehn sind Einmal- oder
Jahresvorgänge. Heute stehen alle gleichgewichtet nebeneinander. Das ist die
Ursache der Überforderung, nicht die Anzahl der Funktionen.

### Beobachtungen aus der Ist-Oberfläche

Geprüft an Screenshots in 390×844, 834×1194 und 1440×900.

1. **Keine Farbe.** Weiß, Grau, ein grüner Avatarkreis. Drei Aktionen als drei
   identische graue Pillen mit gleichem Gewicht.
2. **Desktop verschenkt zwei Drittel der Fläche.** Bei 1440×900 endet der Inhalt
   nach etwa 600 px Höhe, rechts daneben bleibt eine leere Spalte.
3. **Die Begrüßungskarte kostet ein Viertel des Handy-Bildschirms** und sagt
   nichts Handlungsrelevantes.
4. **Doppelte Verneinungen als Status:** "Keine Abmeldung", "Keine Abholung
   heute". Eltern müssen daraus ableiten, dass alles normal ist.
5. **Verwaltungsvokabular:** Stammdaten, Betreuungsangebote, Tagesmeldung,
   "Grund (Pflichtfeld)", Kicker in Versalien über fast jeder Überschrift.
6. **Mehrsprachigkeit halb fertig.** Vier Sprachkataloge existieren, aber
   Kalender, Nachrichtenliste, verknüpfte Konten und Guardian-Invite sind hart
   auf Deutsch verdrahtet.
7. **Die Eltern-App trägt die Personal-Hülle.** Dieselbe `AppShell`, eine
   1510-Zeilen-Sidebar und eine 1063-Zeilen-Bottom-Nav mit
   `mode === "parent"`-Verzweigungen. Der Familienkalender ist die
   1470-Zeilen-Personalkomponente `PersonalCalendar`.

---

## 3. Kundenfeedback

Rückmeldungen der Ganztagskoordinatorin von Schule am Berg, August 2026:

- Die Rubrik **Betreuungszeiten** brauchen Eltern nicht. Die Abgrenzung zu
  Betreuungsangeboten ist unklar.
- Bei den Betreuungsangeboten reicht **die gebuchte Betreuung**. Der Block
  **AGs und Gruppen** irritiert; Eltern sollen dort ausdrücklich nichts anmelden.
- Eine Mutter mit Kita-App-Vergleich braucht tagesaktuell **nur**: Kind ist im
  Ganztag angekommen bzw. wurde abgemeldet. Dazu einmalig die Übersicht der
  gebuchten Betreuungszeiten.
- **Zu viele Informationen richten Schaden an:** Nach einer Änderung der
  Ankunftszeiten laut Stundenplan versuchte ein Vater, die Ankunftszeit ohne
  Kenntnis des Stundenplans wieder zu ändern.
- Eine Elterninfo per Push kam **nicht an**, die betroffene Mutter erfuhr erst
  auf Nachfrage davon.

### Zugehörige Issues

| Issue | Inhalt |
|---|---|
| #2308 | Oberfläche auf häufige Elternaufgaben ausrichten (Dach, priority: high) |
| #2252 | Tagesaktueller Ankunfts- und Abmeldestatus (ready-for-agent) |
| #2250 | Kompakte Betreuungsübersicht |
| #2302 | Betreuungszeiten vollständig entfernen |
| #2303 | AGs und Gruppen vollständig entfernen |
| #2292 | Kurzfristige Abmeldungen einschränken, Krankmeldung bleibt direkt |
| #2293 | Abmeldung muss den Abholzeitpunkt eindeutig machen |
| #2304 | Abholänderung braucht konkrete Uhrzeit und Pflichtgrund |
| #2297 | Push-Aktivierung sichtbar machen |
| #2305 | Benachrichtigungen beim ersten Login einrichten |
| #2306 | Anleitung zum Home-Bildschirm integrieren |
| #2307 | Neue OGS-Nachrichten zusätzlich per E-Mail ankündigen |
| #2326 | Feedback-Funktionen aus allen Portalen entfernen |
| #2295, #2296 | Ausblendbarkeit per Einstellung, **entfällt**, siehe Entscheidung E1 |

---

## 4. Entscheidungen

### E1: Keine neuen Tenant-Einstellungen

Eine Einstellung ist nur gerechtfertigt, wenn Schule A ausdrücklich Verhalten A
und Schule B ausdrücklich Verhalten B verlangt hat. Für keinen der hier
behandelten Punkte trifft das zu. Wir entfernen und vereinfachen restriktiv und
warten ab, ob jemand widerspricht.

Konkret entfällt damit:

- die Ausblendbarkeit aus #2295 und #2296 zugunsten der vollständigen Entfernung
  in #2302 und #2303,
- der Opt-in-Schalter für die kompakte Ansicht aus #2250: die kompakte Ansicht
  ist die einzige Ansicht,
- eine Sichtbarkeitseinstellung für den Tagesstatus.

### E2: Neuigkeiten und Nachrichten bleiben getrennt, die Startseite führt zusammen

Aushänge gehen von der Schule an alle und erlauben keine Antwort; Nachrichten
sind 1:1 und erwarten eine Antwort. Beide zu verschmelzen erzeugt falsche
Erwartungen. Alle untersuchten Kita-Apps trennen sie ebenfalls. Der einzige Ort,
an dem beides zusammenläuft, ist der Bereich "Zu erledigen" auf der Startseite.

### E3: Eigene Eltern-Hülle

Die Eltern-App bekommt eine eigene Navigation und ein eigenes Layout, gelöst von
`Sidebar`, `MobileBottomNav` und `PersonalCalendar` des Personal-Portals. Die
`mode === "parent"`-Verzweigungen in den Personal-Komponenten entfallen.

### E4: Eltern sehen nur "da" oder "nicht da"

Der Tagesstatus ist eine reduzierte Projektion. Räume, Raumwechsel, Besuchs-
historie, Rohereignisse und Mitarbeitendennamen werden nie ausgeliefert. Fachlich
deckt sich das mit dem vorhandenen `operations.presence_mode`: Eltern bekommen
immer die binäre Sicht, unabhängig vom Modus der Schule. `active.visits` wird
für Eltern nie gelesen.

---

## 5. Zielbild: Informationsarchitektur

### Navigation

Mobile Bottom-Nav und Desktop-Sidebar zeigen dieselben Ziele. Kein Alltagsziel
liegt hinter "Mehr".

| Ziel | Inhalt |
|---|---|
| **Start** | Zu erledigen, darunter je Kind eine Tageskarte mit den drei Hauptaktionen |
| **Nachrichten** | Chat mit der OGS, Badge für Ungelesenes |
| **Neuigkeiten** | Aushänge, Umfragen, Termine zum Zusagen, Badge |
| **Mein Kind** (bei mehreren: **Meine Kinder**) | Heute, gebuchte Betreuung, Daten, Kontakte, Essensplan |
| **Mehr** | Kalender, Benachrichtigungen, Sprache, Anmeldung, Abmelden |

Anmeldung, Benachrichtigungen und Sprache sind Einmalvorgänge und gehören nicht
in die Hauptnavigation. Der Kalender bleibt erreichbar, ist aber für den
Alltag nachrangig gegenüber "Zu erledigen".

**Entfällt vollständig aus der Navigation:** Betreuungszeiten (#2302), AGs und
Gruppen (#2303), Produktfeedback (#2326), "Bald im Elternportal".

### Startseite

```
┌──────────────────────────────────────────┐
│  Guten Morgen, Sabine                    │   Begrüßung einzeilig, kein Kicker
├──────────────────────────────────────────┤
│  ZU ERLEDIGEN                            │
│  ┌────────────────────────────────────┐  │   nur Dinge mit offener Handlung:
│  │ ● Umfrage: Sommerfest   [Antworten]│  │   ungelesene Aushänge,
│  │ ● Neue Nachricht der OGS  [Lesen]  │  │   offene Umfragen,
│  │ ● Elternabend 01.09.  [Zu-/Absagen]│  │   Termineinladungen ohne Antwort,
│  └────────────────────────────────────┘  │   ungelesene Nachrichten
├──────────────────────────────────────────┤
│  ┌────────────────────────────────────┐  │
│  │ ▌ FS  Felix Schneider              │  │   farbiges Statusband links
│  │ ▌     Klasse 1a                    │  │
│  │ ▌                                  │  │
│  │ ▌  Seit 12:38 Uhr im Ganztag       │  │   Tagesstatus, groß, mit Icon
│  │ ▌  Abholung heute: 15:00 Uhr       │  │
│  │                                    │  │
│  │  [Krank melden] [Abholung] [OGS]   │  │   drei Aktionen, farbig, 48 px
│  └────────────────────────────────────┘  │
└──────────────────────────────────────────┘
```

Ist nichts offen, zeigt "Zu erledigen" einen ruhigen grünen Zustand
("Alles erledigt"), keine leere Liste und keine Platzhalterkarten. Ein leerer
Neuigkeitenbereich verdrängt nie die Tageskarten (#2308).

### Kinderprofil

Vier Abschnitte in dieser Reihenfolge:

1. **Heute** — Tagesstatus, geplante Abholung, die drei Hauptaktionen
2. **Betreuung** — nur die tatsächlich gebuchte Betreuung mit Wochentagen
3. **Daten** — Stammdaten des Kindes
4. **Kontakte** — Sorgeberechtigte, Abholberechtigte, verknüpfte Konten

Dauerhafte Gehzeiten bleiben erhalten, werden mobil aber als eine Karte je
Wochentag dargestellt statt als Tabelle. Aus dem Stundenplan abgeleitete
Ankunftszeiten erscheinen **nicht** als frei änderbare Elternangabe, das war die
Ursache des Vorfalls mit der Ankunftszeit.

---

## 6. Tagesstatus: fachlicher Kontrakt (#2252)

### Zustände

| Zustand | Anzeige | Herleitung | Farbe |
|---|---|---|---|
| `expected` | "Heute ab 12:30 Uhr erwartet" | Betreuungstag, erwartete Zeit noch nicht erreicht, keine offene Anwesenheit | Blau |
| `not_arrived` | "Noch nicht im Ganztag angekommen" | erwartete Zeit überschritten, keine offene Anwesenheit | Blau |
| `present` | "Seit 12:38 Uhr im Ganztag" | offene Anwesenheit (`check_out_time IS NULL`) | Grün |
| `left` | "Um 15:12 Uhr abgeholt" | Anwesenheit heute vorhanden und geschlossen | Grün |
| `absent` | "Heute abgemeldet" | wirksamer Eintrag in `active.student_status_days` | Rot |
| `no_care` | "Heute keine Betreuung" | kein Betreuungstag laut Betreuungsplan | Grau |
| `unknown` | "Status derzeit nicht verfügbar" | Daten nicht belastbar ladbar oder Schule pflegt keine Anwesenheit | Grau |

### Datenquellen

- **Anwesenheit:** `active.attendance`, ein Datensatz je Kind und Tag,
  `check_out_time IS NULL` bedeutet anwesend.
- **Geplante Abwesenheit:** `active.student_status_days`.
- **Erwartete Zeit und Betreuungstag:** Betreuungsplan des Kindes plus
  wirksame Ausnahme des Tages.
- **Niemals:** `active.visits`.

### Funktioniert mit und ohne NFC

`performCheckIn` in `services/active/attendance_service.go` unterstützt bereits
web-basierte Anwesenheit: bei `deviceID == 0` fällt es auf ein virtuelles Gerät
`WEB-MANUAL-001` zurück. `active.attendance` ist damit die eine Quelle, egal ob
Kiosk-Scan oder Häkchen im Personal-Portal.

**Wichtiger Rückfall:** Eine Schule, die weder NFC nutzt noch Anwesenheit
pflegt, würde sonst den ganzen Tag "Noch nicht angekommen" zeigen und Eltern
grundlos beunruhigen. Existiert für heute **überhaupt kein** Anwesenheitsdatensatz
für die Gruppe des Kindes, liefern wir `unknown`, nicht `not_arrived`.

### Endpunkt

```
GET /parent/me/children/{studentId}/today
→ {
    "state": "present",
    "since": "12:38",              // nur bei present
    "until": null,                 // nur bei left
    "expected_from": "12:30",      // nur bei expected
    "pickup_today": "15:00",
    "pickup_changed": false
  }
```

Zugriff nur mit Guardian-Verknüpfung samt Elternportal-Berechtigung für genau
dieses Kind. Aktualisierung über die vorhandene Eltern-Echtzeitverbindung bei
Check-in-, Checkout- und Abwesenheitsereignissen.

---

## 7. Gestaltung

### Farbe bekommt Bedeutung

Farbe trägt nie allein eine Information; immer zusammen mit Icon und Text
(Regel für geringe digitale Kompetenz und für Farbfehlsichtige). Alle Werte aus
der moto-Palette über `moto-*`-Utilities bzw. `LOCATION_COLORS`, nie als roher
Hex-Wert in einer Utility-Klasse.

| Bedeutung | Farbe | Token |
|---|---|---|
| Kind ist da, alles erledigt | Grün `#83CD2D` | `GROUP_ROOM` |
| Information, erwartet, Nachricht | Blau `#5080D8` | `OTHER_ROOM` |
| Offene Handlung, Antrag wartet | Orange `#F78C10` | `SCHOOLYARD` |
| Krank, abgemeldet, Fehler | Rot `#DC2626` | `SICK` / `DANGER` |
| Keine Betreuung, unbekannt | Grau `#6B7280` | `HOME` |

### Schrift

| Ebene | Größe | Regel |
|---|---|---|
| Seitentitel | 24 px | einzeilig, kein Kicker darüber |
| Tagesstatus | 20 px, halbfett | die wichtigste Zeile der App |
| Fließtext, Buttonbeschriftung | 17 px | Untergrenze für alles Bedienbare |
| Sekundärtext | 15 px | Untergrenze insgesamt |

Ersatzlos gestrichen: Versalien-Mikrolabels in 11 px, wie sie heute über jedem
Feld stehen ("TAGESMELDUNG", "GRUND (PFLICHTFELD)").

### Buttons: eine Spezifikation für die ganze App

- **Höhe:** mindestens 48 px für jede bedienbare Fläche, auch für Icon-Buttons
  und Listeneinträge. Kein `size="md"` mit 36 px mehr in Elternansichten.
- **Beschriftung:** immer ein Verb, das die Folge benennt. "Krank melden", nicht
  "Speichern". Icon links, Text rechts, Icon nie allein.
- **Nur eine Hauptaktion je Bildschirm oder Dialog.** Alles andere ist sekundär.
- **Anordnung im Dialog:**
  - Mobile: fest angeheftete Fußzeile, Hauptaktion volle Breite, sekundäre
    Aktion darunter als Textbutton.
  - Desktop und Tablet: Paar rechtsbündig, Hauptaktion außen rechts.
- **Anordnung in Karten:** Aktionsreihe unten in der Karte, auf Mobile
  untereinander in voller Breite, ab `sm` nebeneinander gleich breit.
- **Zerstörende Aktionen** brauchen eine Rückfrage, die die Folge benennt.

### Formulare

- **Pflichtfelder** tragen einen Stern am Label plus einmal je Formular die
  Erklärung "Felder mit * müssen ausgefüllt werden". Der Stern allein reicht für
  die Zielgruppe nicht.
- **Fehler stehen direkt am Feld**, in Alltagssprache, mit dem nächsten Schritt.
  Nicht "Validierungsfehler", sondern "Bitte tragen Sie eine Uhrzeit ein".
- **Keine nativen Zeit- und Datumsfelder** mehr für Eltern. Uhrzeiten kommen als
  Auswahl aus den Betreuungsbausteinen des Kindes (#2293), Datumsangaben über
  den Kit-Datumswähler mit Schnellwahl "Heute" und "Morgen".
- **Folgen vor dem Absenden benennen:** "Die OGS wird darüber informiert." statt
  eines stummen Speicherns.

### Sprache

Alltagssprache statt Verwaltungsdeutsch, durchgehend in allen vier Katalogen.

| Statt | Besser |
|---|---|
| Stammdaten | Daten von Felix |
| Betreuungsangebote | Gebuchte Betreuung |
| Tagesmeldung | Heute |
| Grund (Pflichtfeld) | Warum? * |
| Keine Abmeldung | Felix wird heute betreut |
| Care Exception / Abholzeit-Ausnahme | Abholung ändern |

Lücken in der Mehrsprachigkeit werden geschlossen: Kalender, Nachrichtenliste,
verknüpfte Konten und Guardian-Invite bekommen Katalogeinträge in de, en, ru
und sq.

---

## 8. Responsive

| Breite | Muster |
|---|---|
| ab 320 px | eine Spalte, Bottom-Nav mit fünf Zielen, Aktionen volle Breite |
| ab 640 px (Tablet hoch) | Kinderkarten zweispaltig, Aktionen nebeneinander |
| ab 1024 px (Tablet quer, Desktop) | Sidebar links, Inhalt zweispaltig: "Zu erledigen" links, Kinderkarten rechts, damit die rechte Bildschirmhälfte nicht leer bleibt |
| ab 1440 px | Inhaltsbreite begrenzt, Karten wachsen mit statt zu strecken |

Prüfkriterien aus #2308: Kernaufgaben funktionieren bei 320 px Breite, 200 %
Zoom und vollständig per Tastatur.

---

## 9. Was entfällt

| Entfällt | Grund |
|---|---|
| Rubrik Betreuungszeiten | #2302, Abgrenzung zu Betreuungsangeboten unklar |
| Block AGs und Gruppen | #2303, irritiert, Anmeldung ohnehin nicht gewollt |
| Produktfeedback im Elternportal | #2326, gehört nicht in eine Eltern-App |
| "Bald im Elternportal" mit Platzhaltern | unfertige Navigationseinträge |
| Begrüßungskarte "Willkommen im Elternportal" | verbraucht ein Viertel des Bildschirms ohne Inhalt |
| Kicker in Versalien über Überschriften | zweite Textzeile vor der eigentlichen Information |
| "Neue Anmeldung" als Hauptaktion im Seitenkopf | #2308, Einmalvorgang |
| Stundenplanbasierte Ankunftszeit als änderbares Feld | #2250, Ursache falscher Elternänderungen |
| `PersonalCalendar` in der Eltern-App | Personalwerkzeug, für Eltern ungeeignet |

---

## 10. Benachrichtigungen

Nach Kundenfeedback der wichtigste ungelöste Punkt: Eine Elterninfo kam nicht an.

- **Beim ersten Login** führt ein Dialog durch die Aktivierung von
  Benachrichtigungen, wie in gängigen Apps üblich (#2305). Kein stiller
  Opt-in-Schalter irgendwo in den Einstellungen.
- **Anleitung zum Home-Bildschirm** für iPhone und iPad ist in die App
  integriert (#2306), weil Web-Push auf iOS ohne Home-Bildschirm-Installation
  nicht funktioniert.
- **Zusätzliche E-Mail** bei neuen OGS-Nachrichten (#2307): Betreff "Neue
  Nachricht in moto", kurze Vorschau, Schaltfläche "Antworten", die in die App
  führt. Die E-Mail ist der Rückfall, wenn Push nicht eingerichtet ist.
- Die Sichtbarkeit des Push-Status wird in der App geführt (#2297), damit Eltern
  erkennen, ob sie überhaupt erreichbar sind.

---

## 11. Offene Fragen

**F1 (#2292): Dürfen Eltern ihr Kind weiterhin selbst für den Tag abmelden?**
Die Schule will Krankmeldungen direkt lassen, aber spontane Tagesabmeldungen
unterbinden, weil unklar bleibt, ob das Kind nach dem Unterricht oder nach der
Randstunde geht. Das Issue schlägt eine dreistufige Einstellung vor, die nach
Entscheidung E1 entfällt. Der restriktive Weg ohne Einstellung: Abholzeit-
Änderung nur noch über eindeutige Auswahl aus den Betreuungsbausteinen (#2293),
freie Uhrzeit als dritte Option, Krankmeldung unverändert direkt.
**Entscheidung steht aus.**

**F2: Reihenfolge Backend und Frontend.** Der Tagesstatus (#2252) ist die
Voraussetzung für die Tageskarte. Vorschlag: Backend-Endpunkt zuerst, danach
die Oberfläche, damit die Karte nie eine Attrappe ist.

**F3: Umfang der Feedback-Entfernung.** #2326 entfernt Feedback aus allen drei
Portalen, dem Kiosk und dem Backend. In diesem Vorhaben entfernen wir nur den
Eltern-Anteil aus Navigation und Oberfläche; der vollständige Rückbau bleibt
#2326 vorbehalten.

---

## 12. Umsetzung in Etappen

| Etappe | Inhalt | Issues |
|---|---|---|
| 1 | Tagesstatus im Backend: Projektion, Endpunkt, Echtzeit, Tests | #2252 |
| 2 | Eltern-Hülle: eigene Navigation, Ablösung von Sidebar und Bottom-Nav | #2308 |
| 3 | Startseite: Zu erledigen, Tageskarten, Zero State | #2308, #2250 |
| 4 | Kinderprofil in vier Abschnitten, Entfernung Betreuungszeiten und AGs | #2302, #2303 |
| 5 | Formulare und Dialoge: Buttons, Pflichtfelder, eindeutige Abholzeit | #2293, #2304 |
| 6 | Familienkalender als eigene Elternansicht, Mehrsprachigkeit vervollständigt | — |
| 7 | Benachrichtigungen: Erstlogin-Dialog, Home-Bildschirm, E-Mail-Rückfall | #2305, #2306, #2307, #2297 |

Jede Etappe endet mit `pnpm run check`, Tests und Vorher/Nachher-Aufnahmen in
Mobile, Tablet und Desktop.

---

## 13. Randbedingungen aus den Projektregeln

- Neue Bausteine gehören nach `frontend/src/components/ui/`, nicht als
  Einzelstücke in den Eltern-Ordner (`.claude/rules/frontend-ui-kit.md`). Die
  größeren Touchflächen und die Statuskarte werden Kit-Bestandteile und in der
  PR begründet.
- Farben über `moto-*`-Utilities bzw. `LOCATION_COLORS`, nie als roher Hex-Wert
  in einer Utility-Klasse.
- Kalenderdaten als `timezone.Date` im Backend und `"YYYY-MM-DD"` im Frontend,
  niemals über `.toISOString()` abgeleitet.
- Handler bleiben dünn, Logik in Services, Datenzugriff in Repositories.
- Der In-App-Hilfe-Guide wird in derselben PR aktualisiert, sobald sich ein
  dokumentierter Ablauf ändert.
