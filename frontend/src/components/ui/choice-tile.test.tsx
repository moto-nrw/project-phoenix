import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { Checkbox } from "./checkbox";
import { ChoiceTile } from "./choice-tile";

describe("ChoiceTile", () => {
  it("rendert als label, damit die Checkbox über die ganze Kachel klickbar ist", () => {
    const onChange = vi.fn();
    render(
      <ChoiceTile>
        <Checkbox checked={false} onChange={onChange} />
        Montag
      </ChoiceTile>,
    );

    fireEvent.click(screen.getByText("Montag"));

    expect(onChange).toHaveBeenCalledTimes(1);
    expect(screen.getByText("Montag").closest("label")).toHaveClass(
      "rounded-xl",
      "border",
      "border-gray-200",
      "bg-white",
      "cursor-pointer",
    );
  });

  it("rendert als button ohne submit-Verhalten und reicht onClick durch", () => {
    const onClick = vi.fn();
    render(
      <ChoiceTile as="button" onClick={onClick} aria-pressed={false}>
        Text eingeben
      </ChoiceTile>,
    );

    const button = screen.getByRole("button", { name: "Text eingeben" });
    fireEvent.click(button);

    expect(button).toHaveAttribute("type", "button");
    expect(onClick).toHaveBeenCalledTimes(1);
  });

  it("rendert als div für Zeilen mit eigenem Steuerelement", () => {
    render(
      <ChoiceTile as="div" data-testid="row">
        Zeile
      </ChoiceTile>,
    );

    const row = screen.getByTestId("row");
    expect(row.tagName).toBe("DIV");
    expect(row).not.toHaveClass("cursor-pointer");
  });

  it("färbt den gewählten Zustand nach tone", () => {
    const { rerender } = render(
      <ChoiceTile data-testid="tile" selected>
        Gewählt
      </ChoiceTile>,
    );
    const tile = screen.getByTestId("tile");
    expect(tile.className).toContain("border-gray-300 bg-gray-50");
    expect(tile.className).not.toContain("bg-white");

    rerender(
      <ChoiceTile data-testid="tile" selected tone="green">
        Gewählt
      </ChoiceTile>,
    );
    expect(tile.className).toContain("border-moto-green/50 bg-moto-green/10");

    rerender(
      <ChoiceTile data-testid="tile" selected tone="blue">
        Gewählt
      </ChoiceTile>,
    );
    expect(tile.className).toContain("border-moto-blue bg-moto-blue/10");
  });

  it("schaltet bei disabled Hover und Fokusring ab", () => {
    render(
      <ChoiceTile as="button" disabled>
        Gesperrt
      </ChoiceTile>,
    );

    const button = screen.getByRole("button", { name: "Gesperrt" });
    expect(button).toBeDisabled();
    expect(button).toHaveClass("cursor-not-allowed", "opacity-60");
    expect(button).not.toHaveClass("hover:bg-gray-50");
    expect(button).not.toHaveClass("focus-visible:ring-2");
  });

  it("lässt Padding und Rahmenfarbe über className überschreiben", () => {
    render(
      <ChoiceTile data-testid="tile" className="border-moto-red/20 p-4">
        Zeile
      </ChoiceTile>,
    );
    const { className } = screen.getByTestId("tile");

    expect(className).toContain("p-4");
    expect(className).not.toContain("px-3");
    expect(className).toContain("border-moto-red/20");
    expect(className).not.toContain("border-gray-200");
  });
});
