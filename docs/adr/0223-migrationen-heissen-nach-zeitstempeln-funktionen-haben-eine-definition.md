---
status: accepted
---

# Migrationen heißen nach Zeitstempeln, Funktionen haben eine Definition

Entscheidung vom 2026-11-13 zu Forschungsfrage 5 in
[#3301](https://github.com/moto-ogs/project-phoenix/issues/3301). Sie ändert,
wie neue Migrationen benannt und wie Funktionen geändert werden. Sie fügt
keinen Owner und keine Policy-Regel hinzu.

## Kontext

Jede Migration trug eine laufende Nummer (`001015963` für `1.15.963`). Seit
#6392 musste eine neue Nummer über der höchsten des Ziel-Branches liegen. Jeder
Merge einer Migration machte damit jeden offenen PR mit Migration rot; in 60
Tagen kamen 311 Migrationen dazu, und jede Umnummerierung kostete einen
vollen CI-Lauf.

Der Grund für die Regel war #6391: bun führt eine nachgelieferte kleinere
Nummer trotzdem aus. Eine ältere Kopie von
`platform.erase_closed_demo_visitors` überschrieb so einen neueren Löschweg.
Die Funktion wurde in 27 Migrationen neu definiert, die fünf
`platform.school_erasure_*`-Funktionen je 16-mal, jedes Mal als volle Kopie.

Die Nummernregel behob das nicht. Zwei Branches kopierten dieselbe ältere
Fassung und änderten sie verschieden. Nach dem Umnummerieren lief die zweite
Kopie in der richtigen Reihenfolge und verwarf trotzdem die Änderung der
ersten. Git meldete nichts, weil jede Kopie in einer neuen Datei stand.

## Entscheidung

1. **Zeitstempel.** Eine neue Migration heißt nach ihrem UTC-Zeitstempel
   (`YYYYMMDDHHMMSS`), und ihre `Version` ist dieser Präfix. Die bisherigen
   Migrationen behalten Namen und Version. bun sortiert Namen als
   Zeichenketten, also laufen Zeitstempel nach allen bisherigen.
2. **Eine Definition je Funktion.** Die aktuelle Definition einer Funktion
   steht in einer Datei `repeatable_<schema>_<name>.go`. Jeder Migrationslauf
   installiert alle Definitionen nach den nummerierten Migrationen, auch ohne
   ausstehende Migration. Eine neue Migration darf eine neue Funktion anlegen,
   aber keine bestehende neu definieren. Die acht am häufigsten neu
   definierten Funktionen sind umgezogen; jede weitere zieht um, sobald sie
   das nächste Mal geändert wird.
3. **Die Reihenfolge wird geprüft, nicht erzwungen.** Liegt eine neue
   Migration unter der neuesten des Ziel-Branches, spielt
   `TestLateMigrationsYieldTheFreshSchema` die Historie zweimal ab: in
   Namensreihenfolge und mit dieser Migration am Ende. Weichen die Schemas ab
   oder läuft die Migration so nicht, schlägt der Test fehl, und die Migration
   wird auf einen aktuellen Zeitstempel umbenannt.

## Folgen

- Zwei PRs mit Migrationen blockieren sich nicht mehr. Ändern beide dieselbe
  Funktion, gibt es einen Git-Konflikt in der Zeile, die beide ändern.
- Eine Definition, die von Hand oder durch eine alte Migration überschrieben
  wurde, stellt der nächste Lauf wieder her. Dafür gibt es keine
  Prüfsummentabelle: Die Anweisungen sind idempotent und kosten Millisekunden.
- Eine frühere Fassung einer Funktion stellt kein `Down` mehr wieder her. Sie
  steht in der Git-Historie.
- Eine nummerierte Migration, die eine Funktion mit eigener Definition während
  ihres Laufs aufruft, bekommt die Fassung des vorherigen Laufs.
- Eine Definition bleibt in einer Datei unter der Dateigrenze von 800 Zeilen
  (ADR 0087). Wächst eine Funktion darüber, wird sie in SQL geteilt.

## Grenzen

- Der Reihenfolge-Test vergleicht Schemas, keine Zeilen. Ein Backfill, der die
  Daten einer anderen Migration braucht, nennt sie in `DependsOn`. Kennungen
  aus Sequenzen können sich zwischen einer frischen und einer gewachsenen
  Datenbank unterscheiden.
- Der Test sieht die Migrationen, die beim CI-Lauf auf dem Ziel-Branch liegen.
  Wird danach eine weitere gemergt, prüft erst der nächste Lauf gegen sie. Ein
  PR mit Migration holt deshalb vor dem Merge den Ziel-Branch.
- Der Wächter gegen Neudefinitionen liest die hinzugefügten Zeilen. SQL, das
  seinen Funktionsnamen erst zur Laufzeit zusammensetzt, sieht er nicht. Für
  Funktionen mit eigener Definition fängt das der nächste Lauf ab.

## Verworfen

- **Strikte Reihenfolge mit automatischem Umnummerieren** (Atlas `atlas.sum`,
  goose `fix`): behält die Last, dass jeder Merge offene PRs zum Rebase
  zwingt, und findet die verlorene Änderung an einer Funktion nicht.
- **Prüfsummen für die Definitionen** (Flyway `R__`): spart Millisekunden und
  lässt eine von einer alten Migration überschriebene Definition stehen, bis
  jemand ihre Datei ändert.
- **Zeitstempel allein:** hätte #6391 wieder geöffnet.
