---
status: accepted
---

# Mandantenrückgabe hat einen eigenen Leseadapter

Slice 3838-a setzt die bereits in ADR 0092 beschlossene administrative
Leseausnahme für #3838 um. Der ausführende Adapter liegt getrennt von den
schreibenden Persistenzadaptern bei Organisation & Tenancy. Er bekommt einen
neuen, genau benannten Policy-Leseeintrag für die bekannten Katalogobjekte,
keinen neuen Owner. Die einzige dynamische SQL-Ausnahme ist dessen
`exportRun.exportTable`-`QueryContext` mit bestehendem Policy-Leseeintrag;
andere Calls und Schreibrechte bleiben unverändert verboten. Ein Test
prüft jede Katalogtabelle mit zwei Schulen und alle Spaltenausschlüsse.

Das Transaktionsregister erhält ausschließlich
`tenant_export.go:*tenantExporter.start:e.db.BeginTx#1` mit festem
ADR-bezogenem Grund. Die Schrumpf-/Relokationsregel bleibt für alle anderen
Grenzen unverändert; geänderte Gründe oder fremde Stellen werden nicht
freigegeben. Die eigene Grenze ist erforderlich, weil ein normaler
HTTP-Admin-Transaktionsrahmen weder die lange Snapshot-Lektüre noch deren
Read-only-Isolation garantiert.

Die NOLOGIN-Rolle `phoenix_tenant_export` kann über `pg_read_all_data` lesen,
auch in später angelegten Schemas, und RLS für diesen administrativen Pfad
umgehen. Sie hat keine Schreibrechte und wird nicht der HTTP-Rolle zugewiesen.
Die gesamte DB-Lektüre läuft zusätzlich in `REPEATABLE READ READ ONLY`.
Tabellen-, Personen-/Schulbezüge und Geheimnisausschlüsse bleiben Aufgabe des
einzigen Katalogexecutors. Die Rolle ist kein Kundenzugang. Ein breiter
lesender Wartungspfad ist hier bewusst akzeptiert; er ist weder ein
ungefilterter Export noch ein frei ausführbares Superuser-Skript.

Die neue Komposition bindet ausschließlich DB-Pool und die am Root gelieferten
SHA-256-Funktionen von Security Runtime. Sie führt kein SQL aus und baut
keinen fremden Servicegraphen. Dafür steigt der konkrete Kompositionsumfang
von `modules/organizationtenancy/compose` von **253 auf 271 LOC**, das
Gesamtbudget von **12384 auf 12402 LOC**. Die 18 LOC sind exakt der neue
Konstruktor, ohne Reserve und ohne verschobene Bestandslogik. Dies ist die
begrenzte Erweiterung nach ADR 0057 Punkt 5; andere Budgets steigen nicht.

Verworfen: den Export an den bisherigen schreibenden Adapter anzuhängen,
allgemeine Cross-Owner-Rechte zu öffnen, fremde Implementierungen zu
importieren oder einen zweiten Datenkatalog einzuführen. Dateien brauchen
einen stabilen lokalen Stand zusätzlich zum DB-Snapshot. Der technische
Fertig-Marker ersetzt weder fachliche Ergänzungen noch Verschlüsselung und
produktive Nachweise aus dem Rückgabe-Runbook.
