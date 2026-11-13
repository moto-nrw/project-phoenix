---
status: accepted
---

# Der Go-Modulpfad folgt der GitHub-Organisation

Entscheidung vom 12.11.2026. Sie ergänzt
[ADR 0020](0020-a-relocated-package-keeps-its-debt.md) und
[ADR 0083](0083-ratchets-protect-invariants-and-expose-command-effects.md).
Sie ändert keine Regel, keinen Owner und keine Rolle.

## Kontext

Die GitHub-Organisation heißt künftig `moto-ogs` statt `moto-nrw`. Repository,
Container-Images, Links und Werkzeuge ziehen mit. Der Go-Modulpfad
`github.com/moto-nrw/project-phoenix` ist nur ein Bezeichner, kein Download-Ort;
er würde ohne Änderung weiter bauen. Ein Modulpfad, der eine fremde oder leere
Organisation nennt, führt Leser aber in die Irre und bleibt eine Falle, sobald
jemand den alten Namen belegt.

Der Architektur-Ratchet vergleicht jede Änderung mit der unveränderlichen Base.
Paketpfade, Effect-Bindungen und ihre Digests tragen den Modulpfad. Ohne
Vorkehrung liest der Vergleich die Umbenennung als Lockerung
(`module_path changed`), jede Lesegrant als neu und jede reviewte Ausnahme als
neue Ausnahme ohne ADR.

## Entscheidung

1. **Der Modulpfad wird `github.com/moto-ogs/project-phoenix`.** Die Umbenennung
   ist ein eigener Pull Request, der Go-Quellen nur im Modulpfad ändert.
2. **Eine reviewte Epoche deklariert sie.** `policy.json` trägt
   `module_rename` mit `from` (dem alten Pfad) und `approval_adr` (dieser ADR).
   Die Epoche steigt.
3. **Der Ratchet liest die Base unter dem neuen Pfad.** Solange die Base noch
   `from` nennt, liest PR mode Base-Policy, Baseline und Effect-Register mit dem
   neuen Pfad und vergleicht wie immer. Die Deklaration gilt nur, wenn
   `approval_adr` in dieser Änderung hinzukommt und jede Go-Datei außerhalb von
   `internal/architecture/` ihrer Base-Datei gleicht, sobald der neue Pfad als
   der alte geschrieben wird. Hinzugefügte oder gelöschte Go-Dateien scheitern.
4. **Digests derselben Identität übernehmen den Kandidatenwert.** Sie hashen
   nach Punkt 3 dieselben Quellen unter dem alten Pfad. Jede andere
   Effect-, Klassifizierungs-, Ownership- und Regeländerung bleibt verboten.

## Folgen

- Nach dem Merge nennt die Base den neuen Pfad. Die Deklaration ist dann ein
  Nachweis und greift nicht erneut, wie eine abgeschlossene Relocation
  (ADR 0020).
- Offene Pull Requests bekommen Konflikte in Importen und müssen auf
  `development` aktualisiert werden; ein erneutes Ersetzen von
  `github.com/moto-nrw/project-phoenix` genügt dafür.
- Das npm-Paket `@moto-nrw/design-system` ist ein Registry-Name, kein
  GitHub-Pfad, und bleibt unverändert.
