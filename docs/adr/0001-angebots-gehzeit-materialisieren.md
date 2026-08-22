# Angebots-Gehzeiten werden aus Buchungsfenstern projiziert

Status: accepted; ersetzt die ursprüngliche Materialisierungsentscheidung in
dieser ADR für Angebots-Gehzeiten (#2412, #2415).

## Kontext

Betreuungsangebote (`enrollment.care_offerings`) tragen Gehzeiten pro
Wochentag. Die erste Umsetzung kopierte diese Werte als Zeilen mit
`source = care_offering` nach `schedule.student_pickup_schedules`.

Diese Kopie hat kein Datumsfenster. Eine heute genehmigte, aber erst später
wirksame Buchungsänderung kann deshalb am Stichtag nicht automatisch greifen.
Der Produktionsvorfall aus #2412 hat genau diesen Fehler bestätigt. Ein
täglicher Reparaturjob würde die Kopie nur nachträglich synchronisieren und
bliebe ein zusätzlicher Korrektheitsmechanismus.

Ein Datumsfenster direkt auf `student_pickup_schedules` passt nicht zum
bestehenden Modell: Die Tabelle erlaubt durch ihre Eindeutigkeit nur eine Zeile
pro Kind und Wochentag. Mehrere aufeinanderfolgende Buchungsfenster würden diese
Eindeutigkeit aufbrechen. Außerdem fehlen in Bestandsdaten Gehzeiten, die sich
nicht ohne fachliche Entscheidung aus Namen oder Regelterminen ableiten lassen.

## Entscheidung

- `enrollment.request_child_offerings` bleibt mit seinem halb offenen Fenster
  `[valid_from, valid_until)` die Quelle für die wirksame Buchung.
- Ein zentraler Schedule-Dienst projiziert die Angebots-Gehzeit für das
  angefragte Datum oder Datumsfenster. Alle Leser nutzen diese Grenze.
- Gespeicherte `staff`-Zeilen in `student_pickup_schedules` sind manuelle
  Overrides und haben Vorrang vor der Projektion.
- Alte `care_offering`-Zeilen werden beim Lesen ignoriert. Ein vollständiges
  Speichern des Gehplans entfernt solche Altzeilen; es gibt keine destruktive
  Datenmigration.
- Treffen mehrere wirksame Angebote auf denselben Wochentag, gilt wie bisher
  die späteste Gehzeit.
- Aktive Angebote, die als Betreuungstage zählen, brauchen für jeden gewählten
  Wochentag eine Gehzeit. Unvollständige Bestandsangebote werden im Katalog
  sichtbar und müssen beim nächsten Bearbeiten vervollständigt oder fachlich
  anders eingeordnet werden.
- Buchungs- und Angebotsänderungen stoßen weiterhin die abhängigen
  Auto-Abmeldungen und Live-Aktualisierungen an. Sie schreiben keine
  Angebotszeilen mehr.

## Folgen

- Zukunftswirksame Änderungen greifen am Stichtag ohne Scheduler-Lauf.
- Klassenlisten, Exporte, Kinderdetail, Betreuungsstatus, Auto-Abmeldungen und
  Slotlisten erhalten dieselbe datumsgenaue Gehzeit.
- Die Projektion lädt Buchungen und Angebote je Anfrage gebündelt; sie erzeugt
  keine Abfrage pro Kind oder Tag.
- `source` und `care_offering_id` bleiben vorerst im Schema, damit Altzeilen
  lesbar und kontrolliert abbaubar bleiben. Neue `care_offering`-Zeilen werden
  nicht mehr erzeugt.
- Eine spätere Konsistenzprüfung kann Abweichungen melden. Sie ist ein
  Betriebsnetz, nicht Teil der fachlichen Korrektheit.
