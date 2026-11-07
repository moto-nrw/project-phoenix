import { useState } from "react";
import { describe, expect, it } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { TextPresetField, type TextPreset } from "./text-preset-field";

// Name der Betreuung: OGS, GTS oder ein eigener Name (#6042).

const PRESETS: TextPreset[] = [
  { label: "OGS", value: "OGS" },
  { label: "GTS", value: "GTS" },
];

function Harness({ initial }: Readonly<{ initial: string }>) {
  const [value, setValue] = useState(initial);
  const [picked, setPicked] = useState("");
  return (
    <>
      <TextPresetField
        ariaLabel="Name der Betreuung"
        value={value}
        onChange={setValue}
        onPick={(next) => {
          setPicked(next);
          setValue(next);
        }}
        presets={PRESETS}
        customLabel="Eigener Name"
      />
      <output aria-label="Gespeichert">{value}</output>
      <output aria-label="Sofort gespeichert">{picked}</output>
    </>
  );
}

describe("TextPresetField", () => {
  it("marks the stored preset and hides the text field", () => {
    render(<Harness initial="OGS" />);

    expect(
      screen.getByRole("radiogroup", { name: "Name der Betreuung" }),
    ).toBeVisible();
    expect(screen.getByRole("radio", { name: "OGS" })).toBeChecked();
    expect(
      screen.getByRole("radio", { name: "Eigener Name" }),
    ).not.toBeChecked();
    expect(screen.queryByRole("textbox")).not.toBeInTheDocument();
  });

  it("saves a picked preset right away", () => {
    render(<Harness initial="OGS" />);

    fireEvent.click(screen.getByRole("radio", { name: "GTS" }));

    expect(screen.getByLabelText("Sofort gespeichert")).toHaveTextContent(
      "GTS",
    );
    expect(screen.getByRole("radio", { name: "GTS" })).toBeChecked();
  });

  it("shows a stored own name in the open text field", () => {
    render(<Harness initial="Ganztag Nord" />);

    expect(screen.getByRole("radio", { name: "Eigener Name" })).toBeChecked();
    expect(
      screen.getByRole("textbox", { name: "Name der Betreuung: Eigener Name" }),
    ).toHaveValue("Ganztag Nord");
  });

  it("keeps the text field open while the own name is typed", () => {
    render(<Harness initial="OGS" />);

    fireEvent.click(screen.getByRole("radio", { name: "Eigener Name" }));
    const input = screen.getByRole("textbox", {
      name: "Name der Betreuung: Eigener Name",
    });
    fireEvent.change(input, { target: { value: "GTS" } });

    expect(screen.getByLabelText("Gespeichert")).toHaveTextContent("GTS");
    expect(screen.getByRole("radio", { name: "Eigener Name" })).toBeChecked();
    expect(input).toBeVisible();
  });
});
