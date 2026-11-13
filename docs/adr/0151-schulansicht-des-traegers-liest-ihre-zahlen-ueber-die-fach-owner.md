---
status: proposed
---

# Schulansicht des Trägers liest ihre Zahlen über die Fach-Owner

Vorschlag vom 04.11.2026 zu [#5686](https://github.com/moto-ogs/project-phoenix/issues/5686).
Er ändert [ADR 0084](0084-traeger-portal-liest-ueber-eine-projektion-und-schreibt-in-der-schule.md)
Abschnitt 1 für diese Fläche und folgt dem Muster von
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

ADR 0084 hat „Lesefunktionen je Fach-Owner“ verworfen, weil die Admin-Einträge
über viele Pakete verstreut lägen und kein einzelner Ratchet den Filter prüfen
könnte. Der erste Grund entfällt hier (Punkt 2). Der zweite bleibt und ist der
Preis dieser Entscheidung (siehe Folgen).

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
   antworten gleich mit 403. So verrät die Route keine Schule. (Seit dem
   Nachtrag vom 10.11.2026: 404.)
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
- Den Filter dieser Abfragen sichert `api/traeger_school_figures_test.go` mit je
  einer fremden Schule, nicht der Ratchet von ADR 0084.
- Der Grund im Transaktionsregister zu `Overview.read` nennt weiter nur das
  Zugriffsprotokoll.
- `Betreuungskräfte anwesend` hängt an der Zeiterfassung. Eine Schule ohne
  Zeiterfassung zeigt 0 und keinen Schlüssel; die Seite sagt das.

## Nachtrag 06.11.2026: Stichtagszahlen ([#4673](https://github.com/moto-ogs/project-phoenix/issues/4673))

Das Träger-Portal zeigt die festgehaltenen Stichtagszahlen je Schule. Es liest sie
nach demselben Muster über ihren Owner: Organisation & Tenancy bietet
`TraegerMembershipQuery.ListTraegerBillingKeyDateCounts`. Die Fähigkeit prüft den
Träger wie `ListTraegerSchoolIDs` und liest die Zeilen des Operator-Berichts
(`platform.live_billing_key_date_counts`) nur für die nicht gelöschten Schulen
dieses Trägers, mit derselben Spaltenliste und derselben Zeilenabbildung wie der
Bericht. Sie läuft in `runRead` und damit in der Admin-Einheit der Projektion;
es entsteht keine neue Transaktionsstelle und keine Compose-Zeile. Die Projektion
behält nur Zeilen der Schulmenge, die sie selbst aufgelöst hat. Dass beide Ansichten
dieselben Zahlen zeigen, belegt `api/traeger_key_date_counts_test.go` mit einer
echten Erfassung über zwei Monate und einer Schule eines anderen Trägers.

## Nachtrag 10.11.2026: Fremde Schule antwortet mit 404 ([#6461](https://github.com/moto-ogs/project-phoenix/issues/6461))

Punkt 5 gab der Schulansicht 403, der Absprung (`POST /traeger/school-sessions`)
antwortet für dieselbe Schule mit 404. Beide tragen `traeger.school_not_found`.
Jetzt antwortet auch die Schulansicht mit 404: der Code sagt „nicht gefunden“,
das Repo antwortet auch sonst bei fremden Mandantenobjekten mit 404
(`iot.room_not_found`), und eine fremde und eine unbekannte Schule bleiben
gleich, also verrät die Route weiter keine Schule. Ein fehlendes Recht bleibt
403 mit eigenem Code. Fehlercode und Klasse bleiben gleich.
