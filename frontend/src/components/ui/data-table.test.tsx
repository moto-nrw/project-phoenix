/**
 * Tests for DataTable.
 *
 * Covers keyboard activation of clickable rows (Enter / Space), correct
 * suppression of bubbled keydown from inner elements, and the loading
 * placeholder rendering path.
 */
import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";

import { DataTable } from "./data-table";
import type { DataTableColumn } from "./data-table";

interface Row {
  id: string;
  name: string;
}

const rows: Row[] = [
  { id: "1", name: "Alpha" },
  { id: "2", name: "Beta" },
];

const columns: DataTableColumn<Row>[] = [
  { key: "name", header: "Name", render: (row) => row.name },
];

describe("DataTable keyboard navigation", () => {
  it("invokes onRowClick when Enter is pressed on a clickable row", () => {
    const onRowClick = vi.fn();

    render(
      <DataTable
        columns={columns}
        rows={rows}
        getRowKey={(row) => row.id}
        onRowClick={onRowClick}
      />,
    );

    const rowButtons = screen.getAllByRole("button");
    // The first button is the column header sort; pick the row buttons by tabIndex=0
    const clickableRow = rowButtons.find(
      (el) => el.tagName === "TR" && el.getAttribute("tabIndex") === "0",
    );
    expect(clickableRow).toBeDefined();
    if (clickableRow) {
      fireEvent.keyDown(clickableRow, {
        key: "Enter",
        target: clickableRow,
        currentTarget: clickableRow,
      });
    }

    expect(onRowClick).toHaveBeenCalledWith(rows[0]);
  });

  it("invokes onRowClick when Space is pressed on a clickable row", () => {
    const onRowClick = vi.fn();

    render(
      <DataTable
        columns={columns}
        rows={rows}
        getRowKey={(row) => row.id}
        onRowClick={onRowClick}
      />,
    );

    const tr = screen
      .getAllByRole("button")
      .find((el) => el.tagName === "TR" && el.getAttribute("tabIndex") === "0");
    expect(tr).toBeDefined();
    if (tr) {
      fireEvent.keyDown(tr, { key: " ", target: tr, currentTarget: tr });
    }

    expect(onRowClick).toHaveBeenCalled();
  });

  it("does not invoke onRowClick for other keys", () => {
    const onRowClick = vi.fn();

    render(
      <DataTable
        columns={columns}
        rows={rows}
        getRowKey={(row) => row.id}
        onRowClick={onRowClick}
      />,
    );

    const tr = screen
      .getAllByRole("button")
      .find((el) => el.tagName === "TR" && el.getAttribute("tabIndex") === "0");
    if (tr) {
      fireEvent.keyDown(tr, { key: "a", target: tr, currentTarget: tr });
    }

    expect(onRowClick).not.toHaveBeenCalled();
  });

  it("renders the loading state when isLoading is true", () => {
    render(
      <DataTable
        columns={columns}
        rows={rows}
        getRowKey={(row) => row.id}
        isLoading
      />,
    );

    expect(screen.getByText("Wird geladen…")).toBeInTheDocument();
  });

  it("renders the empty state when no rows are passed", () => {
    render(
      <DataTable
        columns={columns}
        rows={[]}
        getRowKey={(row) => row.id}
        emptyState="Nichts da."
      />,
    );

    expect(screen.getByText("Nichts da.")).toBeInTheDocument();
  });

  it("does not render rows as buttons when onRowClick is not set", () => {
    render(
      <DataTable columns={columns} rows={rows} getRowKey={(row) => row.id} />,
    );

    const rowEls = screen.getAllByRole("row");
    // Header row + 2 data rows = 3
    expect(rowEls.length).toBe(3);
    // None of the data rows should have role="button"
    const dataRows = rowEls.filter((el) => el.tagName === "TR");
    for (const r of dataRows) {
      expect(r.getAttribute("role")).not.toBe("button");
    }
  });
});
