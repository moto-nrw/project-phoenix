import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

import { CapacityStrip } from "./capacity-strip";
import { ResourceGrid, type ResourceGridColumn } from "./resource-grid";

interface Row {
  readonly id: string;
  readonly name: string;
}

const ROWS: readonly Row[] = [
  { id: "r1", name: "A. Krause" },
  { id: "r2", name: "B. Yilmaz" },
];

const DAY_COLUMNS: readonly ResourceGridColumn[] = [
  { key: "mo", label: "Mo 13.07." },
  { key: "di", label: "Di 14.07.", isCurrent: true },
  { key: "mi", label: "Mi 15.07." },
];

function renderGrid(
  overrides: Partial<Parameters<typeof ResourceGrid<Row>>[0]> = {},
) {
  return render(
    <ResourceGrid<Row>
      columns={DAY_COLUMNS}
      rows={ROWS}
      getRowKey={(row) => row.id}
      renderRowHeader={(row) => <span>{row.name}</span>}
      renderCell={() => null}
      ariaLabel="Testraster"
      {...overrides}
    />,
  );
}

describe("ResourceGrid", () => {
  it("clips the rounded surface without becoming a scroll container", () => {
    const { container } = renderGrid();

    expect(container.firstElementChild).toHaveClass("overflow-clip");
    expect(container.firstElementChild).not.toHaveClass("overflow-hidden");
  });

  it("renders the row header in a sticky first column", () => {
    renderGrid();

    const header = screen.getByText("A. Krause").closest("th");
    expect(header).not.toBeNull();
    expect(header).toHaveClass("sticky", "left-0");
  });

  it("renders an empty cell as a labelled button wired to onEmptyCellClick", () => {
    const onEmptyCellClick = vi.fn();
    renderGrid({
      emptyCellLabel: (row, column) =>
        `Schicht anlegen, ${row.name}, ${column.label}`,
      onEmptyCellClick,
    });

    const button = screen.getByRole("button", {
      name: "Schicht anlegen, A. Krause, Mo 13.07.",
    });
    expect(button).toHaveAttribute("type", "button");
    fireEvent.click(button);
    expect(onEmptyCellClick).toHaveBeenCalledWith(ROWS[0], DAY_COLUMNS[0]);
  });

  it("renders no empty-cell button when the empty-cell props are omitted", () => {
    renderGrid();
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });

  it("renders caller cell content in the order the render prop returns it", () => {
    renderGrid({
      renderCell: (row, column) =>
        column.key === "mo" ? (
          <div>
            <span>{`${row.name} früh`}</span>
            <span>{`${row.name} spät`}</span>
          </div>
        ) : null,
    });

    const cell = screen.getByText("A. Krause früh").closest("div");
    expect(cell?.textContent).toBe("A. Krause frühA. Krause spät");
  });

  it("tints the current column neutral gray and never amber", () => {
    renderGrid();

    const currentHead = screen.getByText("Di 14.07.").closest("th");
    expect(currentHead).toHaveClass("bg-gray-100");
    expect(currentHead?.className).not.toContain("amber");
  });

  it("uses a narrower per-column min width in weeks mode", () => {
    const { rerender } = renderGrid({ columnMode: "days" });
    expect(screen.getByText("Mo 13.07.").closest("th")).toHaveClass(
      "min-w-[7.5rem]",
    );

    rerender(
      <ResourceGrid<Row>
        columns={DAY_COLUMNS}
        rows={ROWS}
        getRowKey={(row) => row.id}
        renderRowHeader={(row) => <span>{row.name}</span>}
        renderCell={() => null}
        columnMode="weeks"
      />,
    );
    expect(screen.getByText("Mo 13.07.").closest("th")).toHaveClass(
      "min-w-[3.25rem]",
    );
  });

  // Issue #2026: an empty and a filled plan must raster identically. Column
  // widths therefore come from a fixed table layout plus a colgroup, never from
  // the cell content, and every cell carries the same height floor.
  it("lays the table out with fixed columns from a colgroup, not from content", () => {
    const { container } = renderGrid();

    const table = container.querySelector("table");
    expect(table).toHaveClass("table-fixed");
    // One <col> for the sticky row header plus one per data column.
    expect(container.querySelectorAll("colgroup col")).toHaveLength(
      DAY_COLUMNS.length + 1,
    );
    expect((table as HTMLTableElement).style.minWidth).toBe(
      `calc(13rem + ${DAY_COLUMNS.length} * 7.5rem)`,
    );
  });

  it("scales the table min width with the narrower weeks columns", () => {
    const { container } = renderGrid({ columnMode: "weeks" });

    const table = container.querySelector("table") as HTMLTableElement;
    expect(table.style.minWidth).toBe(
      `calc(13rem + ${DAY_COLUMNS.length} * 3.25rem)`,
    );
  });

  it("gives empty, bare and filled cells the same height floor", () => {
    const { container } = renderGrid({
      renderCell: (row, column) =>
        column.key === "mo" ? <span>{`${row.name} früh`}</span> : null,
      emptyCellLabel: (row, column) => `Anlegen, ${row.name}, ${column.label}`,
      onEmptyCellClick: vi.fn(),
    });

    const filledWrapper = screen.getByText("A. Krause früh").parentElement;
    const emptyWrapper = screen.getByRole("button", {
      name: "Anlegen, A. Krause, Di 14.07.",
    }).parentElement;

    expect(filledWrapper).toHaveClass("min-h-16");
    expect(emptyWrapper).toHaveClass("min-h-16");
    // Every cell wrapper in the body carries the floor, including the ones
    // without any empty-cell affordance.
    for (const cell of container.querySelectorAll("tbody td")) {
      expect(cell.firstElementChild).toHaveClass("min-h-16");
    }
  });

  it("uses the shorter cell height floor in weeks mode", () => {
    const { container } = renderGrid({ columnMode: "weeks" });

    for (const cell of container.querySelectorAll("tbody td")) {
      expect(cell.firstElementChild).toHaveClass("min-h-11");
    }
  });

  it("renders the footer slot inside a tfoot (CapacityStrip interplay)", () => {
    renderGrid({
      footer: (
        <CapacityStrip
          rowLabel="Kapazität 12-16"
          cells={[
            { key: "mo", content: 6 },
            { key: "di", content: 5 },
            { key: "mi", content: 6 },
          ]}
        />
      ),
    });

    const label = screen.getByText("Kapazität 12-16");
    expect(label.closest("tfoot")).not.toBeNull();
  });

  it("renders the corner header slot above the sticky column", () => {
    renderGrid({ cornerHeader: "Person" });
    const corner = screen.getByText("Person").closest("th");
    expect(corner).toHaveClass("sticky", "left-0");
  });

  it("exposes the scroll region as keyboard-focusable and links an optional hint", () => {
    renderGrid({ scrollHintId: "scroll-hint-x" });

    const region = screen.getByRole("region", { name: "Testraster" });
    expect(region).toHaveAttribute("tabindex", "0");
    expect(region).toHaveAttribute("aria-describedby", "scroll-hint-x");
  });

  it("scrolls the region horizontally with the arrow keys", () => {
    renderGrid();

    const region = screen.getByRole("region", { name: "Testraster" });
    const scrollBy = vi.fn();
    region.scrollBy = scrollBy as unknown as HTMLElement["scrollBy"];

    fireEvent.keyDown(region, { key: "ArrowRight" });
    expect(scrollBy).toHaveBeenCalledWith({ left: 240, behavior: "smooth" });

    fireEvent.keyDown(region, { key: "ArrowLeft" });
    expect(scrollBy).toHaveBeenCalledWith({ left: -240, behavior: "smooth" });
  });

  it("ignores non-arrow keys on the scroll region", () => {
    renderGrid();

    const region = screen.getByRole("region", { name: "Testraster" });
    const scrollBy = vi.fn();
    region.scrollBy = scrollBy as unknown as HTMLElement["scrollBy"];

    fireEvent.keyDown(region, { key: "Enter" });
    expect(scrollBy).not.toHaveBeenCalled();
  });

  it("does not hijack arrow keys from an interactive cell descendant", () => {
    renderGrid({
      renderCell: (_row, column) =>
        column.key === "mo" ? <input aria-label="Zelleneingabe" /> : null,
    });

    const region = screen.getByRole("region", { name: "Testraster" });
    const scrollBy = vi.fn();
    region.scrollBy = scrollBy as unknown as HTMLElement["scrollBy"];

    const input = screen
      .getAllByRole("textbox", { name: "Zelleneingabe" })
      .at(0);
    if (!input) throw new Error("Testzelle wurde nicht gerendert");
    const wasNotPrevented = fireEvent.keyDown(input, { key: "ArrowRight" });

    expect(wasNotPrevented).toBe(true);
    expect(scrollBy).not.toHaveBeenCalled();
  });
});
