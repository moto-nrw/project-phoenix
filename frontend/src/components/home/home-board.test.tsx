import { describe, expect, it } from "vitest";
import { renderToString } from "react-dom/server";

import type { HomeBlockPlacement } from "~/lib/home-blocks";
import { HomeBoard } from "./home-board";

/**
 * Das Brett muss schon im Server-HTML bei jeder Bildschirmbreite richtig
 * stehen. Vorher entschied eine Media-Query aus JavaScript über die
 * Spaltenzahl; der Server kannte sie nicht, packte alles in zwei Spalten,
 * und in einem breiten Fenster standen die Kacheln bis zur Hydration in der
 * linken Hälfte des Vier-Spalten-Rasters.
 */
const placements: readonly HomeBlockPlacement[] = [
  { key: "section.recent_activity", span: 4, col: 0, row: 0 },
  { key: "tile.students_present", span: 1, col: 0, row: 2 },
  { key: "tile.students_in_rooms", span: 1, col: 1, row: 2 },
  { key: "tile.students_sick", span: 1, col: 2, row: 2 },
  { key: "tile.students_home", span: 1, col: 3, row: 2 },
];

function renderBoard(): string {
  return renderToString(
    <HomeBoard
      placements={placements}
      addable={[]}
      requiredKeys={new Set()}
      editing={false}
      onMove={() => {}}
      onMoveBy={() => {}}
      onSpanChange={() => {}}
      onRemove={() => {}}
      onAdd={() => {}}
      onRestoreDefault={() => {}}
      restoring={false}
    >
      {(placement) => <span>{placement.key}</span>}
    </HomeBoard>,
  );
}

function itemTag(html: string, key: string): string {
  const match = new RegExp(
    `<li[^>]*data-testid="home-block-${key}"[^>]*>`,
  ).exec(html);
  if (!match) throw new Error(`Kachel ${key} fehlt im Server-HTML`);
  return match[0];
}

describe("HomeBoard im Server-HTML", () => {
  it("trägt die Vier-Spalten-Zelle der Person und legt sie per CSS ab xl an", () => {
    const html = renderBoard();

    const wideBlock = itemTag(html, "section.recent_activity");
    expect(wideBlock).toContain("--cell-col-4:1 / span 4");
    expect(wideBlock).toContain("--cell-row-4:1 / span 2");
    expect(wideBlock).toContain("xl:[grid-column:var(--cell-col-4)]");
    expect(wideBlock).toContain("xl:[grid-row:var(--cell-row-4)]");

    // Die vierte Kennzahl steht rechts außen, nicht in einer neuen Zeile.
    const lastTile = itemTag(html, "tile.students_home");
    expect(lastTile).toContain("--cell-col-4:4 / span 1");
    expect(lastTile).toContain("--cell-row-4:3 / span 1");
  });

  it("trägt daneben die Zwei-Spalten-Zelle für Tablet und schmalen Desktop", () => {
    const html = renderBoard();

    const wideBlock = itemTag(html, "section.recent_activity");
    expect(wideBlock).toContain("--cell-col-2:1 / span 2");
    expect(wideBlock).toContain("sm:[grid-column:var(--cell-col-2)]");

    // In zwei Spalten rückt die vierte Kennzahl unter die dritte.
    const lastTile = itemTag(html, "tile.students_home");
    expect(lastTile).toContain("--cell-col-2:2 / span 1");
    expect(lastTile).toContain("--cell-row-2:4 / span 1");
  });

  it("weist keiner Kachel eine Zelle über JavaScript zu", () => {
    const html = renderBoard();
    for (const placement of placements) {
      expect(itemTag(html, placement.key)).not.toMatch(/grid-column:\d/);
    }
  });

  it("lässt die Kacheln auf dem Handy in Lesereihenfolge fließen", () => {
    const html = renderBoard();
    expect(itemTag(html, "section.recent_activity")).toContain("col-span-2");
    expect(itemTag(html, "tile.students_present")).toContain("col-span-1");
  });
});
