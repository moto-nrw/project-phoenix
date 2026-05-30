# moto Anleitung Content Map

Stand: 2026-05-29

## Ziel

Die Anleitung soll drei Fragen beantworten:

1. Wie richtet eine Einrichtung moto sauber ein?
2. Wie schlagen Mitarbeitende im Alltag eine konkrete Aufgabe nach?
3. Wie kann die Einrichtung die Inhalte drucken oder als PDF speichern?

Darum ist die Anleitung kein langer Fließtext. Sie besteht aus
aufgabenorientierten Artikeln mit einheitlicher Struktur.

## Produktstruktur

### `/anleitung`

Startseite mit Suche und vier Einstiegen:

- `Ersteinrichtung`
- `Alltag & Funktionen`
- `NFC & Geräte`
- `Druckversion`

### `/anleitung/setup`

Lineares Setup-Handbuch für Leitung und Admins. Diese Seite wird der Reihe
nach abgearbeitet.

Kapitel:

1. Zugang und Grundprüfung
2. Stammdaten anlegen
3. Kinder und Betreuungszeiten
4. Planung und Verwaltung testen
5. Go-live-Check

### `/anleitung/alltag`

Nachschlage-Doku für Mitarbeitende und Vertretungen. Die Artikel sind nicht
nach Datenbankbereichen sortiert, sondern nach Aufgaben.

Beispiele:

- Ein Kind schnell finden
- Was tun, wenn ein Kind nicht gefunden wird?
- Kind einchecken, entschuldigen oder zurücksetzen
- Spontane Aktivität starten
- Räume prüfen und Kinder unterwegs zuweisen
- Arbeitszeit, Pause und Abwesenheit erfassen

### `/anleitung/standard`

Gesamtansicht für Einrichtungen ohne NFC. Enthält Setup, Alltag und
Nachschlagen.

### `/anleitung/nfc`

Gesamtansicht für Einrichtungen mit NFC. Enthält alle Standard-Inhalte plus
NFC-Vorbereitung.

### `/anleitung/print`

Druckbare Gesamtansicht. Navigation und interaktive Elemente werden im Druck
ausgeblendet. Kapitel und Artikel bleiben zusammen, soweit der Browserdruck es
zulässt.

## Artikelstruktur

Jeder Artikel soll diese Felder haben:

- `Ziel`: Was ist nach dem Artikel erledigt?
- `Voraussetzungen`: Was muss vorher existieren?
- `Schritt für Schritt`: Klickpfad und Handlung.
- `Pflichtfelder`: Nur wenn Eingaben nötig sind.
- `Prüfen`: Woran erkennt die Person, dass es geklappt hat?
- `Typische Fehler`: Was geht oft schief?
- `Hinweise`: Kurze Zusatzinfos, falls nötig.
- `Screenshot`: Gewünschter Screenshot-Zustand.

Diese Struktur macht die Inhalte für Online-Doku und Druck nutzbar.

## Zielgruppen

Artikel markieren ihre Zielgruppe:

- `Leitung/Admin`
- `Mitarbeitende`
- `Vertretung`
- `NFC-Verantwortliche`

Die Zielgruppe ist wichtig, weil nicht jede Person Einstellungen oder
Stammdaten ändern darf.

## Varianten

Artikel markieren, ob sie für `standard`, `nfc` oder beide Varianten gelten.
NFC-Inhalte ergänzen die Standard-Doku. Sie ersetzen sie nicht.

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
- Link zur Online-Version nennen: `/anleitung`.

## Offene Arbeit

- Echte Screenshots mit anonymisierten Demo-Daten erstellen.
- Doku aus der Portal-Navigation verlinken, falls gewünscht.
- Bei größeren UI-Änderungen die betroffenen Artikel suchen und prüfen.
