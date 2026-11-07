import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { NumberStepper } from "./number-stepper";

const LABELS = {
  decrementLabel: "Einen Tag weniger",
  incrementLabel: "Einen Tag mehr",
};

describe("NumberStepper", () => {
  it("reports each step and each typed value", () => {
    const onStep = vi.fn();
    const onChange = vi.fn();
    render(
      <>
        <label htmlFor="days">Tage</label>
        <NumberStepper
          id="days"
          name="days"
          value="3"
          onChange={onChange}
          onStep={onStep}
          {...LABELS}
        />
      </>,
    );

    fireEvent.click(screen.getByRole("button", { name: "Einen Tag weniger" }));
    expect(onStep).toHaveBeenLastCalledWith(-1);
    fireEvent.click(screen.getByRole("button", { name: "Einen Tag mehr" }));
    expect(onStep).toHaveBeenLastCalledWith(1);

    const field = screen.getByLabelText("Tage");
    expect(field).toHaveValue("3");
    fireEvent.change(field, { target: { value: "4" } });
    expect(onChange).toHaveBeenCalledTimes(1);
  });

  it("disables only the step the caller rules out", () => {
    render(
      <NumberStepper
        id="days"
        name="days"
        value="1"
        onChange={vi.fn()}
        onStep={vi.fn()}
        canDecrement={false}
        {...LABELS}
      />,
    );

    expect(
      screen.getByRole("button", { name: "Einen Tag weniger" }),
    ).toBeDisabled();
    expect(
      screen.getByRole("button", { name: "Einen Tag mehr" }),
    ).toBeEnabled();
    expect(screen.getByRole("textbox")).toBeEnabled();
  });

  it("disables field and both steps together", () => {
    render(
      <NumberStepper
        id="days"
        name="days"
        value="1"
        onChange={vi.fn()}
        onStep={vi.fn()}
        disabled
        {...LABELS}
      />,
    );

    expect(screen.getByRole("textbox")).toBeDisabled();
    expect(
      screen.getByRole("button", { name: "Einen Tag weniger" }),
    ).toBeDisabled();
    expect(
      screen.getByRole("button", { name: "Einen Tag mehr" }),
    ).toBeDisabled();
  });

  it("shows the field error", () => {
    render(
      <NumberStepper
        id="days"
        name="days"
        value="x"
        onChange={vi.fn()}
        onStep={vi.fn()}
        error="Bitte eine Zahl eintragen."
        {...LABELS}
      />,
    );

    expect(screen.getByText("Bitte eine Zahl eintragen.")).toBeInTheDocument();
    expect(screen.getByRole("textbox")).toHaveAttribute("aria-invalid", "true");
  });
});
