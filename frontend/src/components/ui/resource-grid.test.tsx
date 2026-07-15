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

  it("applies an optional status edge color to the row header", () => {
    renderGrid({
      rowEdgeColor: (row) => (row.id === "r2" ? "#EAB308" : undefined),
    });

    const edged = screen.getByText("B. Yilmaz").closest("th");
    expect(edged).toHaveStyle({ borderLeft: "2px solid #EAB308" });
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
});
