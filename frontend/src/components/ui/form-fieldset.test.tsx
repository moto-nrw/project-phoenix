import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";

import { FormFieldset } from "./form-fieldset";

describe("FormFieldset", () => {
  it("names the group by its legend", () => {
    render(
      <FormFieldset legend="Wochentage">
        <input aria-label="Montag" />
      </FormFieldset>,
    );
    expect(screen.getByRole("group", { name: "Wochentage" })).toContainElement(
      screen.getByLabelText("Montag"),
    );
  });

  it("marks a required group without changing its name", () => {
    render(<FormFieldset legend="Wochentage" required />);
    // The star is aria-hidden, the name stays the word itself.
    expect(screen.getByRole("group", { name: "Wochentage" })).toBeVisible();
    expect(screen.getByText("*", { exact: false })).toBeInTheDocument();
  });

  it("describes the group by its hint", () => {
    render(
      <FormFieldset legend="Wochentage" hint="Mindestens einen wählen." />,
    );
    expect(screen.getByRole("group")).toHaveAccessibleDescription(
      "Mindestens einen wählen.",
    );
    expect(screen.getByRole("group")).not.toHaveAttribute("aria-invalid");
  });

  it("marks the group invalid and links the error", () => {
    render(
      <FormFieldset legend="Wochentage" error="Bitte einen Tag wählen." />,
    );
    const group = screen.getByRole("group");
    expect(group).toHaveAttribute("aria-invalid", "true");
    expect(group).toHaveAccessibleDescription("Bitte einen Tag wählen.");
    expect(screen.getByText("Bitte einen Tag wählen.")).toBeInTheDocument();
  });

  it("uses the caller's error id and passes native fieldset props on", () => {
    render(
      <FormFieldset
        legend="Wochentage"
        error="Fehlt."
        errorId="weekday-error"
        disabled
        id="weekdays"
      />,
    );
    const group = screen.getByRole("group");
    expect(group).toBeDisabled();
    expect(group).toHaveAttribute("id", "weekdays");
    expect(screen.getByText("Fehlt.")).toHaveAttribute("id", "weekday-error");
  });
});
