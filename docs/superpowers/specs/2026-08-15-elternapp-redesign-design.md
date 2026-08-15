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
| #2292 | Kurzfristige Abmeldungen einschränken — **nicht in diesem Umfang**, siehe E5 |
| #2293 | Abholzeitpunkt eindeutig machen — **nicht in diesem Umfang**, siehe E5 |
| #2304 | Abholänderung braucht konkrete Uhrzeit und Pflichtgrund (bereits umgesetzt) |
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

### E6 bis E11: Produktentscheidungen vom 2026-08-15

| # | Entscheidung | Konsequenz |
|---|---|---|
| **E6** | **Optik: erkennbar moto, mit mehr bedeutungsvoller Farbe.** Präzisiert am selben Tag: Farbe erscheint als **Akzent** (farbige Kante, Icon-Feld, Statuspille), **nicht** als ganzflächige Einfärbung und **nie** als Verlauf. Verständlichkeit kommt aus Größe und Kontrast der Typografie, nicht aus Buntheit. | Wer die moto-Website kennt, erkennt die Eltern-App sofort. Es darf nicht nach KI-Generat aussehen. Einzelheiten in Abschnitt 7. |
| **E6b** | **Icons: `@phosphor-icons/react`**, wie auf der Website. Ersetzt die Festlegung vom 25.07.2026. Umfang: nur die Eltern-App. | Paket ist bereits installiert (`^2.1.10`). Personal- und Operator-Portal bleiben bei `lucide-react`. Kein `duotone`. |
| **E7** | **Anrede: "Sie".** | Bestehende Texte behalten die Anredeform, nur Wortwahl und Ton werden auf OGS-Sprache umgestellt. |
| **E8** | **Mobile Navigation: Start · Kinder · Nachrichten · Kalender · Mehr.** | Neuigkeiten liegen unter "Mehr" und tragen ihren Ungelesen-Zähler auf das "Mehr"-Symbol. Die Startseite bleibt der Ort, an dem offene Aushänge und Umfragen erscheinen. |
| **E9** | **Kind-zentriert mit Umschalter.** Bei einem Kind entfällt die Liste, die App zeigt direkt dieses Kind. Bei mehreren steht oben ein Umschalter mit Initialen. | Der Navigationspunkt heißt weiterhin "Kinder", führt bei einem Kind aber ohne Zwischenschritt auf dessen Seite. |
| **E10** | **Kalender wird eine eigene Elternansicht** als chronologische Terminliste mit Zu- und Absage in der Zeile. Monatsraster nur auf Tablet und Desktop als Zusatz. | `PersonalCalendar` wird im Elternportal nicht mehr verwendet. |
| **E11** | **PWA-Installation wird vorgezogen** (#2306, #2297) direkt hinter das Designsystem. | Ohne Home-Bildschirm-Installation gibt es auf iOS weder App-Charakter noch Push. |

### E12: Rückmeldungen einer Schule dürfen nicht für alle Schulen gelten

Das Feedback in Abschnitt 3 stammt von **einer** Schule. Das Produkt bedient
alle. Daraus folgen drei Regeln, die bei jeder Umsetzung gelten:

1. **Keine Behauptung über einen Mechanismus, den nicht jede Schule nutzt.**
   Texte dürfen nicht sagen, woher ein Wert stammt, wenn die Quelle
   konfigurierbar ist. Beispiel: "Die Bringzeit ergibt sich aus dem
   Stundenplan der Schule" war falsch, weil `timetable.enabled` abschaltbar ist
   und Zeiten auch von Hand oder aus der Anmeldung stammen können. Korrekt ist
   die Aussage über die **Zuständigkeit**: "Diese Zeiten pflegt die OGS."
   Abgesichert durch einen Test, der das Wort "Stundenplan" im Kinderbereich
   verbietet.
2. **Anzeige entfernen ist erlaubt, Fähigkeit entfernen nicht.** Betreuungszeiten
   (#2302) und AGs (#2303) sind aus der **Eltern-Oberfläche** verschwunden. Die
   Backend-Endpunkte, das Datenmodell und die Personal-Seite sind unangetastet.
   Meldet eine andere Schule, dass sie die Anzeige will, ist es eine
   Oberflächenfrage, keine Wiederherstellung verlorener Funktion.
3. **Was eine Schule abschalten kann, bleibt abgeschaltet sichtbar.** Alle
   Alltagsaktionen hängen weiter an `getChildFeatures`; Neuigkeiten und
   Essensplan hängen an ihren Gates. Keine Aktion wird angeboten, die das
   Backend mit 403 abweisen würde, und keine wird entfernt, nur weil eine
   Schule sie nicht mag.

**Was bewusst NICHT unter diese Regel fällt:** Der Pflichtgrund bei
Abholänderungen (#2304, bereits vor diesem Vorhaben umgesetzt) ändert das
Verhalten für alle Schulen. Er ist als allgemeine Verbesserung vertretbar, weil
jede OGS wissen muss, warum eine Abholzeit abweicht. Sollte eine Schule
widersprechen, ist das der Fall, in dem nach E1 erstmals eine Einstellung
gerechtfertigt wäre.

### E5: Der Abmelde- und Abhol-Ablauf bleibt fachlich unverändert

Was Eltern dürfen, ändert sich in diesem Vorhaben nicht. Krankmeldung bleibt
direkt, die Abholzeit-Änderung bleibt mit freier Uhrzeit und Pflichtgrund
bestehen (#2304 ist bereits umgesetzt). #2292 und #2293 werden hier nicht
angefasst; wir warten weiteres Feedback ab, bevor wir Eltern eine Funktion
wegnehmen, die nur eine Schule bemängelt hat.

Die **Bedienbarkeit** des Dialogs verbessern wir trotzdem: größere Flächen,
Alltagssprache, Fehler am Feld. Die Auswahl aus Betreuungsbausteinen aus #2293
kommt ausdrücklich nicht.

---

## 4a. Mandat für die Oberfläche: Neubau, keine Renovierung

**Dieser Abschnitt ist verbindlich und wird wortgleich in jeden Umsetzungsplan
dieses Vorhabens übernommen.** Wer einen Etappenplan ausführt, liest ihn dort,
nicht als Querverweis.

Für alles oberhalb der Datenschicht gilt ausdrücklich freie Hand. Die
Oberfläche der Eltern-App wird **von Grund auf neu gebaut**, nicht am Bestand
entlang verbessert. Maßstab ist eine gute Kita-Eltern-App, nicht das heutige
Elternportal.

### Was "von Grund auf" ausdrücklich erlaubt

- Bestehende Eltern-Komponenten **löschen und ersetzen**, statt sie zu
  erweitern. `child-detail.tsx`, `parent-dashboard.tsx`, `parent-page.tsx`,
  `child-care.tsx` und die Eltern-Zweige in Sidebar und Bottom-Nav sind
  Ausgangsmaterial, kein Bestandsschutz.
- **Neue Seitenstrukturen** erfinden, die es heute nicht gibt, wenn sie die
  Aufgabe besser abbilden.
- **Bestehende Texte komplett neu schreiben** statt sie zu glätten.
- Karten, Abstände, Typografie und Dichte neu festlegen, solange die Bausteine
  im geteilten UI-Kit landen.

### Die Leitlinien

- **Layout nach Kita-App-Mustern:** eine Sache pro Bildschirm, große Flächen,
  klare Reihenfolge, nichts Dekoratives. Kein Dashboard-Gefühl, keine
  Kachelwände, keine Marketing-Hero-Karten. Der wichtigste Inhalt steht ohne
  Scrollen da.
- **Keine neue Designsprache.** Verbindliche Vorlage ist `moto-nrw/website`,
  `src/app/globals.css` (Abschnitt 7). Farben, Typo-Stufen, Schatten, Easing
  und das Punkte-Muster werden von dort übernommen, nicht neu erfunden. Icons
  bleiben `lucide-react`, Breakpoints bleiben Tailwind-Standard.
- **Farbe trägt Bedeutung, nicht Schmuck.** Aus der moto-Palette, damit die App
  erkennbar zu moto gehört, aber deutlich beherzter eingesetzt als im
  Personal-Portal. Farbe nie als einziger Träger einer Information.
- **Der Punkte-Hintergrund wird zum Gestaltungsmittel**, nicht nur zum
  Seitengrund: maskiert auslaufend auf der Seite, als feine Textur in
  hervorgehobenen Karten.
- **Sprache ist OGS- und Kita-Sprache**, so wie Eltern und Team tatsächlich
  miteinander reden. "Ist Ihr Kind heute krank?" statt "Abwesenheitsmeldung
  erfassen". Keine Systembegriffe, keine Verwaltungswörter, keine Anglizismen.
  Das gilt für alle vier Sprachkataloge, nicht nur für Deutsch.
- **Mobile, Tablet und Desktop werden jeweils eigenständig entworfen**, nicht
  ein Entwurf dreimal gestreckt. Mobile ist der Leitfall, Desktop bekommt eine
  eigene Aufteilung statt einer breiten leeren Spalte.
- **Bedienbar mit wenig digitaler Erfahrung:** Icons immer mit Textlabel, große
  Touchflächen, Folgen vor dem Absenden benennen, nur eine Hauptaktion je
  Bildschirm.

### Der Qualitätsmaßstab: wie eine gute App aus dem App Store

Das Ziel ist die Bedienqualität einer professionellen Store-App, auf allen drei
Gerätearten. Weil "premium" als Anspruch nichts bewirkt, ist der Maßstab hier
als Prüfliste formuliert. Eine Etappe gilt erst als fertig, wenn ihre Punkte
erfüllt sind.

**Ehrliche Randbedingung:** Dies ist eine Web-App. Auf iOS entsteht der
App-Charakter erst nach Installation auf dem Home-Bildschirm (eigenes Icon,
kein Browser-Rahmen, Vollbild), und **nur installiert funktionieren dort Push-
Benachrichtigungen überhaupt**. Die Home-Bildschirm-Anleitung (#2306) ist
deshalb Voraussetzung für dieses Ziel, nicht Beiwerk.

#### Überall

- **Touchflächen** mindestens 44 pt (Apple HIG) bzw. 48 dp (Material). Wir
  nehmen 48 px als eine Zahl für alles.
- **Fließtext ab 17 px.** Keine Versalien-Mikrolabels, keine 11-px-Beschriftung.
- **Ein typografischer Maßstab** für die ganze App, keine Ad-hoc-Größen.
- **Icons immer mit Textlabel.** Ein Icon allein ist nie eine Schaltfläche.
- **Kein horizontaler Seiten-Scroll**, bei keiner Breite. Breite Inhalte
  scrollen in ihrem eigenen Container.
- **Kein Layout-Sprung**, wenn Daten eintreffen: Skelette haben die Form des
  Endzustands, keine ganzseitigen Spinner.
- **Rückmeldung auf jede Berührung** innerhalb von 100 ms über einen
  Aktiv-Zustand, nicht nur über Hover.
- **Leere Zustände** sagen, was passieren wird, und bieten genau eine Aktion an.
- **Bewegung nur, wenn sie etwas erklärt**, ausschließlich über Transform und
  Opacity, und `prefers-reduced-motion` wird respektiert.
- **Bedienbar bei 200 % Zoom und 320 px Breite**, vollständig per Tastatur, mit
  sichtbarem Fokusring.

#### Mobile, der Leitfall

- **Safe Areas** werden respektiert: Notch oben, Home-Indikator unten, über
  `env(safe-area-inset-*)`. Nichts klebt unter der Systemleiste.
- **Bottom-Navigation** fest, in Daumenreichweite, höchstens fünf Ziele, Icon
  und Label, unmissverständlicher Aktiv-Zustand.
- **Hauptaktionen unten**, nicht oben: der Daumen erreicht den unteren Rand,
  nicht die obere Ecke.
- **Dialoge kommen als Sheet von unten**, schließbar über Hintergrund und
  Wischgeste, mit angehefteter Aktionsleiste.
- **Kein Verhalten, das Hover voraussetzt.** Auf Touch gibt es keinen Hover.
- **Installierbar** als PWA mit eigenem Icon und Startbildschirm.

#### Tablet

- **Kein gestrecktes Handy.** Wo zwei Spalten fachlich Sinn ergeben (Liste und
  Detail), gibt es zwei Spalten.
- **Im Querformat** tritt eine Seitennavigation an die Stelle der
  Bottom-Navigation.
- **Dialoge** werden zu mittig stehenden Fenstern statt Vollbild-Sheets.

#### Desktop

- **Dauerhafte Seitennavigation**, kein ausklappbares Menü für Alltagsziele.
- **Begrenzte Zeilenlänge** für Fließtext, keine 1400 px breiten Absätze.
- **Keine leeren Bildschirmhälften.** Wenn eine Spalte nichts trägt, ist die
  Aufteilung falsch.
- **Hover- und Fokuszustände** für jedes bedienbare Element.

### Die einzigen Grenzen

Die Projektregeln aus Abschnitt 13: neue Bausteine gehören ins geteilte UI-Kit
(`frontend/src/components/ui/`), Farben kommen aus der moto-Palette über
`moto-*`-Utilities, Kalenderdaten sind `timezone.Date` bzw. `"YYYY-MM-DD"`,
`pnpm run check` läuft ohne Warnung durch, und jede sichtbare Änderung wird mit
Vorher/Nachher-Aufnahmen in Mobile, Tablet und Desktop belegt.

Innerhalb dieser Grenzen ist alles verhandelbar. Im Zweifel gilt: die für
Eltern verständlichere Lösung schlägt die zum Bestand ähnlichere.

## 5. Zielbild: Informationsarchitektur

### Navigation

Nach Entscheidung E8. Mobile Bottom-Nav und Desktop-Sidebar zeigen dieselben
Ziele.

| Ziel | Inhalt |
|---|---|
| **Start** | Zu erledigen, darunter je Kind eine Tageskarte mit den drei Hauptaktionen |
| **Kinder** | Bei einem Kind direkt dessen Seite (E9), bei mehreren ein Umschalter. Heute, gebuchte Betreuung, Daten, Kontakte |
| **Nachrichten** | Verlauf mit der OGS, Zähler für Ungelesenes |
| **Kalender** | Termine als chronologische Liste, Zu- und Absage in der Zeile (E10) |
| **Mehr** | Neuigkeiten, Essensplan, Benachrichtigungen, Sprache, Neue Anmeldung, Abmelden |

Neuigkeiten liegen hinter "Mehr", **tragen ihren Ungelesen-Zähler aber auf das
"Mehr"-Symbol**, damit ein ungelesener Aushang sichtbar bleibt. Zusätzlich
erscheint jeder ungelesene Aushang und jede offene Umfrage im Bereich
"Zu erledigen" auf der Startseite. Ein Elternteil muss also nirgendwo suchen,
auch wenn der Eintrag selbst nicht in der Hauptnavigation steht.

Anmeldung, Benachrichtigungen und Sprache sind Einmalvorgänge und gehören
ohnehin nicht in die Hauptnavigation.

**Entfällt vollständig aus der Navigation:** Betreuungszeiten (#2302), AGs und
Gruppen (#2303), Produktfeedback (#2326), "Bald im Elternportal".

### Startseite

```
┌──────────────────────────────────────────┐
│  Guten Morgen, Sabine            28px/700│  eine Zeile, kein Kicker,
│                                          │  keine Willkommenskarte
│  Zu erledigen                    20px/600│
│  ┌────────────────────────────────────┐  │  nur Dinge mit offener Handlung:
│  │ 📋 Umfrage: Sommerfest          ›  │  │  ungelesene Aushänge,
│  │ 💬 Neue Nachricht der OGS       ›  │  │  offene Umfragen,
│  │ 📅 Elternabend 01.09.           ›  │  │  Termineinladungen ohne Antwort,
│  └────────────────────────────────────┘  │  ungelesene Nachrichten
│      ↑ feine Punkt-Textur als Fläche     │  ganze Zeile anklickbar, ≥72px
│                                          │
│  ┌────────────────────────────────────┐  │
│  │▌ FS  Felix Schneider               │  │  ▌ 4px farbige Kante,
│  │▌     Klasse 1a                     │  │     Fläche bleibt weiß
│  │▌                                   │  │
│  │▌  ✓  In der OGS          24px/800  │  │  Ebene 1 aus at_ogs
│  │▌     Seit 12:38 Uhr da     15px    │  │  Ebene 2 aus state
│  │                                    │  │
│  │  [ Krank melden      ]             │  │  je 48px, volle Breite mobil,
│  │  [ Abholung ändern   ]             │  │  ab sm nebeneinander
│  │  [ OGS schreiben     ]             │  │
│  └────────────────────────────────────┘  │
└──────────────────────────────────────────┘
```

Ist nichts offen, zeigt "Zu erledigen" einen ruhigen Zustand mit grünem Haken
("Alles erledigt", darunter "Es gibt gerade nichts zu tun."), keine leere Liste
und keine Platzhalterkarten. Ein leerer Neuigkeitenbereich verdrängt nie die
Tageskarten (#2308).

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

### Zweistufig, nicht siebenstufig

Die Anzeige beantwortet zuerst **eine** Frage, und zwar in einer Sekunde
erfassbar: **Ist mein Kind in der OGS?** Das ist eine Ja/Nein-Aussage, groß und
farbig. Erst darunter steht eine erklärende Zeile, die sagt, warum bzw. wann.

```
┌────────────────────────────────┐
│  ●  In der OGS                 │   Ebene 1: binär, groß, farbig
│     Seit 12:38 Uhr da          │   Ebene 2: erklärt Zeitpunkt und Plan
└────────────────────────────────┘
```

Sieben Zustände bleiben fachlich bestehen, sie sind aber die zweite Ebene. Wer
nur hinschaut, sieht "In der OGS" oder "Nicht in der OGS". Wer liest, erfährt
den Rest.

### Zustände

| Zustand | Ebene 1 | Ebene 2 | Herleitung | Farbe |
|---|---|---|---|---|
| `present` | **In der OGS** | "Seit 12:38 Uhr da" | offene Anwesenheit (`check_out_time IS NULL`) | Grün |
| `left` | Nicht in der OGS | "Um 15:12 Uhr nach Hause gegangen" | Anwesenheit heute vorhanden und geschlossen | Grau |
| `expected` | Nicht in der OGS | "Kommt heute um 12:30 Uhr" | Betreuungstag, erwartete Zeit noch nicht erreicht | Blau |
| `not_arrived` | Nicht in der OGS | "Wird seit 12:30 Uhr erwartet" | erwartete Zeit überschritten, keine Anwesenheit | Blau |
| `absent` | Nicht in der OGS | "Heute abgemeldet" | wirksamer Eintrag in `active.student_status_days` | Rot |
| `no_care` | Nicht in der OGS | "Heute keine Betreuung" | kein Betreuungstag laut Betreuungsplan | Grau |
| `unknown` | *(keine Aussage)* | "Status derzeit nicht verfügbar" | Daten nicht belastbar oder Schule pflegt keine Anwesenheit | Grau |

`unknown` ist der einzige Zustand **ohne** Ebene-1-Aussage. Eine Ja/Nein-Aussage
zu treffen, die wir nicht belegen können, wäre schlimmer als zu schweigen.

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

### Quelle der Designsprache: das Website-Repo, sonst nichts

**`moto-nrw/website` ist die einzige verbindliche Quelle** für Farben, Maße,
Effekte, Icons und Assets. Konkret:

| Was | Wo |
|---|---|
| Farben, Typo-Stufen, Radien, Schatten, Easing, Breakpoint-Werte | `src/app/globals.css`, Block `@theme inline` |
| Flächen, Punkte-Muster, Eyebrow, Fokus | `src/app/globals.css`, Block `@layer components` und die Basisregeln |
| Logo, Wortmarke, Favicon | `public/moto_transparent.webp`, `public/moto-logo-wordmark.webp`, `public/favicon-v2.png` |
| PWA-Vorlage | `public/site.webmanifest` |
| Icons | `@phosphor-icons/react` |

**Das npm-Paket `@moto-nrw/design-system` wird vollständig ignoriert.**
Entscheidung vom 2026-08-15. Es ist keine Gestaltungsvorgabe, weder für Farben
noch für Maße noch für Komponenten. Sein `@theme` liefert eine
`steel`/`sage`/`warm`-Palette mit einem fremden Grün (`#7BA05B`), das im
Produkt nirgends vorkommt.

Praktische Folgen:

- Kein Wert der Eltern-App wird aus dem Paket bezogen. Jeder Token, den die
  Eltern-App braucht, steht entweder schon in `frontend/src/styles/globals.css`
  oder wird dort aus der Website-CSS ergänzt.
- `bg-sage-*`, `bg-steel-*`, `--color-brand-primary` und Komponenten aus dem
  Paket werden nicht verwendet. Das entspricht der bestehenden Regel in
  `.claude/rules/frontend-ui-kit.md`.
- Die App importiert `@moto-nrw/design-system/tailwind` heute noch in
  `globals.css`. Diesen Import zu entfernen ist eine eigene Aufräumaufgabe mit
  Blast Radius ins Personal-Portal und **nicht Teil dieses Vorhabens**; die
  Eltern-App darf sich nur nicht darauf stützen.

*Randnotiz zur Versionslage:* project-phoenix liegt auf `^0.5.2`, die Website
auf `0.2.2`. "Veraltet" meint also nicht die Versionsnummer, sondern dass das
Paket als Autorität nicht gilt. Maßgeblich ist ausschließlich die Website.

#### Bekannte Abweichungen, die anzugleichen sind

Die App spiegelt die Website heute größtenteils schon (ihr eigener Kommentar in
`frontend/src/styles/globals.css` sagt das ausdrücklich). Blau, Grün und Orange
stimmen in allen Abstufungen überein. Es fehlen bzw. weichen ab:

| Token | Website | project-phoenix | Maßnahme |
|---|---|---|---|
| Rot | `#DC3545` | `#DC2626` | **Echter Konflikt.** Die Eltern-App übernimmt den Website-Wert. Eine app-weite Angleichung berührt das Personal-Portal und wird getrennt vorgeschlagen, nicht nebenbei gemacht. |
| Rot dunkel | `#D42220` | `#B91C1C` | wie oben |
| Grau 150 | `#EEF0F3` | fehlt | ergänzen |
| Dunkel | `#030712` | fehlt als Token | ergänzen |

#### Farben, wörtlich aus der Website

| Rolle | Token | Hex |
|---|---|---|
| Primär (Blau) | `--color-primary` | `#5080D8` |
| Primär dunkel / hell | `--color-primary-dark` / `-light` | `#3B68C0` / `#6B95E0` |
| Akzent (Grün) | `--color-accent` | `#83CD2D` |
| Akzent dunkel / heller / dunkler | `--color-accent-dark` / `-light` / `-darker` | `#74B825` / `#92D63C` / `#6DB118` |
| Orange | `--color-orange` / `-dark` | `#F78C10` / `#E07400` |
| Rot | `--color-red` / `-dark` / `-light` | `#DC3545` / `#D42220` / `#FF3130` |
| Dunkel | `--color-dark` / `-secondary` | `#030712` / `#111827` |
| Grau 150 (Zwischenton) | `--color-gray-150` | `#EEF0F3` |

`LOCATION_COLORS` in der App bleibt die Quelle für **fachliche** Semantik
(Raum, Status, Ort). Die Website-Tokens liefern die **gestalterischen**
Abstufungen, die dort fehlen: hellere und dunklere Varianten für getönte
Flächen, Hover- und Aktivzustände.

#### Typografie, wörtlich aus der Website

12 · 14 · 16 · 18 · 20 · 24 · 28 · 36 · 42 px. Gewichte 400 / 500 / 600 / 700 /
800. Zeilenhöhen 1.2 (eng), 1.5 (normal), 1.6 (entspannt).

Für die Eltern-App gilt daraus: Fließtext 16-18 px statt 14, Statuszeile 20 px,
Seitentitel 24-28 px. Die Website nutzt 800 als stärkstes Gewicht; das
übernehmen wir für die eine große Statusaussage.

#### Schatten und Bewegung, wörtlich aus der Website

- `--shadow-sm: 0 1px 2px rgba(3,7,18,0.06)` für Flächen
- `--shadow-md: 0 8px 24px rgba(3,7,18,0.08)` für angehobene Karten
- `--shadow-card: 0 10px 30px rgba(0,0,0,0.08)` und `--shadow-card-hover`
- `--shadow-success: 0 2px 8px rgba(131,205,45,0.2)` für den Grün-Zustand
- Haus-Easing: `cubic-bezier(0.22, 1, 0.36, 1)`, Dauern 240-680 ms
- Fokus: 2 px `--color-primary` Outline, 2 px Offset, dazu
  `0 0 0 4px rgba(80,128,216,0.2)`

#### Der Punkte-Hintergrund: mehr damit spielen

Beide Projekte haben ihn bereits im 14-px-Raster. Die Website geht weiter und
setzt ihn an drei Stellen unterschiedlich ein:

1. **Als Seitengrund** (`.moto-dot-field`, `rgba(156,163,175,0.42)`, 1.15 px,
   Deckkraft 0.58) — das hat die App als `.moto-dotted-background` schon.
2. **Maskiert**, damit er zum Rand hin ausläuft statt hart zu enden:
   `--center` (radial), `--top` (nach unten), `--start-panel` (nach beiden
   Seiten). Die App hat Entsprechungen, nutzt sie im Elternportal aber nicht.
3. **Als Textur *innerhalb* von Flächen** (`.navigation-menu-featured`,
   `.product-system-header`: feinere Punkte mit `rgba(156,163,175,0.16-0.32)`
   bei 0.85-0.9 px). **Das fehlt der App vollständig** und ist der größte
   ungenutzte Hebel: hervorgehobene Karten bekommen eine spürbare Textur,
   statt nur weiß auf grau zu sein.

Für die Eltern-App heißt das konkret: Der Bereich "Zu erledigen" und die
Kinder-Tageskarte bekommen die feine Punkt-Textur als Flächenmerkmal, der
Seitengrund bleibt maskiert und läuft weich aus.

#### Icons: Phosphor, wie auf der Website

Die Eltern-App nutzt **`@phosphor-icons/react`**, dieselbe Bibliothek wie die
Website. Das Paket ist in `frontend/package.json` bereits vorhanden (`^2.1.10`,
identisch zur Website), es kommt also keine Abhängigkeit hinzu.

Das ersetzt die frühere Festlegung vom 25.07.2026, mit der eine
Phosphor-Migration verworfen wurde. Umfang jetzt: **nur die Eltern-App.** Das
Personal- und Operator-Portal bleiben bei `lucide-react`; sie flächendeckend
umzustellen wäre ein eigenes Vorhaben mit eigenem Nutzen-Nachweis.

Gewicht: `regular` als Standard, `fill` für aktive Navigationspunkte und den
Anwesend-Zustand. **Kein `duotone`** — der Look wurde bereits einmal abgelehnt.

#### Was wir NICHT von der Website übernehmen

- **Breakpoints.** Die Website nutzt 480/576/768/900/1200/1440. Die App bleibt
  bei den Tailwind-Standardwerten, sonst verschieben sich alle bestehenden
  Layouts im Personal-Portal.
- **Die Kalam-Schrift** aus dem Website-Header. Marketing-Handschrift gehört
  nicht in ein Betreuungswerkzeug.
- **`@tabler/icons-react`.** Eine Icon-Bibliothek genügt.

### Zurückhaltung: es darf nicht nach KI aussehen

Die Eltern-App muss für jemanden, der die moto-Website kennt, sofort als moto
erkennbar sein. Das erreicht man mit denselben Bauteilen, nicht mit mehr
Effekten. Ausdrücklich verboten:

- **Ganzflächig eingefärbte Container.** Eine Karte wird nicht orange oder grün
  ausgefüllt. Farbe erscheint als **Akzent**: farbige Kante links, farbiges
  Icon-Feld, kleine getönte Statuspille. Die Fläche selbst bleibt weiß.
- **Verläufe.** Keine Gradienten auf Karten, Kacheln oder Schaltflächen. Die
  Website hat keine, die App bekommt keine.
- **Bunte Farbe ohne Bedeutung.** Jede Farbe steht für einen Zustand. Wo kein
  Zustand ist, ist keine Farbe.
- **Dekoratives Glühen, Neon, Glasmorphismus, übergroße Emoji.** Nichts davon
  existiert auf der Website.

"Etwas mehr Farbe" heißt: mehr Stellen tragen eine **bedeutungsvolle** Farbe
als heute, nicht dass die Flächen bunt werden.

### Größe und Kontrast statt Buntheit

Die Verständlichkeit soll aus der Typografie kommen, nicht aus Farbe:

- **Deutlichere Sprünge zwischen den Ebenen.** Heute liegen Überschrift und
  Fließtext oft nur eine Stufe auseinander. Künftig: Seitentitel 28 px / 700,
  Abschnittstitel 20 px / 600, Fließtext 17 px / 400, Sekundärtext 15 px. Der
  Statuswert des Kindes 24 px / 800, die stärkste Stelle der ganzen App.
- **Alles eine Stufe größer** als im Personal-Portal.
- **Keine Versalien-Mikrolabels.** Die heutigen 11-px-Beschriftungen in
  Großbuchstaben entfallen ersatzlos; ihre Information steht im Klartext.

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
- **Kein rohes natives `--:--`-Zeitfeld** mehr. Die Uhrzeit bleibt frei
  eingebbar (E5), wird aber über das Kit-Zeitfeld mit 48 px Höhe und sichtbarem
  Format erfasst. Datumsangaben über den Kit-Datumswähler mit Schnellwahl
  "Heute" und "Morgen".
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

### Die zwei Regeln, die überall gelten

Korrigiert am 2026-08-15, nachdem die frühere Vorgabe (Desktop zweispaltig mit
unterschiedlich breiten Blöcken) in der laufenden Anwendung schlecht aussah.

1. **Abschnitte stehen untereinander und nehmen die volle Inhaltsbreite ein.**
   Auf jeder Breite, auf jeder Seite. Ein Abschnitt, der nur die halbe Breite
   füllt, während der darüber die ganze nimmt, liest sich als Fehler.
2. **Nebeneinander nur bei gleicher Breite.** Zwei Blöcke dürfen sich eine
   Zeile teilen, aber dann exakt hälftig. Ungleiche Spaltenbreiten sind
   verboten.

Innerhalb eines Abschnitts gilt das nicht: Kinderkarten, Wochentagskarten und
Terminzeilen liegen in einem `auto-fit`-Raster und füllen die Breite
gemeinsam.

| Breite | Muster |
|---|---|
| ab 320 px | eine Spalte, Bottom-Nav mit fünf Zielen, Aktionen volle Breite |
| ab 640 px (Tablet hoch) | Abschnitte weiter untereinander, volle Breite; innerhalb der Abschnitte füllen Karten das Raster |
| ab 1024 px (Tablet quer, Desktop) | Seitennavigation links, Inhalt **einspaltig**: Abschnitte untereinander über die volle Inhaltsbreite |
| ab 1440 px | Inhaltsbreite begrenzt und zentriert, damit Zeilen nicht überlang werden; Abschnitte füllen diese Breite weiterhin ganz |

Auf der Startseite heißt das konkret: "Zu erledigen" und die Kinderkarten
stehen **übereinander**, nicht nebeneinander.

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

**F1 (#2292/#2293): entschieden am 2026-08-15.** Der Ablauf bleibt fachlich
unverändert, siehe E5.

**F2: Reihenfolge, entschieden am 2026-08-15.** Backend zuerst: der
Tagesstatus (#2252) entsteht vollständig, bevor die Oberfläche darauf aufsetzt.
Damit ist die Tageskarte nie eine Attrappe.

**F3: Umfang der Feedback-Entfernung.** #2326 entfernt Feedback aus allen drei
Portalen, dem Kiosk und dem Backend. In diesem Vorhaben entfernen wir nur den
Eltern-Anteil aus Navigation und Oberfläche; der vollständige Rückbau bleibt
#2326 vorbehalten.

---

## 12. Umsetzung in Etappen

Vier Etappen mit je einem eigenen Umsetzungsplan unter
`docs/superpowers/plans/`. Die ursprünglich sieben Etappen wurden
zusammengefasst, weil Formulare, Kalender und Mehrsprachigkeit sich nicht
sinnvoll von den Seiten trennen lassen, an denen sie hängen.

| Etappe | Inhalt | Plan | Issues |
|---|---|---|---|
| 1 | Tagesstatus im Backend: Projektion, Endpunkt, Echtzeit, Anzeige in der bestehenden Ansicht | `2026-08-15-eltern-tagesstatus-backend.md` | #2252 |
| 2 | Eltern-Hülle: Designgrundlage, Phosphor-Icons, eigene Navigation, Ablösung von Sidebar und Bottom-Nav | `2026-08-15-etappe2-eltern-huelle.md` | #2308 |
| 3 | Die Seiten: Startseite, Kinderbereich, Nachrichten, Kalender, Neuigkeiten, Dialoge, Entfernungen | `2026-08-15-etappe3-eltern-seiten.md` | #2308, #2250, #2302, #2303, Teil von #2326 |
| 4 | Installierbarkeit und Benachrichtigungen | `2026-08-15-etappe4-pwa-benachrichtigungen.md` | #2306, #2297, #2305, #2307 |

**Reihenfolge:** Etappe 1 und 2 laufen parallel (Backend und Frontend berühren
sich nicht). Etappe 3 setzt auf beiden auf. Etappe 4 hängt nur an Etappe 2 und
kann daneben laufen.

Jede Etappe endet mit `pnpm run check`, Tests und Vorher/Nachher-Aufnahmen in
Mobile, Tablet und Desktop.

### Entscheidungen während der Umsetzung

Nach Vorgabe vom 2026-08-15 werden Zweifelsfälle unterwegs nach bestem Urteil
entschieden und hier protokolliert, statt die Umsetzung anzuhalten.

| Datum | Entscheidung | Begründung |
|---|---|---|
| 2026-08-15 | Sieben Etappen auf vier zusammengefasst | Formulare, Kalender und Mehrsprachigkeit hängen an den Seiten und lassen sich nicht getrennt liefern, ohne dieselben Dateien zweimal anzufassen. |
| 2026-08-15 | `pickup_today` und `pickup_changed` **nicht** im Tagesstatus-Endpunkt | Gehören nicht zu #2252 und werden im Frontend bereits aus vorhandenen Betreuungsdaten abgeleitet. Eine zweite Ableitung derselben Information wäre eine Fehlerquelle. |
| 2026-08-15 | Erkennung "Schule pflegt keine Anwesenheit" über einen 14-Tage-Rückblick auf die Anwesenheit des Kindes | Braucht keine neue Repository-Methode und ist ehrlich: ein Kind ohne jede Anwesenheit in 14 Tagen liefert `unknown` statt eines beunruhigenden `not_arrived`. |
| 2026-08-15 | Phosphor nur in der Eltern-App, Personal- und Operator-Portal bleiben bei lucide | Eine flächendeckende Umstellung wäre ein eigenes Vorhaben mit eigenem Nutzen-Nachweis und war 2026-07 bereits einmal abgelehnt. |
| 2026-08-15 | Website-Rot `#DC3545` nur in der Eltern-App, `--color-moto-red` bleibt für das Personal-Portal | Eine app-weite Farbangleichung berührt jede Fläche im Personal-Portal und gehört nicht nebenbei in dieses Vorhaben. |
| 2026-08-15 | Eigenes Eltern-Manifest statt des geteilten `public/site.webmanifest` | Eltern installieren "moto Eltern" mit eigenem Startpunkt, nicht das generische "MOTO" mit `start_url: "/"`. |
| 2026-08-15 | Ein unlesbarer Betreuungsplan entwertet die Anwesenheit nicht mehr (`CareDayResolved`) | Beim Prüfen gegen die laufende Anwendung fiel auf: an einem Samstag antwortet `GetStudentArrivalScheduleForWeekday` mit `invalid weekday`, der Fehler brach die ganze Transaktion ab und verwarf die bereits geladene Anwesenheit. Wer nachweislich da ist, ist da, auch wenn sein Plan nicht lesbar ist. Ohne gelesenen Plan gilt `unknown` statt `no_care`. |
| 2026-08-15 | Wochenenden fragen den Wochenplan gar nicht erst ab | Der Plan kennt nur Montag bis Freitag. Eine Ferienbetreuung am Wochenende fällt trotzdem nicht durchs Raster, weil eine vorhandene Anwesenheit vor dem Betreuungstag geprüft wird. |
| 2026-08-15 | Das Eltern-Manifest entsteht in `app/manifest.ts` per Host-Erkennung, nicht als eigene Route unter `/parents` | Der Proxy bildet den Eltern-Host auf `/parents/*` ab; öffentliche URLs beginnen dort ohne `/parents`. Eine Route unter `/parents/manifest.webmanifest` war nur über eine 307-Umleitung erreichbar, und hinter einer Umleitung ist eine App nicht installierbar. Verifiziert: der Eltern-Host liefert "moto Eltern", der Tenant-Host unverändert "MOTO". |
| 2026-08-15 | Der Eltern-Host wird über `isParentsHost()` erkannt, nicht über die Favicon-Variante | Beinahe-Fehler: Die Favicon-Variante ist auf dem Eltern-Host absichtlich `"normal"` (Commit 23a2650e8, "use production favicon for parents app"), `"eltern"` bleibt Staging vorbehalten. Sie als Stellvertreter für den Host zu nehmen hätte diese Entscheidung stillschweigend rückgängig gemacht. |
| 2026-08-15 | Sorgeberechtigte werden über den vorhandenen `parentmessaging.Emitter` geweckt, nicht über einen neuen Pfad | Der Ereignistyp `parent_child_updated` und die Zustellung an Sorgeberechtigte existierten bereits; `services/active` bekommt nur ein schmales `GuardianWaker`-Interface, damit es nichts über das Eltern-Messaging wissen muss. |

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
