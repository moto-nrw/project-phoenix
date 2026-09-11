import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import "@testing-library/jest-dom/vitest";

import { VertretungListFilter } from "./vertretung-list-filter";

// Der Filter zog mit #2031 aus der Kopfleiste der Seite in die Kopfzeile der
// Liste, die er filtert. Die Verdrahtung mit der Seite prüft
// vertretung-view.test.tsx; hier steht die Bedienung selbst.
describe("VertretungListFilter", () => {
  it("marks the active mode and labels the all-option from the caller", () => {
    render(
      <VertretungListFilter
        mode="stoerungen"
        onModeChange={vi.fn()}
        allLabel="Ganze Woche"
      />,
    );

    expect(
      screen.getByRole("button", { name: "Nur Störungen" }),
    ).toHaveAttribute("aria-pressed", "true");
    expect(
      screen.getByRole("button", { name: "Ganze Woche" }),
    ).toBeInTheDocument();
  });

  it("reports the new mode when the other tab is chosen", () => {
    const onModeChange = vi.fn();
    render(
      <VertretungListFilter
        mode="stoerungen"
        onModeChange={onModeChange}
        allLabel="Ganzer Tag"
      />,
    );

    // Radix-Tabs aktivieren per mousedown, nicht per click.
    fireEvent.click(screen.getByRole("button", { name: "Ganzer Tag" }));

    expect(onModeChange).toHaveBeenCalledWith("ganzer-tag");
  });
});
