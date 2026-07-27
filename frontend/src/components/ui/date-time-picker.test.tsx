import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("./date-picker", async (importOriginal) => ({
  ...(await importOriginal<object>()),
  ...(await import("~/test/mocks/date-picker")).isoDatePickerMock(),
}));

import { DateTimePicker } from "./date-time-picker";

describe("DateTimePicker", () => {
  it("clamps the fallback time to the minimum on the boundary day", () => {
    const onChange = vi.fn();
    render(
      <DateTimePicker
        value=""
        onChange={onChange}
        min="2026-09-01T10:00"
        dateAriaLabel="Startdatum"
      />,
    );

    fireEvent.change(screen.getByLabelText("Startdatum"), {
      target: { value: "2026-09-01" },
    });

    expect(onChange).toHaveBeenCalledWith("2026-09-01T10:00");
  });

  it("keeps the fallback time on days after the minimum", () => {
    const onChange = vi.fn();
    render(
      <DateTimePicker
        value=""
        onChange={onChange}
        min="2026-09-01T10:00"
        dateAriaLabel="Startdatum"
      />,
    );

    fireEvent.change(screen.getByLabelText("Startdatum"), {
      target: { value: "2026-09-02" },
    });

    expect(onChange).toHaveBeenCalledWith("2026-09-02T00:00");
  });

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
