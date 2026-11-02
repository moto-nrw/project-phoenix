---
status: accepted
---

# Schulferien gehören dem Schulkalender, der Ferienhinweis dem Betreuungsplan

Entscheidung vom 2026-10-31 für
[#5345](https://github.com/moto-nrw/project-phoenix/issues/5345). Sie fügt
keinen Owner und keine Policy-Regel über Owner-Grenzen hinzu. Sie weist zwei
Tabellen dem bestehenden Owner `school-calendar` zu, registriert dessen neues
Adapter-Paket `internal/adapters/openholidays`, nimmt eine administrative
Transaktionsgrenze auf und hebt die Compose-Budgets von
`modules/schoolcalendar/compose` um genau 11 und von
`modules/timetable/compose` um genau 6 Zeilen an (ADR 0057 Punkt 5).

## Kontext

Die Terminplanung überspringt gesetzliche Feiertage und Schließtage, kennt
aber keine Schulferien. Ohne eingetragene Schließtage laufen Regeltermine
deshalb in den Ferien weiter. Ferien sind aber keine Schließtage: Manche
Schulen betreuen in den Ferien bewusst. Die Planungsregel bleibt daher
unverändert; die Schule soll nur rechtzeitig sehen, dass in den nächsten
Ferien Regeltermine geplant sind, und selbst entscheiden.

Die Ferien je Bundesland liefert die öffentliche OpenHolidays API. Sie darf
keine Seite blockieren und darf in Tests und im Seeder nicht aufgerufen
werden.

## Entscheidung

1. `calendar.school_vacations` gehört `school-calendar`. Die Tabelle hat keine
   Schule: dieselben Ferien gelten für jede Schule eines Bundeslands. Jede
   Schule darf sie lesen, schreiben darf nur die administrative Rolle.
2. Ein täglicher Worker-Job `school-vacation-sync` liest in einer Anfrage die
   Ferien aller 16 Bundesländer vom 1. August des laufenden Schuljahres bis
   zwei Jahre nach heute (höchstens 1095 Tage, die Grenze der API) und
   ersetzt je Bundesland die gespeicherten Zeilen dieses Fensters. Die Anfrage
   läuft vor der Transaktion. Schlägt sie fehl, bleibt alles Gespeicherte
   stehen und der Fehler steht im Log. `compose.NewSchoolVacations` öffnet
   dafür `tenant.WithinAdmin` (Register-Eintrag in
   `backend/test/school_vacation_transaction.go`).
3. `calendar.school_vacation_acknowledgements` gehört `school-calendar` und
   ist mandantengetrennt (RLS). Eine Zeile hält fest, dass die Schule in einem
   Ferienfenster bewusst Betreuung plant. Das Fenster heißt Bundesland, erster
   und letzter Tag, damit ein späterer Abgleich die Entscheidung nicht
   verliert. Die Bestätigung braucht `schedules:manage`.
4. Den Hinweis berechnet Timetable. Er zählt geplante Regeltermine ab heute
   in Ferien, die innerhalb von acht Wochen beginnen oder laufen. Nicht
   gezählt werden gesetzliche Feiertage, Schließtage, Serien mit
   `include_closing_days` und Serien in einem von der Schule eingetragenen
   Ferienzeitraum (`period_type` `holiday`). Timetable liest die Ferien, Tage
   und Zeiträume über eigene Ports (`SchoolVacationDays`,
   `SchoolVacationReader`), die die Wurzel mit dem Schulkalender bindet. Er
   ändert keinen Termin. Bereinigen kann die Schule nur über die bestehende
   Sammelabsage aus #3594.
5. `compose.NewSchoolVacations` (11 Zeilen mit Import) und
   `compose.NewSchoolVacationHints` (6 Zeilen) sind die Compose-Zeilen, die
   diese Entscheidung genehmigt.

## Betrachtete Optionen

1. **Ferien beim Aufruf der Seite abrufen.** Abgelehnt: Jede Seite hinge an
   der Verfügbarkeit der API, und Tests bräuchten Netz.
2. **Ferien automatisch als Schließtage behandeln.** Abgelehnt: Bewusste
   Ferienbetreuung würde still verschwinden, und vorhandene Termine samt
   Kinderlisten dürfen nur auf ausdrückliche Bestätigung der Schule weichen.
3. **Bestätigung als Schul-Einstellung.** Abgelehnt: Einstellungen sind für
   Werte, die eine Schule bewusst pflegt; eine Liste bestätigter Fenster mit
   Person und Zeitpunkt ist ein Datensatz.
4. **Hinweis im Schulkalender berechnen.** Abgelehnt: Der Schulkalender
   müsste Termine von Timetable zählen. Timetable konsumiert den Schulkalender
   bereits über eigene Ports (#3594).

## Folgen

- Die Schul-Löschung, der Datenkatalog und das Aufbewahrungsregister kennen
  beide Tabellen; die Bestätigungen gehen mit der Schule.
- Der Seeder ruft die API nicht auf. Er legt für die Entwicklung eigene Ferien
  des Bundeslands der Demo-Schule an.
- Ändert ein Land seine Ferien, gilt ab dem nächsten Abgleich das neue
  Fenster; eine Bestätigung für das alte Fenster greift dann nicht mehr, und
  der Hinweis erscheint für das neue Fenster erneut.
