# Etappe 3: Die Seiten der Eltern-App

> **Für agentische Bearbeiter:** ERFORDERLICHE SUB-SKILL: `superpowers:subagent-driven-development` oder `superpowers:executing-plans`.

**Ziel:** Startseite, Kinderbereich, Nachrichten, Kalender und Neuigkeiten werden in Elternsprache neu aufgebaut, auf der Hülle aus Etappe 2 und dem Tagesstatus aus Etappe 1.

**Voraussetzung:** Etappe 1 (`GET /parent/me/children/{studentId}/today`) und Etappe 2 (`ParentShell`, Phosphor-Icons, `Button size="touch"`, Punkt-Textur) sind fertig und gemerged. Binde dich an die Namen, die dort **tatsächlich** entstanden sind; dieser Plan nennt Rollen, keine geratenen Bezeichner.

**Umsetzt:** #2308, #2250, #2302, #2303, Teilmenge von #2326.

---

## Mandat für die Oberfläche: Neubau, keine Renovierung

Übernommen aus Abschnitt 4a der Spezifikation. Verbindlich.

Für alles oberhalb der Datenschicht gilt freie Hand. Die Oberfläche wird **von Grund auf neu gebaut**, nicht am Bestand entlang verbessert. Maßstab ist eine gute Kita-Eltern-App.

**Erlaubt:** bestehende Eltern-Komponenten löschen und ersetzen; neue Seitenstrukturen erfinden; Texte komplett neu schreiben; Karten, Abstände, Typografie und Dichte neu festlegen.

**Leitlinien:**

- **Keine neue Designsprache. Einzige Quelle ist `moto-nrw/website`** (lokal `/Users/flo/Developer/moto/website`), `src/app/globals.css`, Blöcke `@theme inline` und `@layer components`, plus `public/`. Vor jeder Farb-, Typo-, Schatten- oder Effektentscheidung dort nachlesen.
- **Das Paket `@moto-nrw/design-system` wird vollständig ignoriert.**
- **Icons: `@phosphor-icons/react`**, Gewicht `regular`, `fill` für aktive Zustände, **kein `duotone`**, ausschließlich über das Icon-Modul aus Etappe 2.
- **Es darf nicht nach KI aussehen.** Keine ganzflächig eingefärbten Container, keine Verläufe, kein Glühen, kein Glasmorphismus, keine übergroßen Emoji. Farbe als Akzent (farbige Kante links, Icon-Feld, kleine Statuspille), die Kartenfläche bleibt weiß. Jede Farbe steht für einen Zustand.
- **Verständlichkeit aus Größe und Kontrast**, nicht aus Buntheit.
- **Sprache ist OGS- und Kita-Sprache**, Anrede "Sie". Alle vier Kataloge.
- **Mobile ist der Leitfall.**

**Grenzen:** neue Primitive ins geteilte UI-Kit, Kalenderdaten als `"YYYY-MM-DD"` (nie über `.toISOString()`), `pnpm run check` ohne Warnung, jede sichtbare Änderung mit Aufnahmen belegt.

---

## Globale Randbedingungen

**Typografische Leiter:** Statuswert 24/800 · Seitentitel 28/700 · Abschnittstitel 20/600 · Fließtext 17/400 · Sekundärtext 15/400. Nichts unter 15 px, keine Versalien-Mikrolabels.

**Farben:** Grün `#83CD2D` (da, erledigt) · Blau `#5080D8` (Information, erwartet) · Orange `#F78C10` (offene Handlung) · Rot `#DC3545` (krank, abgemeldet) · Grau `#6B7280` (neutral).

**Zustandsdarstellung:** Farbe nie allein. Jeder Zustand hat Icon **und** Text.

**Bestehende Tests nie ändern**, um neuen Code grün zu bekommen. Wenn ein Test bricht, halte an und berichte den Konflikt.

---

## Die Sprache: verbindliche Wortliste

Diese Tabelle ist Vorgabe, nicht Vorschlag. Deutsch führend, die drei anderen Kataloge sinngemäß.

| Statt | Künftig |
|---|---|
| Elternportal / Dashboard | (entfällt, die App braucht keinen Namen auf jeder Seite) |
| Stammdaten | Daten von {Name} |
| Betreuungsangebote | Gebuchte Betreuung |
| Betreuungszeiten | (entfällt vollständig, #2302) |
| AGs und Gruppen | (entfällt vollständig, #2303) |
| Tagesmeldung | Heute |
| Abwesenheitsmeldung erfassen | Krank melden |
| Grund (Pflichtfeld) | Warum? * |
| Keine Abmeldung | {Name} wird heute betreut |
| Care Exception / Abholzeit-Ausnahme | Abholung ändern |
| Sorgeberechtigte | Eltern und Abholberechtigte |
| Verknüpfte Konten | Wer hat noch Zugang? |
| Änderungsanfrage | Ihre Anfrage an die OGS |
| Speichern (im Krank-Dialog) | Krankmeldung senden |
| Speichern (im Abhol-Dialog) | Abholung ändern |
| Neuigkeiten | Aus der OGS |
| Anmeldung / Enrollment | Neue Anmeldung |

**Folgen vor dem Absenden benennen.** Jeder Dialog sagt in einem Satz, was passiert: "Die OGS wird sofort informiert." Kein stummes Speichern.

---

## Dateiübersicht

| Datei | Verantwortung |
|---|---|
| `components/parent/start/parent-start-page.tsx` | Startseite |
| `components/parent/start/todo-list.tsx` | Bereich "Zu erledigen" |
| `components/parent/child/child-day-card.tsx` | Tageskarte je Kind, zweistufiger Status |
| `components/parent/child/child-switcher.tsx` | Umschalter bei mehreren Kindern |
| `components/parent/child/child-page.tsx` | Kinderseite mit vier Abschnitten |
| `components/parent/calendar/parent-calendar-page.tsx` | Terminliste |
| `components/parent/news/parent-news-page.tsx` | Aus der OGS |
| `components/parent/more/more-sheet.tsx` | "Mehr"-Sheet |
| Zu löschen | `parent-dashboard.tsx`, `parent-children-page.tsx`, `child-detail.tsx`, `child-detail-section.tsx`, `child-care-schedule.tsx`, `child-care-offerings.tsx` (nur der AG-Teil), `care-schedule-request-modal.tsx` |

---

### Aufgabe 1: Tageskarte des Kindes

**Das Herzstück der App.** Wird auf der Startseite und im Kinderbereich verwendet.

**Aufbau, zweistufig nach Spezifikation Abschnitt 6:**

```
┌────────────────────────────────────┐
│▌ FS   Felix Schneider              │  ▌ = 4px farbige Kante,
│▌      Klasse 1a                    │      Farbe = Zustandsfarbe
│▌                                   │
│▌  ✓  In der OGS            24px/800│  Ebene 1 aus at_ogs
│▌     Seit 12:38 Uhr da     15px    │  Ebene 2 aus state
│                                    │
│  [ Krank melden ]                  │  je 48px, volle Breite
│  [ Abholung ändern ]               │  mobil untereinander,
│  [ OGS schreiben ]                 │  ab sm nebeneinander
└────────────────────────────────────┘
```

- Fläche **weiß**, Radius 24 px, Rand `#E5E7EB`, Schatten `0 1px 2px rgba(3,7,18,0.06)`.
- Die Farbe erscheint **nur** an der linken Kante und im Icon-Feld. Keine Flächenfüllung.
- Ebene 1 kommt ausschließlich aus `at_ogs` des Endpunkts, nie aus einer eigenen Ableitung aus `state`. Ist `at_ogs` `null`, entfällt Ebene 1 und nur die Ebene-2-Zeile steht da.
- Aktionen erscheinen nur, wenn die Schule sie erlaubt (`getChildFeatures`).

**Texte Ebene 1:** "In der OGS" / "Nicht in der OGS".
**Texte Ebene 2:** "Seit {time} Uhr da" · "Um {time} Uhr nach Hause gegangen" · "Kommt heute um {time} Uhr" · "Wird seit {time} Uhr erwartet" · "Heute abgemeldet" · "Heute keine Betreuung" · "Status derzeit nicht verfügbar".

- [ ] Test zuerst: alle sieben Zustände, Ebene 1 korrekt bzw. abwesend bei `null`, Icon und Text je vorhanden, Aktionen nur bei erlaubter Funktion.
- [ ] Fehlschlag bestätigen, umsetzen, Erfolg bestätigen, committen.

---

### Aufgabe 2: Startseite

**Aufbau von oben nach unten:**

1. **Begrüßung**, eine Zeile, 28/700: "Guten Morgen, {Vorname}" / "Guten Tag" / "Guten Abend" je nach Berliner Uhrzeit. Kein Kicker, keine Beschreibungszeile, keine Willkommenskarte.
2. **Zu erledigen**, nur wenn es etwas gibt. Enthält ungelesene Nachrichten, ungelesene Aushänge, offene Umfragen und Termineinladungen ohne Antwort. Jeder Eintrag: Icon-Feld in der Zustandsfarbe, Titel 17/600, Kontextzeile 15, rechts ein Pfeil, ganze Zeile anklickbar, mindestens 72 px hoch. Der Bereich trägt die feine Punkt-Textur (`moto-dot-texture--soft`) als Flächenmerkmal.
3. **Ist nichts offen:** ein ruhiger Zustand mit grünem Haken, "Alles erledigt", darunter 15 px "Es gibt gerade nichts zu tun." Keine leere Liste, keine Platzhalterkarten.
4. **Tageskarte je Kind** aus Aufgabe 1.

**Ausdrücklich nicht auf der Startseite:** "Neue Anmeldung" als Hauptaktion (#2308), leere Neuigkeitenbereiche, die alte Willkommenskarte.

- [ ] Test zuerst: Begrüßung nach Tageszeit; "Zu erledigen" erscheint nur mit Inhalt; der leere Zustand erscheint sonst; je Kind genau eine Tageskarte; kein Element mit dem Text "Neue Anmeldung".
- [ ] Fehlschlag bestätigen, umsetzen, Erfolg bestätigen, committen.

---

### Aufgabe 3: Kinderbereich, kind-zentriert

Nach Entscheidung E9: Bei **einem** Kind entfällt jede Liste, die Seite zeigt direkt dieses Kind. Bei **mehreren** steht oben ein Umschalter mit Initialen-Kreisen; der aktive trägt einen grünen Ring und den Namen darunter.

**Vier Abschnitte in dieser Reihenfolge:**

| Abschnitt | Inhalt |
|---|---|
| **Heute** | Tageskarte aus Aufgabe 1, darunter die geplante Abholzeit im Klartext |
| **Gebuchte Betreuung** | Nur die tatsächlich gebuchte Betreuung mit Wochentagen. Mobil eine Karte je Wochentag statt einer Tabelle. Aus dem Stundenplan abgeleitete Ankunftszeiten erscheinen **als reine Anzeige**, nie als änderbares Feld (#2250, war die Ursache falscher Elternänderungen). |
| **Daten von {Name}** | Die bisherigen Stammdaten, in Elternsprache beschriftet |
| **Eltern und Abholberechtigte** | Sorgeberechtigte und "Wer hat noch Zugang?" zusammengeführt, keine doppelte Personendarstellung (#2308) |

**Ersatzlos entfernt:** der Abschnitt "Betreuungszeiten" (#2302) und der Block "AGs und Gruppen" (#2303), samt Routen, API-Aufrufen, Komponenten und Übersetzungen, die danach niemand mehr nutzt. Eltern konnten dort ohnehin nichts anmelden; die reine Anzeige hat verwirrt.

- [ ] Test zuerst: bei einem Kind keine Liste und kein Umschalter; bei mehreren ein Umschalter, der den Inhalt wechselt; die vier Abschnitte in der festgelegten Reihenfolge; **kein** Element mit "Betreuungszeiten" oder "AGs"; die Ankunftszeit ist kein Eingabefeld.
- [ ] Fehlschlag bestätigen, umsetzen, Erfolg bestätigen, committen.

---

### Aufgabe 4: Nachrichten

- Ein Kind: die Seite **ist** die Unterhaltung, ohne Zwischenschritt (Verhalten besteht bereits, bleibt).
- Mehrere Kinder: eine Liste mit einer Zeile je Kind, Initialen-Kreis, Name 17/600, letzte Nachricht 15 gekürzt, Zeitstempel, Ungelesen-Zähler. Zeilenhöhe mindestens 72 px.
- Der Verlauf bekommt Sprechblasen mit klarer Absenderunterscheidung: OGS links weiß mit Rand, eigene Nachrichten rechts in blasser Grünfüllung. **Keine kräftige Vollfüllung**, kein Verlauf.
- Eingabefeld unten angeheftet, mindestens 48 px, Senden-Schaltfläche daneben, `env(safe-area-inset-bottom)` respektiert.

- [ ] Test zuerst, Fehlschlag bestätigen, umsetzen, Erfolg bestätigen, committen.

---

### Aufgabe 5: Kalender als Terminliste

Nach Entscheidung E10 wird `PersonalCalendar` im Elternportal **nicht** mehr verwendet.

- Chronologische Liste, gruppiert nach "Diese Woche", "Nächste Woche", "Später", je Gruppe eine Überschrift 20/600.
- Termineintrag: Datum und Uhrzeit 15 in der Zustandsfarbe, Titel 17/600, betroffenes Kind 15 grau, darunter bei offener Rückmeldung zwei Schaltflächen "Zusagen" und "Absagen" je 48 px. Bereits beantwortet: eine Zeile mit Haken und "Zugesagt" bzw. "Abgesagt".
- Termine ohne Rückmeldebedarf tragen keine Schaltflächen.
- Ein Monatsraster erscheint **nur** ab 1024 px als Zusatz neben der Liste, nie auf dem Handy.
- Das ICS-Abo bleibt erreichbar, wandert aber ans Ende der Seite.
- **Die Seite ist vollständig zu lokalisieren**, sie ist heute hart auf Deutsch verdrahtet.

- [ ] Test zuerst, Fehlschlag bestätigen, umsetzen, Erfolg bestätigen, committen.

---

### Aufgabe 6: Aus der OGS (Neuigkeiten) und das Mehr-Sheet

**Neuigkeiten** heißt künftig "Aus der OGS". Liste von Meldungen, ungelesene tragen links eine blaue Kante und den Titel in 600. Umfragen tragen eine orange Kante und die Schaltfläche "Antworten". Bestätigungspflichtige Meldungen tragen "Gelesen bestätigen".

**Mehr-Sheet**: von unten einfahrendes Sheet mit Neuigkeiten (samt Zähler), Essensplan, Benachrichtigungen, Sprache, Neue Anmeldung, Abmelden. Einträge 56 px, Icon links, Text 17, Pfeil rechts. Schließbar über Hintergrund und Wischgeste.

**Feedback ans Produktteam wird aus der Eltern-Navigation entfernt** (Teilmenge von #2326). Die Route bleibt vorerst bestehen, der vollständige Rückbau über alle Portale ist #2326 vorbehalten.

- [ ] Test zuerst, Fehlschlag bestätigen, umsetzen, Erfolg bestätigen, committen.

---

### Aufgabe 7: Dialoge auf Elternsprache und Touch-Maß

Betrifft Krankmeldung und Abholung ändern. **Fachlich ändert sich nichts** (Entscheidung E5): Krankmeldung bleibt direkt, die Abholzeit bleibt frei eingebbar mit Pflichtgrund. Nur die Bedienbarkeit ändert sich.

- Auf Mobile als Sheet von unten mit angehefteter Fußzeile, auf Tablet und Desktop als mittiges Fenster.
- Hauptaktion volle Breite, 48 px, beschriftet mit der Folge: "Krankmeldung senden", "Abholung ändern". Nicht "Speichern".
- Sekundäraktion darunter als Textschaltfläche.
- Pflichtfelder mit Stern **und** einmal je Formular die Zeile "Felder mit * müssen ausgefüllt werden".
- Fehler stehen am Feld, in Alltagssprache: "Bitte tragen Sie eine Uhrzeit ein."
- Kein rohes natives `--:--`-Zeitfeld mehr; das Kit-Zeitfeld mit 48 px und sichtbarem Format.
- Ein Satz oben, der die Folge benennt: "Die OGS wird sofort informiert."

- [ ] Test zuerst, Fehlschlag bestätigen, umsetzen, Erfolg bestätigen, committen.

---

### Aufgabe 8: Abschluss

- [ ] Tote Dateien löschen, die nach den Aufgaben 1 bis 7 niemand mehr importiert. `pnpm run knip` hilft beim Finden.
- [ ] `frontend/src/components/help/guide-data.ts` aktualisieren: Navigation und Kinderbereich haben sich geändert, der In-App-Hilfe-Guide dokumentiert sie (Regel `help-guide-sync`).
- [ ] Vollständiger Prüflauf:

```bash
cd frontend && pnpm vitest run
cd frontend && pnpm run check
```

- [ ] Aufnahmen aller fünf Seiten in 390×844, 834×1194 und 1440×900, vorher gegen `origin/development` und nachher gegen diesen Branch.

---

## Selbstprüfung

| Anforderung | Aufgabe |
|---|---|
| Startseite zeigt pro Kind Status, offene Punkte, drei Hauptaktionen (#2308) | 1, 2 |
| Leere Neuigkeiten verdrängen keine Tagesaufgaben (#2308) | 2 |
| "Neue Anmeldung" nicht mehr Hauptaktion im Kopf (#2308) | 2 |
| Kindprofil trennt Heute, Betreuung, Daten, Kontakte (#2308) | 3 |
| Doppelte Personendarstellungen entfernt (#2308) | 3 |
| Betreuungszeiten vollständig entfernt (#2302) | 3 |
| AGs und Gruppen vollständig entfernt (#2303) | 3 |
| Nur gebuchte Betreuung sichtbar (#2250) | 3 |
| Stundenplan-Ankunftszeit nicht als änderbares Feld (#2250) | 3 |
| Tagesstatus prominent, ohne Frontend-Ableitung (#2250, #2252) | 1 |
| Kalender lokalisiert und elterntauglich (E10) | 5 |
| Alle Texte in Elternsprache, vier Kataloge | alle |
| Kernaufgaben bei 320 px, 200 % Zoom, per Tastatur (#2308) | 8 |
| In-App-Hilfe aktualisiert | 8 |
