import type { HomeBlockSpan, HomeBoardCell } from "~/lib/home-blocks";

/**
 * Was Leseansicht und Anpassen-Modus des Bretts gemeinsam brauchen: beide
 * legen jede Kachel in dieselbe Zelle, damit der Wechsel zwischen ihnen
 * nichts verschiebt.
 */

// Auch das Handy hat zwei Spalten: eine Kennzahl ist eine Zahl mit einem Wort
// und braucht keine volle Bildschirmbreite — untereinander gestellt schiebt
// sie alles Wichtige unter den Rand. Listen nehmen dort beide Spalten. Nur
// dort fließen die Kacheln; ab `sm` überschreiben die Zellvariablen die
// Spalte (siehe CELL_CLASSES).
export const SPAN_CLASS: Record<HomeBlockSpan, string> = {
  1: "col-span-1",
  2: "col-span-2",
  4: "col-span-2",
};

/**
 * Ab der zweispaltigen Ansicht bekommt jede Kachel ihre Zelle ausdrücklich —
 * und zwar ÜBER CSS, nicht über JavaScript. Die Kachel trägt beide Zellen als
 * Variablen (die des Zwei-Spalten-Rasters und die der vier Spalten der
 * Person), und der Breakpoint entscheidet, welche gilt. So stimmt schon das
 * Server-HTML bei jeder Bildschirmbreite. Vorher entschied eine Media-Query
 * aus JavaScript über die Spaltenzahl; die kennt der Server nicht und lieferte
 * immer zwei Spalten. In einem breiten Fenster standen dann alle Kacheln bis
 * zur Hydration in der linken Hälfte des Vier-Spalten-Rasters und sprangen
 * erst danach auf volle Breite.
 */
export const CELL_CLASSES =
  "sm:[grid-column:var(--cell-col-2)] sm:[grid-row:var(--cell-row-2)] xl:[grid-column:var(--cell-col-4)] xl:[grid-row:var(--cell-row-4)]";

/** Spalten des schmalen Rasters (Tablet und schmaler Desktop). */
export const NARROW_BOARD_COLUMNS = 2;

// Jede Zelle bekommt ihren Platz ausdrücklich zugewiesen (Spalte und Zeile
// der Person); das Raster verteilt nichts selbst und füllt keine Lücken. Nur
// auf dem Handy fließen die Kacheln in Lesereihenfolge, dort gibt es keine
// Zelle, neben der etwas stehen müsste. `relative`, damit die Zellen ihre
// Lage relativ zum Raster kennen.
export const BOARD_GRID_CLASSES =
  "relative grid grid-cols-2 gap-4 sm:auto-rows-[7rem] xl:grid-cols-4";

/** Die Zelle eines Rasters als die Variablen, die CELL_CLASSES liest. */
export function cellVariables(
  cell: HomeBoardCell | null,
  columns: number,
): Record<string, string> {
  if (!cell) return {};
  return {
    [`--cell-col-${columns}`]: `${cell.columnStart} / span ${cell.columnSpan}`,
    [`--cell-row-${columns}`]: `${cell.rowStart} / span ${cell.rowSpan}`,
  };
}
