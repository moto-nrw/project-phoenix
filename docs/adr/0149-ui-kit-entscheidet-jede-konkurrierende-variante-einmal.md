---
status: accepted
---

# UI-Kit entscheidet jede konkurrierende Variante einmal

**Status: Akzeptiert.** Tristan Heitger (Product Owner) hat die Entscheidungen
am 2026-11-03 im Epic [#5576](https://github.com/moto-nrw/project-phoenix/issues/5576)
freigegeben (Vergleichsseite im
[Kommentar](https://github.com/moto-nrw/project-phoenix/issues/5576#issuecomment-6604762663));
festgeschrieben in [#5578](https://github.com/moto-nrw/project-phoenix/issues/5578).
Die Regeldatei [`frontend-ui-kit.md`](../../.claude/rules/frontend-ui-kit.md)
setzt die Entscheidungen um, deren Baustein schon existiert.

## Kontext

Ein Scan aller Frontend-Dateien am 2026-11-03 fand dieselbe Aussage in
mehreren Gestalten: drei Kit-Pillen und rund 80 handgebaute für denselben
Status, zwei Kalenderstile, Hinweiskästen in mehreren Farben, Gruppenköpfe in
vier Versalien-Schreibweisen, Kennzahl-Kacheln in fünf Bauweisen. Die harten
Lint-Regeln wirken; die Drift sitzt dort, wo keine Regel hinsieht. Bei jedem
neuen Bildschirm war offen, welcher Baustein gilt, und jede Antwort erzeugte
eine weitere Variante.

## Entscheidung

Jede Oberfläche kommt aus dem Kit (`frontend/src/components/ui/`). Wo zwei
Varianten konkurrieren, gilt die Entscheidung unten. Referenzoptik sind
Anmeldungen und Planung, ohne Kicker. Geltungsbereich: alle Portale (Tenant,
Eltern, Schule, Operator, Träger, Caterer, Hilfe, öffentliche Anmeldung).
Kinderkarten, Notfallseite und Startseite des Elternportals bleiben unberührt.

### Optik

1. **Seitengerüst.** Schul- und Operator-Portal rendern `TenantPage`. Das
   Elternportal behält `ParentPage` und bekommt Lade-, Leer- und Fehler-Zweig
   nach dem `TenantPage`-Vertrag.
2. **Versalien.** Feldbeschriftungen nie in Versalien, immer `FieldLabel` oder
   die `label`-Prop. Gruppenköpfe und Zwischenüberschriften über
   `SectionLabel`: normal geschrieben, `text-sm font-semibold text-gray-600`,
   eine Stufe leiser als der Kartentitel. Außerhalb des Kits keine
   Klassenkette `uppercase tracking-*`.
3. **Warnfarbe nach Rolle.** Hinweiskästen sind orange über
   `Alert type="warning"`. Der Status „Achtung“ bleibt amber
   (`LOCATION_COLORS.WARNING`), nur als Tönung mit dunklem Text, nie als
   Textfarbe auf Weiß. Die Verwechslung „Schulhof“ gegen „Anfrage offen“
   regelt #5617.
4. **Eine Pille.** `StatusBadge` ist die Form für alle Pillen. Neu: Töne
   purple (`EXCUSED`), cyan (`CLASS_TRIP`) und `outline`, Größe `xs` (12 px,
   nicht in Versalien). `StatusColorBadge` und die DataTable-Pille teilen
   dieselbe Basis. Kein Punkt in Pillen (#2476).
5. **StatusDot.** Nur für Legenden und Pläne, wo heute schon Punkte stehen;
   daneben steht sichtbarer Text oder ein sr-Label; kein Pulsieren als Signal.
   Kein allgemeines Status-Mittel; eine Lint-Regel sperrt Punkte anderswo.
6. **Kennzahlen.** `StatCard` `variant="card"` bleibt unverändert
   (Versalien-Label, farbige Zahl, Symbol, Fortschrittsbalken) und ist die
   Standard-Kachel. Die ruhige Anmeldungs-Kachel wird `StatCard
   variant="inline"` (Symbol, Wert, Zusatz, Label) für dichte Raster in
   Karten; ihr Eigenbau im Modul entfällt. Alle übrigen handgebauten Kacheln
   wandern auf `card`.
7. **Kalender.** Einzeldatum und Zeitraum teilen einen Stil: Auswahl gray-900,
   Zeitraum-Mitte hellgrau, heute blau, Zellen `h-9`, Mitteltage in normalem
   Gewicht.
8. **Schließen-X.** In jedem Overlay `Button variant="ghost" size="icon"`.
9. **Kartenpolster.** Ein Wert für alle Karten: `p-5`, mobil `p-4`, wie
   `SectionCard`.
10. **Navigation.** Untere Leiste und Sidebar gibt es nur einmal im Kit.
    Tenant- und Elternportal setzen darauf auf; die Optik bleibt gleich, im
    Elternportal pixelgleich mit Screenshot belegt.
11. **Radius.** Der Code gilt: Bedienelemente `rounded-lg` (12 px), innere
    Flächen `rounded-xl` (16 px), Karten `rounded-2xl` (24 px). Kleine
    Bedienelemente des Kits (32-px-Buttons, Checkbox-Kästchen) bleiben
    `rounded-md`. Die Regeldatei folgt dem Kit, nicht umgekehrt.
12. **Buttons.** Regelt #5529. Dieses Epic ändert keine Button-Optik; wer einen
    Button anfasst, tauscht ihn nur gegen die bestehende Kit-Variante.

**Kicker:** verboten in Tenant, Hilfe, öffentlicher Anmeldung, Schul- und
Operator-Portal. Im Elternportal bleibt er, nur über `SectionCard kicker`.
Weitere Ausnahmen: NFC-Druckanleitung und `/start`.

### Freigabe-Schwelle

Jeder Farbwechsel und jede Änderung, die mehr als etwa 10 Stellen sichtbar
verändert, braucht vorab die Freigabe von Tristan Heitger. Alles andere wird
umgesetzt und mit Vorher/Nachher-Paaren für Desktop und Telefon belegt. Ist
ein Konflikt zwischen zwei Varianten hier nicht entschieden, wird im
Sub-Issue kommentiert und gewartet.

### Aufnahme neuer Kit-Bausteine

Ein neuer Kit-Baustein braucht mindestens 3 Verwender in mindestens 2
Bereichen. Erreicht ein geplanter Baustein die Schwelle nicht, werden die
Fundstellen lokal oder mit einem bestehenden Baustein gelöst. Das Kit enthält
keine Fachlogik. Fehlt etwas, das kein Issue vorsieht: im Issue kommentieren
und die Stelle offen lassen.

### Durchsetzung und Aufteilung großer Migrationen

Jedes aufgeräumte Muster endet in einer oxlint-Regel im UI-Kit-Plugin:
hard-zero, wenn der Bestand auf null geht, sonst eine shrink-only Baseline wie
beim Button-Ratchet. Regel, Ersatz und Nachtrag in `frontend-ui-kit.md`
kommen im selben PR. Ab etwa 40 Fundstellen kommt zuerst ein PR mit
Kit-Baustein, Regel und Baseline; die Migration folgt je Bereich (Tenant,
Eltern, Operator, …) als Sub-Issues des jeweiligen Issues mit je einem
Teil-PR, und jeder Teil-PR senkt die Baseline.

## Folgen

- `frontend-ui-kit.md` nennt jede Entscheidung, deren Baustein existiert, als
  „Need X → use Y“. Bausteine, die erst später entstehen (`SectionLabel`,
  `StatCard inline`, neue `StatusBadge`-Töne, gemeinsamer
  Kalenderstil, Kit-Navigation), stehen nur hier, bis ihr Issue sie liefert.
- Optik prüfen Vorher/Nachher-Paare, keine Snapshot-Tests. Neue Kit-Props
  bekommen Testing-Library-Tests über Rolle, Text und `aria`; jede neue
  Lint-Regel einen verletzenden und einen erlaubten Fall.
- Nicht Teil der Entscheidung: Button-Optik (#5529), neues Farbvokabular für
  Standorte (#5617), Umstieg auf die Komponenten von `@moto-nrw/design-system`,
  Kicker im Elternportal entfernen.
