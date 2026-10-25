---
status: accepted
---

# Träger-Projektion bekommt ihre Komposition und ihre Lesegrenze

Entscheidung vom 24.10.2026 zu [#4649](https://github.com/moto-nrw/project-phoenix/issues/4649),
erster Schnitt von [ADR 0084](0084-traeger-portal-liest-ueber-eine-projektion-und-schreibt-in-der-schule.md)
Abschnitt 1. ADR 0084 hat den Owner `traeger-view` beschlossen. Diese ADR hält
fest, was sein erster Code in den Registern ändert, damit kein Register
stillschweigend wächst (ADR 0057 Punkt 5, ADR 0070).

## Entscheidung

1. `modules/traegerview/compose` baut die Projektion: die Leseanwendung, die
   Postgres-Lesungen und die Schulmenge, die Organisation & Tenancy liefert.
   Die Komposition hält keine Logik. Ihr Budget in
   `moduleComposeBudgets` ist ihr gemessener Umfang, 22 Zeilen; die Summe
   `moduleComposeBudgetTotal` steigt um genau diese 22 Zeilen. Kein anderer
   Eintrag wächst.
2. Das Transaktionsregister erhält genau zwei administrative Stellen:
   die Bindung `tenant.WithinAdmin` in der Komposition und den Aufruf in
   `Overview.Schools`. `test/traeger_view_transaction.go` lässt nur diese
   beiden Schlüssel mit ihren festen Gründen durch; jede andere neue oder
   umformulierte Stelle bleibt vom Schrumpf-Vergleich erfasst. Damit gilt
   ADR 0084: Admin-Einträge mit Träger-Bezug nur aus der Projektion und aus
   Anmeldung und Sitzung in Identity & Access. Anmeldung und Sitzung nutzen
   die vorhandenen Einheiten `globalIdentityRead`,
   `persistSessionInTransaction` und die Refresh-Rotation; sie brauchen
   keine neue Stelle.
3. `TestTraegerViewQueriesFilterOnTheTraegerSchools` prüft jede Query im
   Postgres-Paket der Projektion: nur `NewRaw` über eine Paketkonstante mit
   `= ANY(?0)`, die Schulmenge des Trägers als `pgdialect.Array` an erster
   Stelle. Jeder andere Query-Baustein und jede berechnete Anweisung fällt
   durch. Ein Gegentest belegt, dass der Scanner diese Formen erkennt.

4. Das alte Org-Gerüst fällt, soweit es lief (ADR 0084 Abschnitt 3):
   `TenantMiddleware` weist Träger-Token ab, die automatische
   Admin-Transaktion für Org-Token, der Org-Zweig der Slug-Auflösung, der
   Org-Zweig der Kontenverwaltung samt `ListManageableSchoolIDs` und der
   Org-Fall der Passkey-Anmeldung entfallen. Dadurch fällt eine
   Admin-Stelle aus dem Transaktionsregister, und
   `AccountAdministration.FindManageableAccount` wird ein reiner
   Transaktionsrahmen (Form aus ADR 0075). Das Pass-through-Budget von
   `modules/identityaccess` steigt deshalb um genau eins; die genaue
   Methodenevidenz steht in `passthrough.json`.

## Folgen

- Weitere Flächen des Träger-Portals (Übersicht, Anmeldeliste, Berichte,
  Zugriffsprotokoll) lesen über dieselbe Einheit und denselben Filter. Eine
  neue administrative Stelle oder ein wachsendes Budget braucht wieder eine
  eigene ADR.
- Die Policy erhält nur Regeln mit Quelle im neuen Owner `traeger-view`
  sowie `traeger-view.http.identity-access.public` für die Träger-Rechte,
  die Identity & Access zu Rollen bündelt (ADR 0084 Abschnitt 4).
