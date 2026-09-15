import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { Checkbox } from "./checkbox";
import { ChoiceTile, choiceTileClassName } from "./choice-tile";

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
    expect(choiceTileClassName({ selected: true })).toContain(
      "border-gray-300 bg-gray-50",
    );
    expect(choiceTileClassName({ selected: true, tone: "green" })).toContain(
      "border-moto-green/50 bg-moto-green/10",
    );
    expect(choiceTileClassName({ selected: true, tone: "blue" })).toContain(
      "border-moto-blue bg-moto-blue/10",
    );
    expect(choiceTileClassName({ selected: true })).not.toContain("bg-white");
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
    const className = choiceTileClassName({
      className: "border-moto-red/20 p-4",
    });

    expect(className).toContain("p-4");
    expect(className).not.toContain("px-3");
    expect(className).toContain("border-moto-red/20");
    expect(className).not.toContain("border-gray-200");
  });
});
