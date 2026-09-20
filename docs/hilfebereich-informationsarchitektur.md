# Hilfebereich: Informationsarchitektur nach Rollen und Aufgaben

**Status:** Arbeitsentwurf zur gemeinsamen Abstimmung  
**Issue:** #2229  
**Stand:** 31.08.2026

## 1. Ziel

Der Hilfebereich bekommt für jede Rolle eine eigene Anleitung mit eigener Seitenleiste:

1. Betreuungskräfte
2. Leitungen
3. Eltern
4. Lehrkräfte

Eine Person sieht innerhalb einer Anleitung nur die Themen ihrer Rolle. Die Oberthemen ordnen
ähnliche Aufgaben. Die Unterthemen sind konkrete Anwendungsfälle. Jeder Anwendungsfall erhält eine
Anleitung mit kurzen Schritten.

```text
Rolle
└── Oberthema
    └── Anwendungsfall
        └── Anleitung mit kurzen Schritten
```

Diese Struktur ersetzt die bisherige Gliederung nach langen Handbüchern und Bereichen der
App-Navigation.

## 2. Begriffe

### Rolle

Eine Rolle steht für eine eigene Zielgruppe und eine eigene Seitenleiste. Die Rolle legt fest,
welche Oberthemen und Anwendungsfälle zuerst angeboten werden.

### Oberthema

Ein Oberthema fasst zusammengehörige Anwendungsfälle zusammen. Es erklärt keine Funktion und ist
keine eigene lange Anleitung.

Beispiele:

- Kinder und Anwesenheit
- Gruppen und Aufsicht
- Team und Rechte
- Probleme lösen

### Anwendungsfall

Ein Anwendungsfall beschreibt eine konkrete Aufgabe oder ein konkretes Problem. Er bekommt eine
eigene Seite und eine eigene Adresse.

Gute Titel:

- Ein Kind finden
- Eine Abwesenheit eintragen
- Eine Aufsicht starten
- Eine Betreuung beenden
- Ein Armband wird nicht erkannt

Keine Anwendungsfälle sind allgemeine Namen von Seiten oder Bereichen:

- Home
- Kinderdetailansicht
- Datenverwaltung
- Einstellungen im Überblick

Aus solchen Namen werden konkrete Aufgaben. Aus „Kinderdetailansicht“ können zum Beispiel
„Kontaktdaten eines Kindes ansehen“ und „Eine Betreuung beenden“ werden.

## 3. Gemeinsame Strukturregeln

1. Jede Rolle hat eine eigene Seitenleiste.
2. Oberthemen dienen nur der Ordnung.
3. Jeder anklickbare Eintrag beschreibt einen Anwendungsfall.
4. Eine Seite erklärt möglichst nur eine Aufgabe.
5. Die Reihenfolge folgt dem Arbeitsalltag, nicht der App-Navigation.
6. Voraussetzungen stehen vor den Schritten.
7. Probleme werden als konkrete Situationen benannt.
8. Verwandte Anwendungsfälle stehen am Ende jeder Seite.
9. Einstellungen erzeugen keine zusätzliche Ebene in der Seitenleiste.
10. Unterschiede durch NFC, Anwesenheits-Modus oder Gruppenmodell verändern nur die betroffenen
    Schritte, Hinweise oder Bilder.
11. Einzelne Klicks oder direkt aufeinanderfolgende Schritte sind keine eigenen Anwendungsfälle.
12. Eine Problemlösung steht zuerst beim zugehörigen Anwendungsfall. Nur übergreifende Probleme
    bekommen eine eigene Seite.

## 4. Umgang mit gemeinsamen Anwendungsfällen

Die sichtbaren Anleitungen bleiben nach Rollen getrennt. Ein Anwendungsfall darf trotzdem in
mehreren Seitenleisten erscheinen.

- Ist der Ablauf für mehrere Rollen gleich, verwenden alle dieselbe Seite.
- Unterscheiden sich Handlung, Voraussetzung oder Ergebnis, entstehen getrennte Seiten.
- Gemeinsame Seiten werden nicht kopiert. Dadurch bleiben Text und Bilder an einer Stelle aktuell.
- Jede Rolle sieht nur ihre eigene Einordnung und Reihenfolge.

Beispiel: „Ein Kind finden“ kann für Betreuungskräfte und Leitungen dieselbe Seite sein. „Ein Kind
auschecken“ braucht eine andere Seite, wenn die Rollen unterschiedliche Schaltflächen oder Rechte
haben.

## 5. Aufbau einer Anwendungsfall-Seite

Jede Seite folgt grundsätzlich demselben Muster:

1. **Titel:** eine klare Aufgabe oder Frage.
2. **Kurzbeschreibung:** ein Satz zum Ziel.
3. **Das brauchen Sie:** nur wenn eine Voraussetzung besteht.
4. **So geht es:** kurze nummerierte Schritte.
5. **Danach:** woran die Person das Ergebnis erkennt.
6. **Wenn es anders aussieht:** nur bei einem wichtigen Unterschied.
7. **Wenn es nicht klappt:** direkter Link zur passenden Problemlösung.
8. **Weitere Themen:** höchstens drei sinnvolle nächste Schritte.

Nicht jede Seite braucht jeden Abschnitt. Weglassen ist besser als ein Abschnitt ohne Nutzen.

## 6. Anleitung für Betreuungskräfte

Diese Anleitung beginnt mit den häufigen Aufgaben während eines Betreuungstags. Geprüft wurde die
Standardrolle `Betreuer` (`user`) gegen die aktuelle Navigation, die Seiten und die Berechtigungen.
Eigene Rollen einer Schule können davon abweichen.

Ein direkter Seitenaufruf ohne sichtbaren Einstieg zählt hier nicht als regulärer Anwendungsfall.
Die Anleitung soll zeigen, was Betreuungskräfte in ihrem normalen Arbeitsablauf erreichen können.

Die drei wichtigsten Einstellungen verändern, welche Abläufe in der App erscheinen. Die Hilfeseiten
bleiben auffindbar und erklären, wann ein Ablauf nicht verfügbar ist:

- Bei offener Betreuung entfällt `Meine Gruppen`.
- Bei einfacher Anwesenheit entfallen Räume, Aktivitäten und die aktuelle Aufsicht.
- Ohne NFC entfallen die Tablet-Themen und der Bereich `Aktivitäten`.

### Einstieg

- Einladung annehmen und Konto einrichten
- Bei moto anmelden
- moto als App auf dem Handy oder Tablet hinzufügen
- Sich in moto zurechtfinden

### Tagesplanung

- Meine Termine und Einsätze ansehen
- Den Betreuungsplan ansehen

### Kinder und Anwesenheit

- Ein Kind finden und Angaben ansehen
- Angaben oder Betreuungszeiten eines Kindes ändern
- Ein Kind an- oder abmelden
- Den Aufenthaltsort eines Kindes ändern (nur bei detaillierter Anwesenheit)
- Ein Kind krank oder entschuldigt melden
- Den heutigen Betreuungstag prüfen (wenn die Tagesauswertung eingeschaltet ist)
- Eine Notfallliste drucken

### Gruppen, Räume und Aufsicht

- Meine Gruppen ansehen (nur bei festen Gruppen)
- Meine Gruppe vorübergehend übergeben (nur bei festen Gruppen)
- Räume und ihre aktuelle Belegung ansehen (nur bei detaillierter Anwesenheit)
- Eine Aufsicht starten, führen und beenden (nur bei detaillierter Anwesenheit)
- Eine Aktivität anlegen oder ändern (nur mit NFC und detaillierter Anwesenheit)

### Mit Eltern und dem Team arbeiten

- Eltern eine Nachricht schreiben
- Anfragen von Eltern prüfen und bearbeiten (wenn eingeschaltet)
- Den Team-Chat nutzen (wenn er eingeschaltet ist)
- Eine Person aus dem Team finden
- Eine gemeinsame Datei öffnen oder hochladen (wenn Uploads für Mitarbeitende erlaubt sind)

### Meine Arbeitszeit

- Arbeitszeit und Pausen erfassen
- Eigene Arbeitszeit prüfen und korrigieren
- Urlaub beantragen und den Stand prüfen
- Eine eigene Abwesenheit eintragen

### NFC-Tablet benutzen

- Mit der PIN am Tablet anmelden
- Ein Armband zuweisen oder die Zuweisung ändern
- Die eigene Arbeitszeit mit dem Armband erfassen
- Eine Aufsicht am Tablet starten und beenden
- Kinder mit dem Armband ein- und auschecken

### Wenn etwas nicht klappt

- Ein Menüpunkt fehlt
- Ein Kind oder eine Gruppe fehlt
- Ich kann ein Kind nicht an- oder abmelden
- Das NFC-Tablet funktioniert nicht
- Ich kann mich nicht anmelden

### Bewusst nicht als Betreuungskräfte-Thema eingeordnet

- `Home`, `Datenverwaltung`, `Anmeldungen`, `Einstellungen` und die bearbeitbaren Planungsseiten
  sind in der Navigation admin-gesperrt.
- `Gruppenzugriff` wird von einer Leitung vergeben. Betreuungskräfte übergeben nur ihre eigene
  Gruppe. Eine zugewiesene Gruppe erscheint danach automatisch unter `Meine Gruppen`.
- `Vertretung` ist eine Planungsseite für Admins. Änderungen an eigenen Einsätzen sehen
  Betreuungskräfte unter `Mein Kalender`.
- `Tageslisten` werden Betreuungskräften in der Navigation nicht angeboten. Der direkte Aufruf ist
  mit den Rechten der Standardrolle technisch möglich. Bis diese Abweichung im Produkt geklärt ist,
  gehört der Ablauf nicht in die Betreuungskräfte-Anleitung.

Die Standardrolle besitzt außerdem `users:update`. Dadurch lassen sich in der aktuellen App nicht
nur Kinderdaten, sondern auch Teile von Personal-Stammdaten ändern. Die Informationsarchitektur
ordnet die Personalverwaltung trotzdem den Leitungen zu, weil ein technisches Recht nicht
automatisch eine typische Aufgabe der Rolle ist. Diese Produkt- und Rechteentscheidung sollte vor
der vollständigen Inhaltsmigration bestätigt werden.

## 7. Anleitung für Leitungen

Diese Anleitung beginnt mit Einrichtung, Organisation und wiederkehrender Verwaltung.

### Einstieg

- Bei moto anmelden
- moto als App auf dem Handy oder Tablet hinzufügen

### moto für die OGS vorbereiten

- moto für den ersten Betreuungstag vorbereiten
- Räume anlegen
- Gruppen anlegen
- Aktivitäten anlegen
- Kinder aus einer Liste übernehmen
- Ein Kind einzeln anlegen
- Betreuungszeiten eintragen

Die erste Seite ist eine kurze Checkliste. Sie verlinkt auf die weiteren Anwendungsfälle und
wiederholt deren Schritte nicht.

### Kinder verwalten

- Ein Kind finden und Angaben ansehen
- Angaben eines Kindes verwalten
- Eltern-Konten mit einem Kind verbinden
- Die Betreuung eines Kindes beenden
- Ein Kind dauerhaft löschen
- Kinder in den nächsten Jahrgang übernehmen
- Kinder ohne OGS-Betreuung erfassen

### Eltern informieren und Anfragen bearbeiten

- Eine Nachricht an Eltern senden
- Anfragen der Eltern prüfen und bearbeiten
- Eine Elternmitteilung veröffentlichen
- Einen Elternbrief versenden
- Eine Umfrage erstellen
- Einen Essensplan veröffentlichen

### Anmeldungen verwalten

- Eine Anmeldung vorbereiten
- Eingegangene Anmeldungen prüfen
- Anmeldungen exportieren
- Eine fehlerhafte Anmeldung löschen

### Team und Rechte verwalten

- Mitarbeitende anlegen und einladen
- Mitarbeitende und Rechte verwalten
- Einen Lehrkraft-Zugang vorbereiten

### Betreuung und Team planen

- Kalenderzeiträume anlegen
- Einen Betreuungsplan erstellen
- Eine Vertretung planen
- Einen Dienstplan erstellen
- Tageslisten erstellen
- Arbeitszeiten, Urlaub und Korrekturen prüfen

### Daten auswerten und verwalten

- Die Anwesenheit eines Tages prüfen
- Abwesenheiten auswerten
- Eine Statistik öffnen
- Eine Liste exportieren
- Dateien mit dem Team teilen
- Eine Abrechnung für DATEV vorbereiten

### Einstellungen und Geräte

- Den Betrieb der OGS einstellen
- Festlegen, was Eltern in moto sehen
- NFC-Geräte verwalten
- Ein Info-Display verwalten

### Wenn etwas nicht klappt

- Eine Person sieht einen Menüpunkt nicht

## 8. Anleitung für Eltern

Für das Eltern-Portal gibt es heute noch keine vollständige Hilfe. Die folgenden Anwendungsfälle
sind ein erster Kandidatenbestand. Sie müssen gegen die tatsächlichen Abläufe und Rückmeldungen von
Eltern geprüft werden.

### Einstieg und Konto

- Mein Eltern-Konto einrichten
- Bei moto anmelden
- moto als App auf dem Handy oder Tablet hinzufügen
- Zwischen Schulen wechseln
- Ein weiteres Kind verbinden
- Meine Zugangsdaten ändern

### Mein Kind und die Betreuung

- Angaben und Betreuungszeiten meines Kindes ansehen
- Eine Abwesenheit melden
- Eine Änderung der Betreuung anfragen

### Angaben ändern

- Angaben zu meinem Kind ändern
- Sorgeberechtigte verwalten

### Nachrichten und Informationen

- Nachrichten lesen und schreiben
- Informationen der OGS ansehen
- Benachrichtigungen auf meinem Gerät einschalten

### Angebote und Anmeldung

- Mein Kind anmelden
- Eine Anmeldung weiter bearbeiten

### Wenn etwas nicht klappt

- Ich kann mein Eltern-Konto nicht einrichten
- Mein Kind wird nicht angezeigt
- Eine Funktion ist für mich nicht verfügbar

## 9. Anleitung für Lehrkräfte

Für das Portal „moto schule“ gibt es bisher nur wenige Abläufe. Die folgenden Anwendungsfälle sind
ein erster Kandidatenbestand und müssen mit Lehrkräften geprüft werden.

### Einstieg

- Zugang zu moto schule einrichten
- Bei moto schule anmelden
- moto schule als App auf dem Handy oder Tablet hinzufügen

### Meine Klasse

- Meinen Klassentag ansehen
- Angaben zu einem Kind ansehen

### Aufsicht

- Meine Aufsichten ansehen
- Eine Aufsicht durchführen

### Nachrichten

- Nachrichten lesen und schreiben

### Wenn etwas nicht klappt

- Ich kann mich nicht anmelden
- Angaben zu meiner Klasse oder Aufsicht fehlen

## 10. Varianten innerhalb eines Anwendungsfalls

Die Seitenleiste bildet keine Kombinationen von Einstellungen ab. Varianten stehen direkt im
betroffenen Anwendungsfall.

Für den ersten Umbau werden nur diese drei Faktoren als übergebener Kontext berücksichtigt:

- NFC-Geräte werden genutzt oder nicht genutzt.
- Die Anwesenheit wird detailliert oder nur als anwesend und abwesend erfasst.
- Die OGS arbeitet mit festen Gruppen oder in offener Betreuung.

Eine Variante darf:

- einen Schritt ersetzen,
- einen kurzen Hinweis ergänzen,
- ein anderes Bild zeigen,
- erklären, warum ein Bereich nicht sichtbar ist.

Eine Variante darf keine Berechtigung vortäuschen und keine geheime Information offenlegen.

## 11. Reihenfolge für die weitere Ausarbeitung

1. Die Anleitung für Betreuungskräfte vollständig prüfen.
2. Die wichtigsten Anwendungsfälle mit OGS-Personal testen.
3. Die Anleitung für Leitungen prüfen.
4. Gemeinsame Anwendungsfälle zwischen beiden Rollen markieren.
5. Eltern-Abläufe separat untersuchen und prüfen.
6. Lehrkraft-Abläufe separat untersuchen und prüfen.
7. Erst danach die bestehenden 108 Hilfethemen übernehmen, teilen oder entfernen.

Bestehende Inhalte werden nicht automatisch übernommen. Jeder bisherige Inhalt braucht einen
bestätigten Anwendungsfall und eine passende Rolle.

## 12. Offene Entscheidungen

1. Wie wählt eine Person ihre Rolle beim Einstieg aus?
2. Wie wechselt eine Person mit mehreren Rollen die Anleitung?
3. Heißt die zweite Anleitung „Leitungen“ oder „Leitung und Verwaltung“?
4. Welche Anwendungsfälle sind für Betreuungskräfte wirklich häufig?
5. Welche Anwendungsfälle verursachen heute die meisten Rückfragen?
6. Welche Eltern-Abläufe sind bereits vollständig verfügbar?
7. Welche Aufgaben führen Lehrkräfte tatsächlich in moto schule aus?
8. Werden Problemlösungen innerhalb jedes Oberthemas oder gesammelt am Ende angezeigt?
9. Welche gemeinsamen Anwendungsfälle können dieselbe Seite verwenden?

## 13. Abweichung vom bisherigen Umbauplan

Der bisherige Umbauplan und ADR 0028 ziehen die Grenze des Inhaltsbestands am Portal und nicht an
der Rolle. Dieser Arbeitsentwurf sieht dagegen vier sichtbare, rollenbezogene Anleitungen und
Seitenleisten vor.

Nach der fachlichen Bestätigung müssen der Umbauplan und gegebenenfalls ADR 0028 angepasst werden.
Bis dahin ist dieses Dokument ein Diskussionsentwurf und keine abgeschlossene technische Vorgabe.
