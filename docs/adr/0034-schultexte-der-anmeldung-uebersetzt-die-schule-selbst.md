# Texte der Schule in der Online-Anmeldung übersetzt die Schule selbst

Status: accepted. Gilt für die Online-Anmeldung (öffentliches Formular,
Elternportal, Vorschau). Entstanden aus #3377.

## Kontext

Die Sprachwahl der Eltern übersetzt nur die Texte aus den Katalogen von
next-intl. Was eine Schule selbst schreibt, blieb deutsch: Name der
Anmeldephase, Fragen, Hinweise, Infotexte, Auswahlwerte, Zustimmungstexte,
Name und Beschreibung der Betreuungsangebote und der Gruppenname ihrer
Pflichtauswahl. Die Grundschule Burbach nennt
rund 60 Prozent Kinder mit Migrationshintergrund und braucht vor allem Russisch
und Ukrainisch.

Drei Fragen ließ das Issue offen: woher die Übersetzungen kommen, was bei einer
fehlenden Übersetzung passiert, und was gilt, wenn die Schule den deutschen Text
nach dem Übersetzen ändert.

## Entscheidung

1. **Die Schule pflegt die Übersetzungen von Hand.** Kein Übersetzungsdienst.
   Ein Dienst wäre ein neuer externer Empfänger von Elterntexten samt
   Zustimmungstexten, bräuchte einen Anbieter, einen Schlüssel und einen
   Auftragsverarbeitungsvertrag, und nichts davon ist beschlossen. Die Schule
   hat mehrsprachige Mitarbeitende und hat die manuelle Pflege im Gespräch
   ausdrücklich als tragfähig genannt. Das Datenmodell lässt maschinelle
   Vorschläge später zu: ein Vorschlag wäre ein vorbefülltes Eingabefeld, das
   ein Mensch bestätigt.
2. **Sprachen sind die Portalsprachen aus `backend/localization/locales.json`**,
   keine eigene Liste je Schule. Die Schule füllt nur, was sie braucht.
3. **Fehlt eine Übersetzung, lesen Eltern den deutschen Text**, und zwar je
   Text, nicht je Formular.
4. **Eine Übersetzung gilt nur, solange ihr deutscher Ausgangstext unverändert
   ist.** Jede Übersetzung speichert den deutschen Text, zu dem sie geschrieben
   wurde (`source`). Ändert die Schule den deutschen Text, liefert das Backend
   die Übersetzung nicht mehr aus; Eltern lesen den aktuellen deutschen Text,
   bis jemand die Übersetzung anpasst oder mit „Passt noch“ bestätigt. Der
   Grund sind die Zustimmungstexte: niemand soll einem Wortlaut zustimmen, der
   inhaltlich überholt ist. Die alte Übersetzung bleibt als Vorlage erhalten.
5. **Die Übersetzungen liegen am Objekt, nicht in einer eigenen Tabelle.**
   Form: `translations` = Sprache → Attribut → `{text, source}`. Im
   Formularschema liegt das im vorhandenen JSONB der Felder, Auswahlwerte und
   Rechtsblöcke und wird mit jeder Schemaversion mitversioniert. Anmeldephasen
   und Betreuungsangebote bekommen eine JSONB-Spalte (Migration 1.15.407).
   Kopieren, Duplizieren und Verlängern nehmen die Übersetzungen dadurch ohne
   eigenen Code mit.
6. **Das Frontend wählt die Sprache.** Elternseitige Antworten tragen alle
   noch gültigen Übersetzungen ohne `source`; `enrollment-translations.ts`
   löst je Text auf. Das folgt ADR 0006 („übersetzt wird im Frontend“): das
   Backend erfährt die Sprache der Eltern auf diesem Weg nicht, und ein
   Sprachwechsel braucht keinen neuen Abruf.
7. **Der Zustimmungsnachweis hält die angebotenen Übersetzungen fest.** Der
   Snapshot der Rechtsblöcke einer Anmeldung speichert neben dem deutschen
   Wortlaut die zu diesem Zeitpunkt gültigen Übersetzungen. Welche Sprache die
   Person gelesen hat, weiß der Server nicht; festgehalten ist, welche
   Wortlaute zur Wahl standen.

## Grenzen

- Der Gruppenname der Pflichtauswahl ist zugleich der Schlüssel, der Angebote
  zu einer Regel bündelt. Übersetzt wird deshalb nur, was Eltern lesen; der
  deutsche Name am Angebot bleibt unangetastet. Tragen Angebote einer Gruppe
  verschiedene Übersetzungen, gilt die erste in Formularreihenfolge.

- Care Plan besitzt die Zeilen der Betreuungsangebote und darf Typen der
  Anmeldung nicht importieren. Das Übersetzungsdokument läuft dort als
  unausgewertetes JSON durch; geprüft und ausgewertet wird es ausschließlich
  in `services/enrollment/care_offering_translations.go`.
- Das Modul Anmeldung darf `localization` nicht importieren. Es prüft deshalb
  nur die Form des Sprachcodes und lehnt Deutsch ab. Ein Code außerhalb der
  Portalsprachen bleibt wirkungslos, weil ihn niemand anfragt.
- Nicht übersetzbar bleiben die Rechtstexte aus den Einstellungen
  (`enrollment.legal_*_text`), solange eine Vorlage sie nicht als eigenen Block
  führt, hinterlegte PDF-Dateien, und die Angebotsnamen außerhalb des
  Anmeldeformulars (Buchungen und Kurse im Elternportal).
- Selbst geschriebene Elternnachrichten haben dasselbe Grundproblem. Sie sind
  bewusst nicht Teil dieser Entscheidung.
