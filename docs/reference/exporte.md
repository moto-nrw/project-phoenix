# Exporte: Entscheidungsliste

Stand: 23. Oktober 2026, Issue #2223. Diese Liste hält für jede fachliche
Export- und Druckfunktion fest, ob sie zusammengelegt, angeglichen oder
bewusst so gelassen wird. Importvorlagen, iCalendar und die statischen
Hilfe-PDFs sind nicht Teil davon (Nicht-Ziel in #2223).

## Regeln für alle Exporte

1. **Layoutfamilien.** Jedes Dokument gehört zu genau einer Familie: ausführlicher
   Block pro Datensatz, kompakte Tabelle oder Matrix (Pläne). Alle laufen durch
   `backend/modules/documentrendering/lists`.
2. **Bedienung.** Ein Export ohne Optionen steht im Menü der Seite
   (`OverflowMenu`), ein Eintrag pro Format. Ein Export mit Optionen (Vorlage,
   Zeitraum, Filter) öffnet den gemeinsamen Export-Dialog mit Formatwahl,
   Filteranzeige und Druck-Option. Der gemeinsame Dialog liegt im UI-Kit
   (`frontend/src/components/ui/export-dialog.tsx`); `useExportRun` führt ihn aus.
3. **Formatnamen.** In der Oberfläche heißen die Formate „PDF“, „Word“, „Excel“
   und „CSV“.
4. **Drucken.** „Drucken“ öffnet die PDF in einem neuen Tab und ruft den
   Druckdialog des Browsers auf (`openBlobForPrint` in
   `frontend/src/lib/file-download.ts`). Der Tab öffnet sich vor dem Laden,
   sonst blockiert der Browser ihn.
5. **Download.** Jeder Download läuft über `downloadBlob` und
   `filenameFromDisposition` aus `frontend/src/lib/file-download.ts`.
6. **Unbekanntes Format.** Das Backend lehnt ein unbekanntes Format mit 400 ab.
   Ein fehlendes Format bekommt den dokumentierten Standard des Endpunkts.

## Entscheidungen je Export

| # | Export | Einstieg | Entscheidung |
|---|---|---|---|
| 1 | Kinderlisten (8 Vorlagen) | Kindersuche, Exportzentrale | **Angeglichen.** Nutzt den gemeinsamen Export-Dialog. Die Vorlage „Klassenliste“ hat Spalten und Zellformat der Anmelde-Klassenliste (#16, angeglichen mit #4749). |
| 2 | Geburtstagsliste Personal | Exportzentrale | **Angeglichen.** Gemeinsamer Export-Dialog statt der Kopie des Kinderlisten-Modals. |
| 3 | Raumbelegung „Wer ist wo“ | Räume, Exportzentrale | **Bewusst so lassen.** Die Fachseite exportiert die gefilterten Räume, die Exportzentrale alle Räume mit „Unterwegs“. Das ist gewollt: die Zentrale ist der Einstieg ohne Vorauswahl. |
| 4 | Notfallliste | Notfallliste, Exportzentrale | **Angeglichen.** Der Druck-Tab öffnet sich jetzt vor dem Laden; vorher konnte der Browser ihn blockieren. |
| 5 | Tagesauswertung | Tagesauswertung | **Angeglichen.** „Drucken“ öffnet den Druckdialog statt nur den PDF-Viewer; Word ist im Menü verfügbar, das Backend konnte es schon. |
| 6 | Anwesenheitshistorie | Anwesenheitsprotokoll eines Kindes | **Angeglichen.** Gemeinsamer Export-Dialog mit Zeitraum (`from`/`to`), begrenzt auf die sichtbaren Tage bis heute. |
| 7 | Tageslisten | Tageslisten, Link aus der Exportzentrale | **Bewusst so lassen.** Eigene Konfiguration auf der Seite; Drucken folgt schon Regel 4. |
| 8 | Dienstplan | Dienstplan | **Angeglichen.** `PlanExportModal` nutzt den gemeinsamen Export-Dialog; Matrix-Layout bleibt. |
| 9 | Betreuungsplan, Vertretung | Betreuungsplan, Vertretung | Wie #8. |
| 10 | Eigene Zeiterfassung | Zeiterfassung | **Angeglichen** im Backend: unbekanntes Format gibt 400 statt still CSV. Die Oberfläche (`StaffExportButton`) nutzt den gemeinsamen Export-Dialog und `/api/staff/{id}/time-tracking/export`. Die alte Route `frontend/src/app/api/time-tracking/export` ist entfernt. |
| 11 | Zeiterfassung einer Person | Personal, Tab Zeiterfassung | Wie #10. |
| 12 | Zeitkonten, DATEV | Personal | **Bewusst so lassen.** Eigene Formate (DATEV) und SFTP-Übergabe; kein PDF, kein Druck. Lehnt unbekannte Formate schon ab. |
| 13 | Anmeldungen der Phase | Anmeldephase | **Angeglichen.** Teilt sich die Block-Bausteine mit #14 und #15 (#4750). Das Menü nutzt `OverflowMenu`. |
| 14 | Anmeldungen eines Kindes | Kind, Tab Anmeldungen | Wie #13. Die Einstiege nutzen `OverflowMenu`. |
| 15 | Anmelde-Auswertung | Anmeldephase | Wie #13. Die kompakte Variante (#2215) nutzt den Tabellen-Builder der Anmelde-Klassenliste (#4750). |
| 16 | Klassenlisten der Phase | Anmeldephase | **Getrennt lassen, Layout angeglichen** (#4749). Siehe unten. |
| 17 | Statistik | Statistik | **Bewusst so lassen.** Menü pro Abschnitt folgt Regel 2. |
| 18 | Mittagessen-Teilnehmer | Essensplan | **Angeglichen.** Die Einstiege nutzen `OverflowMenu`. |
| 19 | Bankverbindungen | Eltern, Bankverbindungen | **Angeglichen.** Gemeinsamer Export-Dialog statt eigener Formatwahl. |
| 20 | Einverständnisse | Elternbrief, Rückmeldestand | **Angeglichen.** Die Einstiege nutzen `OverflowMenu`. |
| 21 | Übersicht je Abrechnungslauf für die Kämmerei | Träger-Portal, Abrechnungslauf | **Angeglichen.** Kompakte Tabelle über `lists.Renderer` (PDF, Excel, CSV), Einstieg im `OverflowMenu` der Karte unter „Exportieren“ wie im Schul-Portal (#4672, #5240, ADR 0122). |

Die Nummern 1 bis 16 entsprechen der Bestandsaufnahme in #2223. 17 bis 21 sind
seitdem dazugekommen.

## Klassenliste: warum zwei

Die Kinderlisten-Vorlage „Klassenliste“ liest die Stammdaten der laufenden
Betreuung. Die Klassenliste der Anmeldephase liest Anmeldungen für das
kommende Schuljahr; diese Kinder gibt es in den Stammdaten oft noch nicht.
Eine gemeinsame Datenquelle gibt es also nicht. Beide behalten ihren Endpunkt,
teilen sich aber Spalten, Wochentagszelle („14:30 Uhr, wird abgeholt“) und den
Tabellen-Builder.

Der Tabellen-Builder liegt in
`backend/modules/documentrendering/records/class_roster.go`: gemeinsame
Spalten (Name, Klasse, Montag bis Freitag), die Wochentagszelle und die
Abschnitte je Klasse („alle Klassen“). Die Geh-/Abholregelung je Tag kommt aus
`departure.DayRules`. Die Anmelde-Klassenliste hängt ihre Erziehungsberechtigten
als eigene Spalte an. Die Kinderlisten-Vorlage behält Gruppe,
Betreuungs-/Anmeldestatus, Betreuungstage und Geh-/Abholweise hinter den
gemeinsamen Spalten. Es kommen keine neuen Datenfelder hinzu.

Die Wochentagszelle übernimmt das Format der Anmelde-Klassenliste: Abholzeit
und taggenaue Geh-/Abholregelung stehen zusammen, damit die Liste für die
Abholung lesbar bleibt. Tage ohne Betreuung bleiben „—“. Die
Kinderlisten-Vorlage hat keine Ankunftszeit mehr in
der Zelle; die bleibt der OGS Wochenliste vorbehalten.

## Anmelde-Exporte: gemeinsame Bausteine

Anmeldungen der Phase, Anmeldungen eines Kindes und die Anmelde-Auswertung
(#13 bis #15) bauen ihre Dokumente aus denselben Teilen in
`backend/modules/enrollment/http/export_parts.go` (#4750): Seitenkopf mit
Datenschutz-Fußzeile, Formatweiche (PDF und Word als Blöcke, Excel als
Tabelle), der Block „ein Kind mit seinen Anmeldeangaben“ und ein
Spaltenkatalog für Excel. Jeder Export liest weiter seine eigenen Daten und
wählt seine Felder; der Inhalt der Dokumente ist unverändert. Die kompakte
Auswertung und die Klassenliste der Phase teilen sich die
Klassenlisten-Tabelle; die Auswertung beschriftet die Klassenspalte weiter mit
„Zielklasse“.
