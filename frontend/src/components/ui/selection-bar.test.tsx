import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { SelectionBar } from "./selection-bar";

describe("SelectionBar", () => {
  it("names the region and counts the selection", () => {
    render(<SelectionBar aria-label="Auswahl" count={3} />);
    expect(screen.getByRole("region", { name: "Auswahl" })).toHaveTextContent(
      "3 ausgewählt",
    );
  });

  it("takes its own label, description and actions", () => {
    render(
      <SelectionBar
        aria-label="Auswahl"
        count={2}
        label="2 von 5 ausgewählt"
        description="Zielraum wählen"
        actions={<button type="button">Verschieben</button>}
      />,
    );
    expect(screen.getByText("2 von 5 ausgewählt")).toBeInTheDocument();
    expect(screen.getByText("Zielraum wählen")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Verschieben" }),
    ).toBeInTheDocument();
  });

  it("clears the selection", () => {
    const onClear = vi.fn();
    render(<SelectionBar aria-label="Auswahl" count={1} onClear={onClear} />);
    fireEvent.click(screen.getByRole("button", { name: "Aufheben" }));
    expect(onClear).toHaveBeenCalledOnce();
  });

  it("offers no clearing while nothing is selected or an action runs", () => {
    const { rerender } = render(
      <SelectionBar aria-label="Auswahl" count={0} onClear={vi.fn()} />,
    );
    expect(screen.getByRole("button", { name: "Aufheben" })).toBeDisabled();
    rerender(
      <SelectionBar aria-label="Auswahl" count={2} onClear={vi.fn()} busy />,
    );
    expect(screen.getByRole("button", { name: "Aufheben" })).toBeDisabled();
    expect(screen.getByRole("region", { name: "Auswahl" })).toHaveAttribute(
      "aria-busy",
      "true",
    );
  });
});
