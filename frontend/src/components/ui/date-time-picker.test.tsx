import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("./date-picker", async (importOriginal) => ({
  ...(await importOriginal<object>()),
  ...(await import("~/test/mocks/date-picker")).isoDatePickerMock(),
}));

import { clampTimeToMin, DateTimePicker } from "./date-time-picker";

// Both halves emit through this, so it carries the boundary rule for the time
// field too. It is tested directly because jsdom blanks a time input whose
// value is below its `min` attribute, unlike a real browser, which keeps the
// typed value and only marks the field invalid.
describe("clampTimeToMin", () => {
  const min = { day: "2026-09-01", time: "10:00" };

  it("raises a time below the minimum on the boundary day", () => {
    expect(clampTimeToMin("2026-09-01", "08:00", min)).toBe("10:00");
  });

  it("keeps a time at or above the minimum on the boundary day", () => {
    expect(clampTimeToMin("2026-09-01", "10:00", min)).toBe("10:00");
    expect(clampTimeToMin("2026-09-01", "14:30", min)).toBe("14:30");
  });

  it("leaves days after the minimum unconstrained", () => {
    expect(clampTimeToMin("2026-09-02", "00:00", min)).toBe("00:00");
  });

  it("leaves the time alone when the minimum carries no clock", () => {
    expect(
      clampTimeToMin("2026-09-01", "08:00", { day: "2026-09-01", time: "" }),
    ).toBe("08:00");
  });
});

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

  it("keeps a typed time on days after the minimum", () => {
    const onChange = vi.fn();
    render(
      <DateTimePicker
        value="2026-09-02T10:00"
        onChange={onChange}
        min="2026-09-01T10:00"
        timeAriaLabel="Startzeit"
      />,
    );

    fireEvent.change(screen.getByLabelText("Startzeit"), {
      target: { value: "08:00" },
    });

    expect(onChange).toHaveBeenCalledWith("2026-09-02T08:00");
  });

  it("keeps a typed time above the minimum on the boundary day", () => {
    const onChange = vi.fn();
    render(
      <DateTimePicker
        value="2026-09-01T10:00"
        onChange={onChange}
        min="2026-09-01T10:00"
        timeAriaLabel="Startzeit"
      />,
    );

    fireEvent.change(screen.getByLabelText("Startzeit"), {
      target: { value: "14:30" },
    });

    expect(onChange).toHaveBeenCalledWith("2026-09-01T14:30");
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
