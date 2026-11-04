---
status: proposed
---

# Schulansicht des Trägers liest ihre Zahlen über die Fach-Owner

Vorschlag vom 04.11.2026 zu [#5686](https://github.com/moto-nrw/project-phoenix/issues/5686).
Er ergänzt [ADR 0084](0084-traeger-portal-liest-ueber-eine-projektion-und-schreibt-in-der-schule.md)
Abschnitt 1 und folgt dem Muster von
[ADR 0109](0109-traeger-zugriffsprotokoll-liest-die-audit-spur-ueber-den-owner.md)
und [ADR 0137](0137-traeger-zugriffsprotokoll-zeigt-die-ferienanmeldungen-ueber-enrollment.md).
Er führt keinen neuen Owner ein.

## Kontext

Die Schulansicht zeigt je Schule Kinder gesamt, anwesende und kranke Kinder,
Betreuungskräfte und den Betreuungsschlüssel. Diese Zahlen liegen bei vier Owners.
ADR 0084 sieht dafür einen `read_projections`-Eintrag vor. Die Architekturprüfung
verweigert einem bestehenden Paket aber jede neue Lesefreigabe
(`readProjectionLoosenings`). Der Filter-Ratchet von ADR 0084 prüft nur das
Postgres-Paket der Projektion.

## Entscheidung

1. **Lesen über die Owner.** School Membership (`SchoolRosters`), Student Presence
   (`PresentStudentCounts`), Care Plan (`SickStudents`) und Workforce (`OnSiteStaff`)
   bieten je eine kleine öffentliche Fähigkeit. Sie liefert nur IDs oder Zahlen,
   keine Namen. Die Abfragen liegen im Postgres-Paket des Owners. Jede filtert mit
   `tenant_id = ANY(?)` auf die übergebenen Schulen.
2. **Keine neue Admin-Stelle.** Die Abfragen laufen in der Admin-Einheit der
   Projektion (`Overview.read`), ohne sie verweigert der Adapter das Lesen.
   `transaction_boundaries.json` ändert sich nicht. Die Projektion prüft vorher,
   dass die Schule zum Träger der Sitzung gehört.
3. **Definitionen.** Sie stehen an `traegerview.SchoolFigures`. Kinder gesamt nutzt
   die Regel der Stichtagszahl (#2791). Ein Test belegt dieselbe Zahl wie im
   Bericht des Operators. Betreuungsschlüssel heißt: anwesende Kinder je
   Betreuungskraft vor Ort. Homeoffice zählt nicht.
4. **Kein Protokoll.** Die Antwort trägt zusammengefasste Zahlen und die Kontaktdaten
   der Schule. Nach ADR 0084 Abschnitt 7 steht das Lesen nicht im Zugriffsprotokoll.
5. **Fremde Schule.** Eine Schule eines anderen Trägers und eine unbekannte ID
   antworten gleich mit 403. So verrät die Route keine Schule.
6. **Wurzel und Register.** Die Signaturen nutzen nur eingebaute Typen. Die Fähigkeiten
   der Owner erfüllen die Ports von `traeger-view` also direkt, ohne Übersetzung in
   der Wurzel und ohne neuen Eintrag im Pass-through-Register. Die Konstruktoren heben
   die Compose-Budgets von `modules/schoolmembership/compose` um 6 Zeilen und von
   `modules/studentpresence/compose`, `modules/careplan/compose` und
   `modules/workforce/compose` um je 13 Zeilen, zusammen um 45 Zeilen.

## Verworfene Optionen

- **`read_projections`-Eintrag nach ADR 0084:** braucht eine neue Ausnahme in der
  Architekturprüfung für acht fremde Tabellen.
- **Lesen in der Tenant-Transaktion jeder Schule über die Dashboards der Schule:**
  lädt je Aufruf das ganze Dashboard und eignet sich nicht für viele Schulen.

## Folgen

- Die Übersicht über alle Schulen (#4674) ruft dieselben Fähigkeiten mit der ganzen
  Schulmenge auf.
- Der Filter dieser Abfragen hängt an den Tests der Owner, nicht am Ratchet von
  ADR 0084.
- `Betreuungskräfte anwesend` hängt an der Zeiterfassung. Eine Schule ohne
  Zeiterfassung zeigt 0 und keinen Schlüssel; die Seite sagt das.
