import { render, screen } from "@testing-library/react";
import type { ReactNode } from "react";
import { describe, expect, it } from "vitest";
import "@testing-library/jest-dom/vitest";

import { CapacityStrip, type CapacityStripCell } from "./capacity-strip";

function renderStrip(
  cells: readonly CapacityStripCell[],
  rowLabel: ReactNode = "Kapazität 12-16",
) {
  return render(
    <table>
      <tfoot>
        <CapacityStrip rowLabel={rowLabel} cells={cells} />
      </tfoot>
    </table>,
  );
}

describe("CapacityStrip", () => {
  it("renders the row label in the leading sticky cell", () => {
    renderStrip([{ key: "mo", content: 6 }]);

    const label = screen.getByText("Kapazität 12-16");
    expect(label.tagName).toBe("TH");
    expect(label).toHaveClass("sticky", "left-0");
  });

  it("renders plain preformatted values without any red marking by default", () => {
    renderStrip([
      { key: "mo", content: 6 },
      { key: "di", content: "~42 · 6 P." },
    ]);

    const number = screen.getByText("6");
    expect(number).not.toHaveStyle({ color: "#FF3130" });
    expect(number).not.toHaveClass("font-semibold");
    expect(screen.getByText("~42 · 6 P.")).toBeInTheDocument();
  });

  it("marks an understaffed cell red and semibold (text color only)", () => {
    renderStrip([
      { key: "mo", content: 6 },
      { key: "di", content: 4, understaffed: true },
    ]);

    const marked = screen.getByText("4");
    expect(marked).toHaveStyle({ color: "#FF3130" });
    expect(marked).toHaveClass("font-semibold");
    // Peers stay neutral.
    expect(screen.getByText("6")).not.toHaveStyle({ color: "#FF3130" });
  });

  it("can drop the sticky positioning for standalone use", () => {
    render(
      <table>
        <tfoot>
          <CapacityStrip
            rowLabel="Kapazität"
            cells={[{ key: "mo", content: 6 }]}
            stickyLabel={false}
          />
        </tfoot>
      </table>,
    );

    expect(screen.getByText("Kapazität")).not.toHaveClass("sticky");
  });

  describe('div mode (as="div")', () => {
    it("renders without a <table> context, applying the given column template", () => {
      render(
        <CapacityStrip
          as="div"
          rowLabel="~42 · 6 P."
          gridTemplateColumns="180px repeat(5, 1fr)"
          cells={[{ key: "mo", content: 6 }]}
        />,
      );

      const label = screen.getByText("~42 · 6 P.");
      expect(label.tagName).toBe("DIV");
      const row = screen.getByRole("row");
      expect(row.tagName).toBe("DIV");
      expect(row).toHaveStyle({ gridTemplateColumns: "180px repeat(5, 1fr)" });
      expect(screen.getByText("6")).toBeInTheDocument();
    });

    it("uses border-b in header position instead of the default border-t footer", () => {
      render(
        <CapacityStrip
          as="div"
          position="header"
          rowLabel="Kopf"
          cells={[{ key: "mo", content: 6 }]}
        />,
      );

      const row = screen.getByRole("row");
      expect(row).toHaveClass("border-b");
      expect(row).not.toHaveClass("border-t");
    });

    it("defaults to border-t as a footer", () => {
      render(
        <CapacityStrip
          as="div"
          rowLabel="Fuß"
          cells={[{ key: "mo", content: 6 }]}
        />,
      );

      expect(screen.getByRole("row")).toHaveClass("border-t");
    });
  });

  describe("reductions (F10-Andockpunkt, Kriterium 10)", () => {
    it("shows the unreduced value with no tooltip when reductions is unset", () => {
      renderStrip([{ key: "mo", content: 42 }]);

      const cell = screen.getByText("42");
      expect(cell).toBeInTheDocument();
      expect(cell.closest("td")).not.toHaveAttribute("title");
    });

    it("shows the reduced value plus a breakdown tooltip when reductions is set", () => {
      renderStrip([
        {
          key: "mo",
          content: 42,
          reductions: { excused: 3, sick: 0 },
        },
      ]);

      expect(screen.queryByText("42")).not.toBeInTheDocument();
      const cell = screen.getByText("39");
      expect(cell.closest("td")).toHaveAttribute(
        "title",
        "42 angemeldet, davon 3 abgemeldet",
      );
    });

    it("shows the unreduced value with no tooltip when reductions total zero", () => {
      renderStrip([
        {
          key: "mo",
          content: 42,
          reductions: { excused: 0, sick: 0 },
        },
      ]);

      const cell = screen.getByText("42");
      expect(cell.closest("td")).not.toHaveAttribute("title");
    });

    it("sums excused and sick into the tooltip breakdown and the reduced value", () => {
      renderStrip([
        {
          key: "mo",
          content: 42,
          reductions: { excused: 3, sick: 2 },
        },
      ]);

      const cell = screen.getByText("37");
      expect(cell.closest("td")).toHaveAttribute(
        "title",
        "42 angemeldet, davon 5 abgemeldet",
      );
    });
  });
});
