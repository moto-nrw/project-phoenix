# Komplett-Abmeldungen brauchen einen dauerhaften Abschluss

Status: accepted (#2424). Ergänzt die buchungsgeführte Betreuung aus ADR 0005
und den bestehenden Austritts- und Löschweg aus #2496.

## Kontext

Eine genehmigte Angebotsänderung kann alle betreuungszählenden Buchungen eines
Kindes beenden. Bisher endet dabei nur die Buchung. Das Kind bleibt in seiner
OGS-Betreuung und kann wegen uneinheitlicher Filter weiter in Klassen-, Gruppen-,
Planungs- und Erwartungslisten erscheinen. Der Zustand `inactive` löst das
nicht: Er wird erst aus dem Betreuungszeitraum abgeleitet und ist keine
fachliche Entscheidung.

Eine automatische Beendigung wäre ebenfalls falsch. Nicht jede Schule leitet
Betreuungstage aus Buchungen ab, und auch in einer buchungsgeführten Schule muss
die Schule zwischen dem Erhalt der Historie und der sofortigen Löschung wählen.

## Entscheidung

- Eine Komplett-Abmeldung gibt es nur bei eingeschaltetem
  `enrollment.bookings_authoritative`. Sie entsteht, wenn ein weiterhin
  betreutes Kind von mindestens einer wirksamen, betreuungszählenden Buchung auf
  keine wechselt. Angebote ohne Betreuungstage zählen nicht für den Auslöser.
- Auslöser sind genehmigte Elternanfragen, direkte Änderungen der Schule und das
  bekannte Auslaufen der letzten betreuungszählenden Buchung. Ein reguläres Ende
  von Buchungen und Betreuungszeitraum erzeugt keinen Abschluss.
- Eltern und Schule müssen den vollständigen Wegfall der Betreuungstage vor der
  Änderung ausdrücklich bestätigen. Diese Bestätigung wird mit Person und Zeit
  auditiert. Pflichtangebote und Mindestmengen blockieren eine bestätigte
  Komplett-Abmeldung nicht.
- Nach der Änderung entsteht höchstens ein dauerhafter offener oder geplanter
  Abschluss pro Kind. Er bleibt nach einem geschlossenen Browser erhalten und
  steht ohne persönliche Zuweisung allen Personen mit `users:delete` zur
  Verfügung. Es gibt keine zusätzliche E-Mail.
- Der Abschluss hat genau zwei Ergebnisse: OGS-Betreuung beenden oder Kind über
  den bestehenden Löschweg löschen. Ein bewusstes Behalten ist kein Ergebnis;
  ohne Entscheidung bleibt der Abschluss offen und wird nach Beginn des
  buchungslosen Zeitraums als überfällig hervorgehoben.
- Vor jeder Entscheidung wird der aktuelle Buchungsstand in derselben
  Transaktion erneut geprüft. Eine neue Buchung ohne Betreuungslücke macht den
  Abschluss gegenstandslos. Eine spätere Wiederaufnahme nach einer Lücke tut
  das nicht. Auch das Ausschalten des Buchungsmodus macht offene und geplante
  Abschlüsse gegenstandslos; abgeschlossene Austritte bleiben bestehen.
- Die Schule wählt den letzten Betreuungstag ausdrücklich. Er darf
  rückwirkend liegen, aber nicht nach dem Tag vor Beginn der Buchungslücke und
  nicht vor einer erfassten Anwesenheit. Ein geplanter Austritt kann vor seiner
  Wirksamkeit storniert werden und stellt den vorigen Stand wieder her. Nach
  seiner Wirksamkeit beginnt eine Wiederaufnahme ohne automatisch reaktivierte
  Gruppen, Angebote, Wochenpläne oder Zeiten.
- Ein Austritt beendet alle laufenden und geplanten Angebotsbuchungen,
  Aktivitätszuordnungen, Termine, Wochenpläne sowie Ankunfts- und Gehzeiten. Das
  gilt auch für Mensa, AGs und andere Angebote ohne Betreuungstage. Offene
  Elternanfragen bleiben bis zum letzten Betreuungstag entscheidbar, dürfen
  keine spätere Betreuung erzeugen und werden am Austrittsdatum mit Erklärung
  geschlossen. Eine neue Anmeldung bleibt getrennt.
- Die Prüfung nennt betroffene Angebote und wiederkehrende Wochenmuster konkret.
  Sie zeigt außerdem Folgen für Listen, offene Anfragen, Armband, Anwesenheit,
  gespeicherte Daten und eine spätere Wiederaufnahme. Die Löschoption warnt,
  dass sie sofort wirkt, auch bei einem späteren letzten Betreuungstag, und
  führt danach in die bestehende Löschprüfung.
- Bis zum Beginn der Buchungslücke nimmt das Kind regulär teil. Ab diesem Tag
  entfällt es aus Klassen-, Gruppen-, Planungs- und Erwartungslisten, auch wenn
  der Abschluss noch offen ist. Es bleibt in Kinderverwaltung, Aufgabenliste
  und tatsächlicher Live-Anwesenheit sichtbar. Nach dem Austritt ist es nur als
  beendete Betreuung auffindbar. `inactive` bleibt eine abgeleitete technische
  Markierung; Leser entscheiden anhand von Betreuungszeitraum, Buchungsmodus und
  Bezugsdatum.
- Eltern sehen nach der Genehmigung, dass die Abmeldung bestätigt ist und die
  Schule sie noch abschließt. Nach einem Austritt sehen sie das Enddatum und nur
  noch die Historie. Nach einer Löschung verschwinden Kind und kindbezogener
  Verlauf; es gibt keine zusätzliche E-Mail.

## Folgen

- Der bestehende Austrittsweg muss zusätzlich die Quellbuchungen beenden. Sonst
  kann ein späterer Angebotsabgleich bereits entfernte Zuordnungen wieder
  erzeugen. Gruppen, Wochenplan und Zeiten dürfen nach dem Ende nicht weiter als
  aktuelle Betreuung wirken.
- Alle fachlichen Leser müssen dieselbe datumsbezogene Teilnahmegrenze nutzen.
  Ein Filter nur auf `status` oder nur auf `alumnus` ist unzulänglich.
- Das Einschalten von `enrollment.bookings_authoritative` braucht eine Vorschau
  und wird blockiert, solange ein laufend betreutes Kind keine gebuchten
  Betreuungstage hat. Es gibt kein erzwungenes Überspringen.
- Die Einführung wird zuerst ausgeliefert. Danach wird die Auswirkung für OGS am
  Berg geprüft und der Buchungsmodus dort bewusst eingeschaltet.
- Der Hilfe-Guide muss den geführten Abschluss, seine Folgen, das Stornieren und
  die Wiederaufnahme im selben Änderungsumfang erklären.
