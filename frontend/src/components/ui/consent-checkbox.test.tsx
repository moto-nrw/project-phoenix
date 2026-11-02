import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import { Button } from "./button";
import { ConsentCheckbox } from "./consent-checkbox";

describe("ConsentCheckbox (#5217)", () => {
  it("ticks the box from its text and marks it required", () => {
    const onChange = vi.fn();
    render(
      <ConsentCheckbox
        name="privacy_consent"
        label="Ich stimme zu."
        checked={false}
        required
        onChange={onChange}
      />,
    );

    const box = screen.getByRole("checkbox", { name: "Ich stimme zu." });
    expect(box).toBeRequired();
    fireEvent.click(screen.getByText("Ich stimme zu."));
    expect(onChange).toHaveBeenCalledWith(true);
  });

  it("links the refused box to its hint", () => {
    render(
      <ConsentCheckbox
        name="privacy_consent"
        label="Ich stimme zu."
        checked={false}
        error="Bitte stimmen Sie zu."
        onChange={() => {}}
      />,
    );

    const box = screen.getByRole("checkbox", { name: "Ich stimme zu." });
    expect(box).toHaveAttribute("aria-invalid", "true");
    expect(box).toHaveAccessibleDescription("Bitte stimmen Sie zu.");
  });

  it("does not tick the box when the row action is pressed", () => {
    const onChange = vi.fn();
    const onAction = vi.fn();
    render(
      <ConsentCheckbox
        name="privacy_consent"
        label="Ich stimme zu."
        checked={false}
        onChange={onChange}
        action={
          <Button
            type="button"
            variant="ghost"
            size="compact"
            onClick={onAction}
          >
            Details
          </Button>
        }
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: "Details" }));
    expect(onAction).toHaveBeenCalledOnce();
    expect(onChange).not.toHaveBeenCalled();
  });
});
