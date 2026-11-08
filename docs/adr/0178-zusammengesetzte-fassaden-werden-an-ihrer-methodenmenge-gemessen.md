---
status: proposed
---

# Zusammengesetzte Fassaden werden an ihrer Methodenmenge gemessen

Entscheidung vom 08.11.2026 für Punkt 3 von
[#6178](https://github.com/moto-nrw/project-phoenix/issues/6178), unter
[ADR 0057](0057-only-the-target-is-a-rule.md) Punkt 5. Kein Runtime-Code,
keine Policy-Regel, kein Port und kein bestehendes Budget ändert sich. Neu ist
ein Ratchet-Register.

## Kontext

`TestModuleFacadeWidthRatchet` zählt bei jedem exportierten Interface der
öffentlichen Modul- und Workflow-Pakete nur die selbst deklarierten Methoden.
Eingebettete Interfaces zählen bewusst nicht mit. Sonst würde die Form
`Capability = Query + Command` noch einmal für Methoden belastet, die ihre
Hälften schon tragen. Die Folge: Ein Interface, das nur einbettet, misst fast
0. `identityaccess.Owner` bettet 47 Capabilities ein und misst 3. Seine
Methodenmenge umfasst 262 Methoden. Wächst ein solches Interface um eine
weitere Einbettung, schlägt kein Ratchet an. Das ist auch tatsächlich passiert: Zwischen
development `e59b3f93cd` und `b474b11a27` hat
[#6161](https://github.com/moto-nrw/project-phoenix/pull/6161)
`enrollment.Owner` von 139 auf 140 Methoden vergrößert, unbemerkt. Kurz darauf hat
[#6216](https://github.com/moto-nrw/project-phoenix/pull/6216) ebenso `parentbilling.CareBilling`
von 13 auf 14 Methoden vergrößert. Das Register des bestehenden
Tests ist leer, weil jede Fassade heute höchstens 12 Methoden selbst
deklariert.

## Entscheidung

1. **Ein zweites Register misst die Methodenmenge.**
   `TestModuleFacadeMethodSetRatchet`
   (`backend/test/module_facade_method_set_ratchet_test.go`) prüft dieselben
   Root-Pakete. Er misst jedes exportierte Interface, das mindestens ein
   anderes Interface einbettet, mit `go/types` an seiner vollständigen
   Methodenmenge (`types.Interface.NumMethods`). Die Grenze ist dieselbe wie
   bei der deklarierten Breite: 12 Methoden.
2. **Keine Doppelbelastung.** Ein Interface ohne Einbettung bleibt nur im
   bestehenden Register; seine Methodenmenge ist dort dieselbe Zahl. Die
   eingebetteten Fassaden behalten ihre eigenen Einträge, und ihre deklarierte
   Breite wird nicht noch einmal gezählt. Das neue Register misst nur die
   Composite selbst, also die Fassade, von der ein Konsument dieses Interfaces
   abhängt. Type-Set-Constraints (`~int`, Unions) sind keine Fassaden und
   bleiben draußen.
3. **Startwert ist der heutige Stand.** Gemessen am 08.11.2026 auf development
   `e20e8013b3`: 108 Composites über 12 Methoden, zusammen 4.491 Methoden. Die
   größten sind `identityaccess.Owner` (262), `careplan.Owner` (159),
   `careplan.Capability` (158), `peopledirectory.Owner` (146),
   `peopledirectory.Capability` (143) und `enrollment.Owner` (140). Diese Zahlen
   sind Schulden, kein Ziel.
4. **Es gilt die Ratchet-Semantik des bestehenden Registers.** Ein Eintrag darf
   fallen und muss dann gesenkt oder gelöscht werden. Steigen darf er nicht.
   Eine neue Composite über 12 Methoden schlägt fehl. Erhöhen oder ergänzen
   lässt sich das Register nur mit einer neuen ADR (ADR 0057 Punkt 5). Das
   prüft `scripts/backend-architecture.sh check` gegen die Basis des PRs.

## Konsequenzen

- Wer einem `Owner`- oder `Capability`-Interface eine Einbettung oder eine
  Methode hinzufügt, sieht das als Wachstum. Eine neue Capability bekommt ihr
  eigenes kleines Interface; der Konsument hängt an ihr, nicht an der
  Composite.
- Ein Konsument, der heute eine breite Composite verlangt, kann auf die
  passende Teil-Capability umgestellt werden. Danach sinkt der Eintrag oder
  verschwindet.
- Der Test prüft die Typen nur mit Paketen, die die Policy der Rolle
  `test-support` erlaubt (`go/parser`, `go/types`, `go/build/constraint`, `os`).
  `golang.org/x/tools` (compiler-tooling), `go/build` und `os/exec` gehören
  nicht dazu, und eine neue Policy-Regel ist nicht Teil dieser Entscheidung.
  First-Party-Pakete und die Standardbibliothek werden ohne Funktionsrümpfe aus
  dem Quelltext geprüft. Die Dateien wählt der Test wie der Architektur-Check
  für linux/amd64 ohne cgo und mit dem Tag `testhooks`. Dadurch wird auch eine
  Composite richtig gemessen, deren Hälften in einem anderen Paket oder in der
  Standardbibliothek liegen. Drittanbieter-Importe bleiben leer. Bettet eine
  Fassade ein Interface ein, das deshalb unaufgelöst bleibt, bricht der Scan ab
  (`TestModuleFacadeMethodSetRejectsUnresolvedEmbeddings`) und misst die
  Methodenmenge nicht still zu klein. Der Scan dauert lokal unter einer Sekunde.
- Der Fixture-Test `TestModuleFacadeMethodSetCatchesCompositeGrowth` belegt die
  Lücke: Ein `Owner` wächst durch eine vierte Einbettung von 19 auf 25 Methoden.
  Die deklarierte Zählung sieht 0, das neue Register schlägt fehl.
