# moto Anleitung Content Map

Stand: 2026-05-30

## Ziel

Die Anleitung soll drei Fragen beantworten:

1. Wie richtet eine Einrichtung moto sauber ein?
2. Wie schlagen Mitarbeitende im Alltag eine konkrete Aufgabe nach?
3. Wie kann die Einrichtung die Inhalte drucken oder als PDF speichern?

Darum ist die Anleitung kein langer Fließtext. Sie besteht aus
aufgabenorientierten Artikeln mit einheitlicher Struktur.

## Produktstruktur

Implementiert sind eine Startseite und drei Anleitungsseiten. Jede Seite
gruppiert ihre Karten in betitelte Kapitel mit eigener Akzentfarbe (`tone`).
Es gibt keine separate Druck- oder Variantenseite: Drucken läuft über den
`PDF speichern`-Button (print-optimierte Styles auf jeder Seite), die drei
Seiten erreicht man über die Tab-Leiste im Kopf.

### `/help`

Startseite mit drei Einstiegskarten (`EntryPointCard`):

- `Ersteinrichtung`
- `Die App im Alltag`
- `NFC & Tablets`

### `/help/setup`

Lineares Setup-Handbuch für Leitung und Admins, abhängigkeitsgeordnet und
durchnummeriert. Kapitel:

1. Zugang und Team (Konto, Mitarbeitende)
2. Ihre OGS-Struktur anlegen (Räume, Gruppen, Aktivitäten)
3. Kinder und Betreuungszeiten (Import, manuell, Betreuungszeiten)
4. Testlauf vor dem Start (Go-live-Check als Checkliste)

### `/help/features`

Nachschlage-Doku für Mitarbeitende und Vertretungen, in Seitenleisten-Reihenfolge.
Karten tragen das jeweilige Seitenleisten-Icon statt einer Nummer. Kapitel:

1. Alltag und Aufsicht (Alle Kinder, Meine Gruppen, Aktuelle Aufsicht)
2. Räume, Team und Vertretung (Aktivitäten, Räume, Mitarbeiter, Vertretungen)
3. Planung und Zeit (Stundenplan, Zeiterfassung)
4. Verwaltung und Austausch (Datenverwaltung, Anmeldungen, Feedback)

### `/help/nfc`

Zusätzliche Schritte nur für Einrichtungen mit Tablets oder NFC-Armbändern.
Ersteinrichtung und Funktionen gelten unverändert weiter. Kapitel:

1. Daten und Geräte vorbereiten (Kinder prüfen, Namen, Geräte)
2. Vor dem ersten Einsatz (Checkliste vor dem ersten Tablet-Einsatz)

## Artikelstruktur (`GuideStep`)

Jede Karte ist ein `GuideStep` in `guide-data.ts` mit diesen Feldern:

- `title`: Aufgabe oder Seitenleisten-Bereich.
- `summary`: Ein bis zwei Sätze, was die Karte erledigt.
- `steps?`: Nummerierte Handlungsschritte. Optional, weil eine Karte
  stattdessen `checklist` tragen kann.
- `checklist?`: Prüfpunkte, als Häkchenliste gerendert (z. B. Go-live-Check).
- `callout?`: Hervorgehobener Hinweis mit eigenem `tone` (z. B. Passwort
  vergessen, Geburtstage im Import).
- `screenshot`: Bildunterschrift / Alt-Text.
- `image?`: Pfad unter `/public/help/screens/...`. Fehlt das Bild, zeigt
  die Karte einen gestrichelten Platzhalter mit der Bildunterschrift.
- `icon?`: Lucide-Icon statt Nummer (nur auf `funktionen`).

Karten liegen in `GuideChapter`-Gruppen (`id`, `title`, `description`, `icon`,
`tone`, `steps`). Der `tone` färbt Kapitel-Icon, Karten-Badge und Callout.

## Zielgruppen

Artikel markieren ihre Zielgruppe:

- `Leitung/Admin`
- `Mitarbeitende`
- `Vertretung`
- `NFC-Verantwortliche`

Die Zielgruppe ist wichtig, weil nicht jede Person Einstellungen oder
Stammdaten ändern darf.

## Varianten

Es gibt keine getrennten `standard`/`nfc`-Varianten mehr. Ersteinrichtung und
Funktionen gelten für alle Einrichtungen. NFC ist eine eigene Zusatzseite, die
die anderen beiden ergänzt, nicht ersetzt.

## Schreibregeln

- Kurze Sätze.
- Button- und Feldnamen exakt schreiben.
- Keine Produktphilosophie in Alltagsartikeln.
- Keine echten Kindernamen, Telefonnummern oder produktiven E-Mail-Adressen in
  Screenshots.
- Fehlerfälle direkt benennen.
- Wenn ein Schritt Admin-Rechte braucht, die Zielgruppe entsprechend setzen.

## Druckregeln

- Navigation ausblenden.
- Inhaltsverzeichnis zeigen.
- Checklisten zusammenhalten.
- Screenshots im Druck nicht als leere Platzhalter ausgeben.
- Stand der Doku nennen.
- Link zur Online-Version nennen: `/help`.

## Offene Arbeit

- Echte Screenshots liegen unter `/public/help/screens/`. Die vier
  NFC-Karten haben noch keine Bilder und zeigen den Platzhalter.
- Doku aus der Portal-Navigation verlinken, falls gewünscht.
- Bei größeren UI-Änderungen die betroffenen Artikel suchen und prüfen.
