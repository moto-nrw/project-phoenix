import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { WizardStepper } from "./wizard-stepper";

const STEPS = ["Vorschau prüfen", "Freigeben", "An die Familien senden"];

describe("WizardStepper", () => {
  it("names the position and the current step by default", () => {
    render(<WizardStepper steps={STEPS} current={1} />);

    expect(screen.getByText("Schritt 2 von 3")).toBeInTheDocument();
    expect(screen.getByText("Freigeben")).toBeInTheDocument();
    expect(screen.queryByText("Vorschau prüfen")).not.toBeInTheDocument();
  });

  it("names every step in order when labeled", () => {
    render(<WizardStepper steps={STEPS} current={0} labeled />);

    expect(
      screen.getAllByRole("listitem").map((item) => item.textContent),
    ).toEqual([
      "1. Vorschau prüfen",
      "2. Freigeben",
      "3. An die Familien senden",
    ]);
    expect(screen.queryByText("Schritt 1 von 3")).not.toBeInTheDocument();
  });
});
