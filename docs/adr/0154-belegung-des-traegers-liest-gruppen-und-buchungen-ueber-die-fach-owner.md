---
status: accepted
---

# Belegung des Trägers liest Gruppen und Buchungen über die Fach-Owner

Freigegeben von Yannick Wenger am 07.11.2026

Vorschlag vom 06.11.2026 zu [#4676](https://github.com/moto-nrw/project-phoenix/issues/4676).
Er folgt [ADR 0151](0151-schulansicht-des-traegers-liest-ihre-zahlen-ueber-die-fach-owner.md)
und ändert [ADR 0084](0084-traeger-portal-liest-ueber-eine-projektion-und-schreibt-in-der-schule.md)
Abschnitt 1 für diese Fläche. Er führt keinen neuen Owner ein.

## Kontext

Das Träger-Büro will die Belegung je Schule, OGS-Gruppe und Betreuungsform sehen
und exportieren, um zu planen. Diese Angaben liegen bei vier Owners: die
Zugehörigkeit eines Kindes zu einer Gruppe bei School Membership, der Name der
Gruppe bei School Structure, Betreuungsangebote und ihre Buchungen bei Care Plan
und die Verbindung einer Buchung (Anmeldekind) zum Kind bei Enrollment. Die
Gründe aus ADR 0151 gegen einen `read_projections`-Eintrag gelten unverändert.

## Entscheidung

1. **Lesen über die Owner wie in ADR 0151.** School Membership erweitert
   `SchoolRosters` um `MembershipGroupIDsByTenant`. School Structure bietet
   `TraegerGroupNames`, Care Plan `TraegerCareBookings`, Enrollment
   `BookedChildren`; dessen Abfrage nennt seit #5977 auch die laufenden
   Anmeldephasen. Jede Abfrage liegt im Postgres-Paket ihres Owners, filtert mit
   `tenant_id = ANY(?)` auf die übergebenen Schulen und verweigert ohne die
   Admin-Einheit der Projektion. Die Antworten tragen IDs, Zahlen und die Namen
   von Gruppen und Angeboten, keine Namen von Kindern.
2. **Definitionen.** Sie stehen an `traegerview.Occupancy`. Gezählt werden nur
   die Kinder der Stichtagszahl-Regel von heute, dieselben wie „Kinder gesamt“ der
   Schulansicht. Eine Betreuungsform ist ein Betreuungsangebot, das als Betreuung
   zählt (`counts_as_care`) und heute läuft oder heute mindestens eine gültige
   Buchung hat. Es läuft heute, wenn es aktiv ist und zu einer aktiven
   Anmeldephase gehört, deren Leistungszeitraum heute enthält. Enrollment nennt
   diese Phasen, Care Plan liefert ihre Angebote; so gibt es keine zweite
   Laufzeitregel und keinen Join über Owner (#5977). Ein laufendes Angebot ohne
   Buchung hat 0 Kinder und alle Plätze frei. Ein Kind zählt je Angebot einmal. Mittagessen und ähnliche Angebote ohne Betreuung zählen
   nicht. Ein Angebot nennt seine Teilnehmergrenze als Plätze.
3. **Kein Protokoll.** Die Antwort und der CSV-Export tragen zusammengefasste
   Zahlen. Nach ADR 0084 Abschnitt 7 steht das Lesen nicht im Zugriffsprotokoll.
4. **Wurzel und Register.** `MembershipGroupIDsByTenant` und `BookedChildren`
   nutzen nur eingebaute Typen und erfüllen die Ports direkt. Gruppennamen und
   gebuchte Angebote übersetzt die Wurzel in die Werte der Projektion
   (`api/traeger_occupancy_ports.go`); `traegerGroupNames.GroupsByTenant` steht
   deshalb im Pass-through-Register. Die Konstruktoren heben die Compose-Budgets
   von `modules/careplan/compose` und `modules/schoolstructure/compose` um je
   13 Zeilen und von `modules/enrollment/compose` um 6 Zeilen, zusammen um
   32 Zeilen.

## Verworfene Optionen

- **Gruppen und Buchungen in einer Abfrage der Projektion:** braucht wie in
  ADR 0151 eine neue Ausnahme in der Architekturprüfung für fünf fremde Tabellen.
- **Nur Angebote mit Buchung:** So stand es bis #5977. Das Träger-Büro sah ein
  laufendes Angebot ohne Buchung nicht und damit auch nicht seine freien Plätze.
- **Laufzeit in der Abfrage von Care Plan:** bräuchte einen Join auf
  `enrollment.phases`, die Tabelle von Enrollment.

## Folgen

- Die laufenden Phasen kosten keine eigene Abfrage: Enrollment liest sie im
  selben Statement wie die gebuchten Kinder. Das Budget
  `api.traeger.schools_occupancy` bleibt bei 8.
- Den Filter dieser Abfragen sichert `api/traeger_schools_occupancy_test.go` mit
  einer Schule eines anderen Trägers, nicht der Ratchet von ADR 0084.
- Eine Schule mit wochenplangeführter Betreuung kann Kinder ohne Buchung zeigen;
  die Seite nennt diese Kinder „ohne Betreuungsform“.
