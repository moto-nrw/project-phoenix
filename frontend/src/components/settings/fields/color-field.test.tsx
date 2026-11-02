import { useState } from "react";
import { describe, expect, it } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { Button } from "~/components/ui/button";
import { ColorField, type ColorPreset } from "./color-field";

// Farbe ohne Farbcode wählen (#5239).

const PRESETS: ColorPreset[] = [
  { label: "Schwarz", value: "#111827" },
  { label: "Dunkelblau", value: "#1F4E79" },
  { label: "Rot", value: "#B91C1C" },
];

function Harness({ initial }: Readonly<{ initial: string }>) {
  const [value, setValue] = useState(initial);
  return (
    <>
      <ColorField
        ariaLabel="Farbe"
        value={value}
        fallback="#111827"
        onChange={setValue}
        presets={PRESETS}
      />
      <output aria-label="Gespeichert">{value}</output>
      <Button type="button" onClick={() => setValue("#111827")}>
        Zurücksetzen
      </Button>
    </>
  );
}

function customRadio() {
  return screen.getByRole("radio", { name: "Eigene Farbe" });
}

describe("ColorField with presets", () => {
  it("marks the stored preset and hides the colour code", () => {
    render(<Harness initial="#1F4E79" />);

    expect(screen.getByRole("radiogroup", { name: "Farbe" })).toBeVisible();
    expect(screen.getByRole("radio", { name: "Dunkelblau" })).toBeChecked();
    expect(customRadio()).not.toBeChecked();
    expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
  });

  it("matches a preset regardless of letter case", () => {
    render(<Harness initial="#1f4e79" />);

    expect(screen.getByRole("radio", { name: "Dunkelblau" })).toBeChecked();
  });

  it("stores the picked preset as #RRGGBB", () => {
    render(<Harness initial="#111827" />);

    fireEvent.click(screen.getByRole("radio", { name: "Rot" }));

    expect(screen.getByLabelText("Gespeichert")).toHaveTextContent("#B91C1C");
    expect(screen.getByRole("radio", { name: "Rot" })).toBeChecked();
  });

  it("shows a stored colour outside the presets as Eigene Farbe", () => {
    render(<Harness initial="#254798" />);

    expect(customRadio()).toBeChecked();
    for (const preset of PRESETS) {
      expect(
        screen.getByRole("radio", { name: preset.label }),
      ).not.toBeChecked();
    }
    expect(
      screen.getByLabelText("Farbe: Eigene Farbe als Farbcode"),
    ).toHaveValue("#254798");
    expect(screen.getByLabelText("Farbe: Eigene Farbe")).toHaveValue("#254798");
  });

  it("opens the colour picker under Eigene Farbe without changing the value", () => {
    render(<Harness initial="#1F4E79" />);

    fireEvent.click(customRadio());

    expect(customRadio()).toBeChecked();
    expect(screen.getByRole("radio", { name: "Dunkelblau" })).not.toBeChecked();
    expect(
      screen.getByLabelText("Farbe: Eigene Farbe als Farbcode"),
    ).toHaveValue("#1F4E79");
    expect(screen.getByLabelText("Gespeichert")).toHaveTextContent("#1F4E79");
  });

  it("keeps Eigene Farbe open while a colour is entered there", () => {
    render(<Harness initial="#254798" />);

    fireEvent.change(screen.getByLabelText("Farbe: Eigene Farbe"), {
      target: { value: "#b91c1c" },
    });

    expect(screen.getByLabelText("Gespeichert")).toHaveTextContent("#B91C1C");
    expect(customRadio()).toBeChecked();
    expect(screen.getByRole("radio", { name: "Rot" })).not.toBeChecked();
  });

  it("goes back to the presets when a preset is picked", () => {
    render(<Harness initial="#254798" />);

    fireEvent.click(screen.getByRole("radio", { name: "Schwarz" }));

    expect(screen.getByRole("radio", { name: "Schwarz" })).toBeChecked();
    expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
  });

  it("marks the preset again after a reset from outside", () => {
    render(<Harness initial="#1F4E79" />);
    fireEvent.click(customRadio());

    fireEvent.click(screen.getByRole("button", { name: "Zurücksetzen" }));

    expect(screen.getByRole("radio", { name: "Schwarz" })).toBeChecked();
    expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
  });
});
