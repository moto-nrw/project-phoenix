import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { ExportMenu, exportFormatOptions } from "./export-menu";

describe("ExportMenu", () => {
  it("offers one entry per format under the format names", () => {
    const onSelect = vi.fn();
    render(
      <ExportMenu
        options={exportFormatOptions(["pdf", "docx", "xlsx", "csv"])}
        onSelect={onSelect}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Herunterladen" }));
    const names = screen
      .getAllByRole("menuitem")
      .map((item) => item.textContent);
    expect(names).toEqual(["PDF", "Word", "Excel", "CSV"]);

    fireEvent.click(screen.getByRole("menuitem", { name: "Excel" }));
    expect(onSelect).toHaveBeenCalledWith({ key: "xlsx", format: "xlsx" });
  });

  it("names the Drucken entry and marks the running one", () => {
    render(
      <ExportMenu
        label="Liste erstellen"
        options={[{ key: "print" }, { key: "pdf", format: "pdf" }]}
        busyKey="pdf"
        onSelect={vi.fn()}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Liste erstellen" }));

    expect(screen.getByRole("menuitem", { name: "Drucken" })).toBeDisabled();
    expect(
      screen.getByRole("menuitem", { name: "Wird erstellt…" }),
    ).toBeDisabled();
  });

  it("does not open while the export data is unavailable", () => {
    render(
      <ExportMenu
        unavailable
        options={exportFormatOptions(["pdf"])}
        onSelect={vi.fn()}
      />,
    );

    const trigger = screen.getByRole("button", { name: "Herunterladen" });
    expect(trigger).toBeDisabled();
    fireEvent.click(trigger);
    expect(screen.queryByRole("menu")).toBeNull();
  });
});
