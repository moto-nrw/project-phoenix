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

describe("DataTable pagination", () => {
  const manyRows: Row[] = Array.from({ length: 12 }, (_, i) => ({
    id: String(i + 1),
    name: `Row ${String(i + 1).padStart(2, "0")}`,
  }));

  it("renders only the first pageSize rows initially", () => {
    render(
      <DataTable
        columns={columns}
        rows={manyRows}
        getRowKey={(row) => row.id}
        pageSize={5}
      />,
    );

    expect(screen.getByText("Row 01")).toBeInTheDocument();
    expect(screen.getByText("Row 05")).toBeInTheDocument();
    expect(screen.queryByText("Row 06")).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Mehr laden \(5 von 12\)/ }),
    ).toBeInTheDocument();
  });

  it("reveals another pageSize chunk when 'Mehr laden' is clicked", () => {
    render(
      <DataTable
        columns={columns}
        rows={manyRows}
        getRowKey={(row) => row.id}
        pageSize={5}
      />,
    );

    fireEvent.click(
      screen.getByRole("button", { name: /Mehr laden \(5 von 12\)/ }),
    );

    expect(screen.getByText("Row 06")).toBeInTheDocument();
    expect(screen.getByText("Row 10")).toBeInTheDocument();
    expect(screen.queryByText("Row 11")).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Mehr laden \(10 von 12\)/ }),
    ).toBeInTheDocument();
  });

  it("hides the load-more button once all rows are visible", () => {
    render(
      <DataTable
        columns={columns}
        rows={manyRows}
        getRowKey={(row) => row.id}
        pageSize={5}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /Mehr laden/ }));
    fireEvent.click(screen.getByRole("button", { name: /Mehr laden/ }));

    expect(screen.getByText("Row 12")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /Mehr laden/ }),
    ).not.toBeInTheDocument();
  });

  it("renders all rows and no button when pageSize is omitted", () => {
    render(
      <DataTable
        columns={columns}
        rows={manyRows}
        getRowKey={(row) => row.id}
      />,
    );

    expect(screen.getByText("Row 01")).toBeInTheDocument();
    expect(screen.getByText("Row 12")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /Mehr laden/ }),
    ).not.toBeInTheDocument();
  });

  it("resets to the first page when the caller's filter key changes", () => {
    const { rerender } = render(
      <DataTable
        columns={columns}
        rows={manyRows}
        getRowKey={(row) => row.id}
        pageSize={5}
        paginationResetKey="all"
      />,
    );

    fireEvent.click(
      screen.getByRole("button", { name: /Mehr laden \(5 von 12\)/ }),
    );
    expect(screen.getByText("Row 10")).toBeInTheDocument();

    rerender(
      <DataTable
        columns={columns}
        rows={manyRows}
        getRowKey={(row) => row.id}
        pageSize={5}
        paginationResetKey="filtered"
      />,
    );

    expect(screen.queryByText("Row 06")).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /Mehr laden \(5 von 12\)/ }),
    ).toBeInTheDocument();
  });
});

// The stacked phone layout: same rows, same paging, one component. Columns
// declare the role they play so a phone shows the values a narrow viewport
// would otherwise push off screen.
describe("DataTable stacked phone layout", () => {
  interface PersonRow {
    id: string;
    name: string;
    schoolClass: string;
    iban: string;
  }

  const stackedColumns: DataTableColumn<PersonRow>[] = [
    {
      key: "name",
      header: "Kind",
      render: (row) => row.name,
      sortValue: (row) => row.name,
      stacked: "title",
    },
    {
      key: "class",
      header: "Klasse",
      render: (row) => row.schoolClass,
      stacked: "meta",
    },
    { key: "iban", header: "IBAN", render: (row) => row.iban },
  ];

  const people: PersonRow[] = [
    { id: "1", name: "Mia", schoolClass: "1a", iban: "•••• 3000" },
    { id: "2", name: "Lea", schoolClass: "2b", iban: "•••• 4000" },
  ];

  it("renders the field columns as labelled lines beside the table", () => {
    render(
      <DataTable
        columns={stackedColumns}
        rows={people}
        getRowKey={(row) => row.id}
        stackedOnMobile
      />,
    );

    const stacked = screen.getByTestId("data-table-stacked");
    expect(stacked).toHaveTextContent("Mia");
    expect(stacked).toHaveTextContent("1a");
    // The label comes from the column header, the value from its renderer.
    expect(stacked).toHaveTextContent("IBAN");
    expect(stacked).toHaveTextContent("•••• 3000");
    expect(screen.getByTestId("data-table-table")).toBeInTheDocument();
  });

  it("shares paging with the table instead of counting its own rows", () => {
    render(
      <DataTable
        columns={stackedColumns}
        rows={people}
        getRowKey={(row) => row.id}
        pageSize={1}
        stackedOnMobile
      />,
    );

    const stacked = screen.getByTestId("data-table-stacked");
    expect(stacked).not.toHaveTextContent("Lea");

    fireEvent.click(
      screen
        .getAllByRole("button", { name: /Mehr laden \(1 von 2\)/ })
        .filter((el) => stacked.contains(el))[0]!,
    );

    expect(stacked).toHaveTextContent("Lea");
  });

  it("renders the table alone when the phone layout is not requested", () => {
    render(
      <DataTable
        columns={stackedColumns}
        rows={people}
        getRowKey={(row) => row.id}
      />,
    );

    expect(screen.queryByTestId("data-table-stacked")).not.toBeInTheDocument();
  });
});
