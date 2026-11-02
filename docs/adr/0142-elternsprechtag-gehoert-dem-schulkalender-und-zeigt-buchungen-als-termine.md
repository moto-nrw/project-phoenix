---
status: accepted
---

# Der Elternsprechtag gehört dem Schulkalender und zeigt Buchungen als Termine

Entscheidung vom 2026-10-27 für
[#4801](https://github.com/moto-nrw/project-phoenix/issues/4801). Sie fügt
keinen Owner, keine Policy-Regel und keine neue Ausnahme im
Command-Effects-Inventar hinzu. Sie weist zwei Tabellen dem bestehenden Owner
`school-calendar` zu und hebt das Compose-Budget von
`modules/schoolcalendar/compose` um genau 7 Zeilen an (ADR 0057 Punkt 5).

## Kontext

Eine Lehrkraft bietet für eine ihrer Klassen einen Elternsprechtag an: Tag,
Beginn, Ende, Länge eines Gesprächs, Pausen und Raum. Familien buchen je Kind
ein freies Zeitfenster, verschieben oder sagen es bis 24 Stunden vorher ab.
Ein Fenster darf auch bei gleichzeitiger Buchung nur ein Kind halten. Jede
Buchung soll im Elternkalender und im Kalender von moto schule stehen, und
beide Seiten sollen von Buchung und Absage erfahren.

Die Logik für moto schule und den Elternkalender liegt im
Schulkalender-Portal (`modules/schoolcalendar/portal`, Owner
`school-calendar`). Es legt Termine bereits über die Capability von
Appointments an, ändert und storniert sie und verschickt dabei Mail und Push
an die Eltern. Diese Schreibwege sind im Inventar als bekannte Schuld
(ADR 0083) erfasst. Jede neue schreibende Methode von `school-calendar` nach
`appointments` wäre eine neue Ausnahme.

## Entscheidung

1. `calendar.parent_conferences` und `calendar.parent_conference_bookings`
   gehören dem Owner `school-calendar`. Sein Postgres-Adapter speichert Tage
   und Buchungen; die öffentliche Schnittstelle
   `schoolcalendar.ParentConferences` prüft Fenster, Pausen und
   Gesprächslänge und leitet die Zeitfenster ab. Die Fenster werden nicht
   einzeln gespeichert.
2. Zwei Unique-Constraints sind der Schutz gegen Doppelbuchung: ein Fenster
   je Tag hält ein Kind, ein Kind hält ein Fenster je Tag.
3. Eine Buchung ist zusätzlich ein Termin der Lehrkraft an die Eltern des
   Kindes (Zieltyp `parents_by_student`, Art `elternsprechtag`). Das Portal
   legt ihn mit den bestehenden Capability-Methoden an, verschiebt und
   storniert ihn. Mail und Push an die Eltern laufen über die bestehenden
   Wege. Den Termin kann niemand von Hand ändern; Änderungen laufen nur über
   den Sprechtag.
4. Die Lehrkraft erfährt von Buchung, Verschiebung und Absage über den
   bestehenden Zustellweg als Hinweis in moto schule
   (`school_parent_conference_booking`), nur mit ihrer Zustimmung.
5. Am Ende des Schuljahres (1. August, `schoolcalendar.DefaultSchoolYear`)
   löscht der nächtliche Lauf die Tage mit ihren Buchungen über den
   Schulkalender und die zugehörigen Termine über die Aufbewahrung von
   Appointments, jeweils im eigenen Owner.
6. `compose.NewParentConferences` sind die 7 Compose-Zeilen, die diese
   Entscheidung genehmigt.

## Betrachtete Optionen

1. **Tabellen bei Appointments.** Abgelehnt: Das Portal hätte über neue
   schreibende Methoden in einen fremden Owner geschrieben; das Inventar
   verlangt dafür eine neue Ausnahme.
2. **Buchungen nur als eigene Kalenderquelle ohne Termin.** Abgelehnt: Mail-
   und Push-Texte, Abo-Feed und Erinnerungen hätten einen zweiten Weg
   gebraucht.
3. **Ein neuer Workflow für den Sprechtag.** Abgelehnt für diesen Schritt:
   Er würde die bestehende Schuld des Portals nicht auflösen, sondern einen
   zweiten Koordinator daneben stellen. Die Verlagerung des ganzen Portals in
   einen Workflow bleibt der Folgeschritt aus ADR 0083.

## Folgen

- Die Schul-Löschung, der Datenkatalog und das Aufbewahrungsregister kennen
  beide Tabellen.
- Familien ohne OGS-Platz (#4803) erreicht der Sprechtag, sobald ihre Kinder
  als normale Kind-Datensätze mit Klasse und Eltern-Beziehung vorliegen; der
  Sprechtag liest nur Klasse und Beziehung.
