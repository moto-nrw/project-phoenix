import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { CheckboxCard } from "./checkbox-card";

describe("CheckboxCard", () => {
  it("ticks from anywhere in the row and reports the new state", () => {
    const onChange = vi.fn();
    render(
      <CheckboxCard
        checked={false}
        onChange={onChange}
        label="Nach Kind ordnen"
        hint="Eine Zwischenzeile je Kind."
      />,
    );
    fireEvent.click(screen.getByText("Eine Zwischenzeile je Kind."));
    expect(onChange).toHaveBeenCalledWith(true);
    expect(
      screen.getByRole("checkbox", { name: /Nach Kind ordnen/ }),
    ).not.toBeChecked();
  });

  it("disables the checkbox with the row", () => {
    render(
      <CheckboxCard checked onChange={vi.fn()} label="Gesperrt" disabled />,
    );
    expect(screen.getByRole("checkbox", { name: /Gesperrt/ })).toBeDisabled();
    expect(screen.getByRole("checkbox", { name: /Gesperrt/ })).toBeChecked();
  });
});
