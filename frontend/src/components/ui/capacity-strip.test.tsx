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
});
