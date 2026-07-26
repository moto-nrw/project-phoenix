import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("./date-picker", () => ({
  ISODatePicker: () => <div data-testid="date-picker" />,
}));

import { DateTimePicker } from "./date-time-picker";

describe("DateTimePicker", () => {
  it("clears an optional datetime when its time is cleared", () => {
    const onChange = vi.fn();
    render(
      <DateTimePicker
        value="2026-09-01T09:30"
        onChange={onChange}
        timeAriaLabel="Startzeit"
      />,
    );

    fireEvent.change(screen.getByLabelText("Startzeit"), {
      target: { value: "" },
    });

    expect(onChange).toHaveBeenCalledWith("");
  });
});
