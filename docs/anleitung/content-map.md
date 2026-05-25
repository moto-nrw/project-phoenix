# moto Anleitung Content Map

Stand: 2026-05-25

## Produktformat

Die Anleitung wird eine klare Dokumentationsseite im moto Design. Sie ist keine gespeicherte Einrichtungshilfe und kein interaktiver Assistent. Die Seite erklärt moto Schritt für Schritt und kann sauber als PDF gedruckt werden.

## Varianten

### moto Standard

Für die Nutzung ohne NFC-Tablet. Schwerpunkt:

- Konto erstellen und anmelden
- Kinder anlegen oder importieren
- Mitarbeitende anlegen und einladen
- Räume anlegen
- Gruppen anlegen
- Kindersuche nutzen
- aktuelle Aufsicht nutzen
- Stundenplan nutzen
- spontane Aktivitäten starten
- Zeiterfassung nutzen
- Anmeldungen verwalten
- Feedback nutzen
- Einstellungen verstehen

### moto NFC

Ergänzung zu moto Standard. NFC wird nur dort erklärt, wo moto vorbereitet werden muss:

- Aktivitäten für das Tablet vorbereiten
- Kinder in moto sauber anlegen, bevor Armbänder zugewiesen werden
- Räume, Gruppen und Aktivitäten so benennen, dass sie im Tablet verständlich sind
- typische Prüfungen vor dem Tablet-Einsatz

Die vorhandene NFC-PDF wird nur als Nebenquelle verwendet. Tablet-Bedienung selbst gehört nicht in diese Anleitung.

## Struktur der Anleitung

Die Navigation soll nicht die gesamte Produktbreite auf einmal zeigen. Die erste Ebene bleibt klein:

1. `Startseite`
2. `moto Standard`
3. `moto NFC`

Die Startseite fragt zuerst ab, welche Version genutzt wird:

- `moto Standard`
- `moto NFC`

Nach der Auswahl führt die jeweilige Anleitung in die passenden Kapitel.

Innerhalb von `moto Standard` bleibt die erste Ebene klein:

1. `Erste Schritte`
2. `Im OGS-Alltag`
3. `Planung`
4. `Kinderakten und Verläufe`
5. `Anmeldungen`
6. `Feedback`
7. `Weitere Funktionen und Einstellungen`

Innerhalb von `moto NFC` gibt es zusätzlich eigene NFC-Kapitel:

1. `NFC vorbereiten`
2. `Aktivitäten für Tablets`
3. `Geräte prüfen`
4. `Vor dem ersten Einsatz`

Innerhalb der Kapitel werden einzelne Aufgaben als kurze Schrittlisten dargestellt. Spezialbereiche liegen hinten im Nachschlagewerk.

## Routeninventar

### Zugang

| Bereich | Route | Sichtbare Begriffe |
| --- | --- | --- |
| Login | `/[tenant]` | Ihr OGS Portal, E-Mail-Adresse, Passwort, Passwort vergessen?, Anmelden |
| Einladung | `/[tenant]/invite` | Konto erstellen, Einladung |
| Passwort zurücksetzen | `/[tenant]/reset-password` | Passwort zurücksetzen |

### Hauptnavigation

| Bereich | Route | Sichtbare Begriffe |
| --- | --- | --- |
| Home | `/dashboard` | Home |
| Kindersuche | `/students/search` | Kindersuche |
| Aktivitäten | `/activities` | Aktivitäten, Aktivität suchen..., Aktivität erstellen, Meine Aktivitäten |
| Räume | `/rooms` | Räume, Raum suchen..., Frei, Belegt, Unterwegs, Zuweisen |
| Mitarbeiter | `/staff` | Mitarbeiter |
| Vertretungen | `/substitutions` | Vertretungen, Fachkraft suchen..., Verfügbar, In Vertretung, Tagesübergaben, Zuweisen, Beenden |
| Stundenplan | `/timetables` | Stundenplan, Termin, Serien, Planungsperiode, Lücken füllen, Woche neu berechnen |
| Zeiterfassung | `/time-tracking` | Zeiterfassung, Einstempeln, Ausstempeln, Pause, Homeoffice, Abwesenheit melden |
| Feedback | `/suggestions` | Feedback, Neuer Beitrag, Beliebteste, Neueste, Alle Status |
| Einstellungen | `/settings` | Betrieb, Datenschutz, Geräte, Anmeldung, System, Allgemein |
| Profil | `/profile` | Profil, Profilbild ändern, Bearbeiten, Passwort ändern |

### Datenverwaltung

| Bereich | Route | Sichtbare Begriffe |
| --- | --- | --- |
| Übersicht | `/database` | Kinder, Personal, Räume, Aktivitäten, Gruppen, Rollen, Geräte, Berechtigungen, Verwalten |
| Kinder | `/database/students` | Kinder, Importieren, Schüler |
| Kinderimport | `/database/students/import` | Import-Anleitung, Vorlage herunterladen, Datei hochladen, Datenvorschau, Schüler importieren |
| Personal | `/database/personal` | Personal, Vorname, Nachname, E-Mail, Rolle, Temporäres Passwort |
| Räume | `/database/rooms` | Räume, Raumname, Kategorie, Gebäude, Etage, Farbe |
| Aktivitäten | `/database/activities` | Aktivitäten, Name, Kategorie, Maximale Teilnehmer, Hauptbetreuer |
| Gruppen | `/database/groups` | Gruppen, Gruppenname, Gruppenraum, Gruppenleitung |
| Gruppenkombinationen | `/database/groups/combined` | Kombinierte Gruppen |
| Rollen | `/database/roles` | Rollen, Name, Beschreibung, Systemrolle, Berechtigungen |
| Geräte | `/database/devices` | Geräte, Geräte-ID, Gerätetyp, Gerätename, Verbindung, Status, API-Schlüssel |
| Berechtigungen | `/database/permissions` | Berechtigungen, Ressource, Aktion, Anzeigename |

### Anmeldungen

| Bereich | Route | Sichtbare Begriffe |
| --- | --- | --- |
| Überblick | `/admin/enrollments` | Überblick, Anmeldephasen und Eingänge |
| Anmeldephasen | `/enrollment-phases` | Anmeldephasen, Schuljahr, Ferienbetreuung, Sonstiges |
| Betreuungsangebote | `/care-offerings` | Betreuungsangebote, Wochentage, Kapazität, Preis, Ohne Preis |
| Anmeldeformulare | `/enrollment-form` | Anmeldeformulare, Formularvorlage, Vorschau, Formularfelder |
| Anfrage prüfen | `/admin/enrollments/[id]` | Anmeldedetail, Entscheidung, Genehmigen, Ablehnen, Warteliste |
| Öffentliche Anmeldung | `/[tenant]/enroll` | Auswahl, Formular, Bestätigung |
| Elternbereich | `/parents` | Elternbereich, Kinder, Anmeldung |

### Kinderdetail

| Bereich | Route | Sichtbare Begriffe |
| --- | --- | --- |
| Kinderdetail | `/students/[id]` | Stammdaten, Erziehungsberechtigte, Betreuungszeiten, Historie |
| Anwesenheitsprotokoll | `/students/[id]/room-history` | Anwesenheitsprotokoll |
| Feedbackhistorie | `/students/[id]/feedback_history` | Feedbackhistorie, Alle, Heute, Diese Woche, Letzte 7 Tage, Diesen Monat |
| Mensaverlauf | `/students/[id]/mensa_history` | Mensaverlauf |

## Kapitelabdeckung

### 1. Erste Schritte

Enthält:

- Konto erstellen und anmelden
- Navigation verstehen
- Kinder manuell anlegen
- Kinder per Excel oder CSV importieren
- Mitarbeitende anlegen
- Räume anlegen
- Gruppen anlegen
- Aktivitäten anlegen

### 2. Im OGS-Alltag

Enthält:

- Kindersuche
- aktuelle Aufsicht
- spontane Aktivität
- Räume Hauptseite
- Mitarbeiter Hauptseite
- Vertretungen
- Zeiterfassung

### 3. Planung

Enthält:

- Stundenplan
- Planungsperiode
- Termin
- Serie
- Termine erzeugen
- Lücken füllen
- Woche neu berechnen
- geplante Aktivität starten

### 4. Kinderakten und Verläufe

Enthält:

- Stammdaten
- Erziehungsberechtigte
- Betreuungszeiten
- Ankunftsplan
- Abholplan
- Historie
- Anwesenheitsprotokoll
- Feedbackhistorie
- Mensaverlauf

### 5. Anmeldungen

Enthält:

- Überblick
- Anmeldephasen
- Betreuungsangebote
- Anmeldeformulare
- öffentliche Anmeldung
- Anfragen prüfen

### 6. Feedback

Enthält:

- Feedbackübersicht
- Sortierung
- Statusfilter
- Beitrag erstellen
- Beitrag bearbeiten oder löschen
- Stimmen und Kommentare, falls sichtbar

### 7. Weitere Funktionen und Einstellungen

Enthält:

- Datenverwaltung Übersicht
- Rollen
- Berechtigungen
- Geräte
- Einstellungen
- Profil

## Best Practices für die Web-Doku

Aus der Websuche abgeleitete Regeln:

- Kurze, direkte Sätze verwenden.
- Fachbegriffe beim ersten Auftreten erklären.
- Schrittlisten so schreiben, dass eine Person einen Schritt liest, ihn ausführt und danach den nächsten Schritt findet.
- Nicht alle Informationen vorne zeigen. Erst die wichtigste Aufgabe erklären, Details später anzeigen.
- Screenshots immer nah an den beschriebenen Schritt setzen.
- Keine rein farbbasierten Anweisungen wie "grünen Button klicken".
- PDF-Export muss durchsuchbar und mit Screenreadern nutzbar bleiben.

Quellen:

- Department for Education, Plain Language Standard: https://design.education.gov.uk/content-design/plain-language
- GOV.UK Content Principles: https://www.gov.uk/government/publications/govuk-content-principles-conventions-and-research-background/govuk-content-principles-conventions-and-research-background
- GOV.UK Accessible Documents: https://www.gov.uk/guidance/publishing-accessible-documents
- IBM Progressive Disclosure: https://www.ibm.com/docs/en/technical-content?topic=practices-progressive-disclosure

## Markdown-Dateien

- `docs/anleitung/landing.md`
- `docs/anleitung/standard.md`
- `docs/anleitung/nfc.md`
- `docs/anleitung/screenshots.md`

Die Dateien dienen als Inhaltsquelle. Die spätere Webpage kann daraus direkt Content-Blöcke übernehmen oder den Inhalt in TypeScript-Strukturen spiegeln.

## Schreibregeln für die sichtbare Anleitung

- Kurze Sätze.
- Ein Schritt pro Satz.
- Buttontexte exakt wie in moto.
- Keine internen Begriffe.
- Keine Fachsprache, wenn ein Alltagssatz reicht.
- Keine internen Hinweise auf Entwicklung, Tickets oder Arbeitsstände.
- Keine Beschreibung der Leserschaft.
- Keine Begriffe aus Einrichtungsassistenten.
- Erst erklären, was zu tun ist, dann warum es wichtig ist.

## UI-Prinzipien

- Doku-Seite, keine Marketing-Seite.
- Direkter Einstieg mit `moto Standard` und `moto NFC`.
- Linke Kapitel-Navigation.
- Sticky Inhaltsübersicht auf großen Bildschirmen.
- Schrittlisten mit klarer Nummerierung.
- Kleine Checklisten am Kapitelende.
- Ruhige Hinweisboxen.
- Screenshots nah am beschriebenen Schritt.
- Print/PDF Ansicht ohne Navigation, Animationen und interaktive Controls.
- Mobiltauglich, aber lange Tabellen im PDF lesbar halten.
