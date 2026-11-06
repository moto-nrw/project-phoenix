import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { useState } from "react";
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

function Controlled({
  initial,
  min,
  onChange,
}: Readonly<{
  initial: string;
  min?: string;
  onChange?: (value: string) => void;
}>) {
  const [value, setValue] = useState(initial);
  return (
    <DateTimePicker
      value={value}
      min={min}
      onChange={(next) => {
        setValue(next);
        onChange?.(next);
      }}
      dateAriaLabel="Tag"
      timeAriaLabel="Zeit"
    />
  );
}

describe("DateTimePicker", () => {
  it("emits nothing while the time is still being typed", () => {
    const onChange = vi.fn();
    render(
      <DateTimePicker
        value="2026-09-01T09:30"
        onChange={onChange}
        timeAriaLabel="Startzeit"
      />,
    );

    const time = screen.getByLabelText("Startzeit");
    fireEvent.change(time, { target: { value: "15" } });

    // Anker: das Feld zeigt die Eingabe.
    expect(time).toHaveValue("15");
    expect(onChange).not.toHaveBeenCalled();
  });

  it("emits nothing for a time that does not exist", () => {
    const onChange = vi.fn();
    render(
      <DateTimePicker
        value="2026-09-01T09:30"
        onChange={onChange}
        timeAriaLabel="Startzeit"
      />,
    );

    const time = screen.getByLabelText("Startzeit");
    fireEvent.change(time, { target: { value: "25:00" } });

    // Anker: das Feld zeigt die Eingabe.
    expect(time).toHaveValue("25:00");
    expect(onChange).not.toHaveBeenCalled();
  });

  it("returns to the saved time when a half-typed time is left", () => {
    render(<Controlled initial="2026-09-01T09:30" />);

    const time = screen.getByLabelText("Zeit");
    fireEvent.change(time, { target: { value: "15" } });
    fireEvent.blur(time);

    expect(time).toHaveValue("09:30");
  });

  it("raises a time below the minimum on the boundary day when left", () => {
    const onChange = vi.fn();
    render(
      <Controlled
        initial="2026-09-01T12:00"
        min="2026-09-01T10:00"
        onChange={onChange}
      />,
    );

    const time = screen.getByLabelText("Zeit");
    fireEvent.change(time, { target: { value: "08:00" } });

    // Anker: das Feld zeigt die Eingabe.
    expect(time).toHaveValue("08:00");
    expect(onChange).not.toHaveBeenCalled();

    fireEvent.blur(time);

    expect(onChange).toHaveBeenCalledWith("2026-09-01T10:00");
    expect(time).toHaveValue("10:00");
  });

  it("settles a typed time before Enter submits the form", () => {
    const onChange = vi.fn();
    render(
      <Controlled
        initial="2026-09-01T12:00"
        min="2026-09-01T10:00"
        onChange={onChange}
      />,
    );

    const time = screen.getByLabelText("Zeit");
    fireEvent.change(time, { target: { value: "08:00" } });
    fireEvent.keyDown(time, { key: "Enter" });

    expect(onChange).toHaveBeenCalledWith("2026-09-01T10:00");
    expect(time).toHaveValue("10:00");
  });

  it("falls back to the saved time when Enter meets a half-typed time", () => {
    render(<Controlled initial="2026-09-01T09:30" />);

    const time = screen.getByLabelText("Zeit");
    fireEvent.change(time, { target: { value: "15" } });
    fireEvent.keyDown(time, { key: "Enter" });

    expect(time).toHaveValue("09:30");
  });

  it("reaches a typed time on the boundary day through lower ones", async () => {
    const user = userEvent.setup();
    const onChange = vi.fn();
    render(
      <Controlled
        initial="2026-09-01T12:00"
        min="2026-09-01T10:00"
        onChange={onChange}
      />,
    );

    const time = screen.getByLabelText("Zeit");
    await user.clear(time);
    await user.type(time, "1530");

    expect(time).toHaveValue("15:30");
    expect(onChange).toHaveBeenLastCalledWith("2026-09-01T15:30");
  });

  it("keeps the day while the time is retyped after erasing it", () => {
    const onChange = vi.fn();
    render(<Controlled initial="2026-09-01T09:30" onChange={onChange} />);

    const time = screen.getByLabelText("Zeit");
    fireEvent.change(time, { target: { value: "" } });

    expect(onChange).toHaveBeenLastCalledWith("");
    expect(time).toBeEnabled();
    expect(screen.getByLabelText("Tag")).toHaveValue("2026-09-01");

    fireEvent.change(time, { target: { value: "15:30" } });

    expect(onChange).toHaveBeenLastCalledWith("2026-09-01T15:30");
  });

  it("drops the day when the field is left with an erased time", () => {
    render(<Controlled initial="2026-09-01T09:30" />);

    const time = screen.getByLabelText("Zeit");
    fireEvent.change(time, { target: { value: "" } });
    fireEvent.blur(time);

    expect(screen.getByLabelText("Tag")).toHaveValue("");
    expect(time).toBeDisabled();
  });

  it("names the time field and its format without a visible label", () => {
    render(
      <DateTimePicker
        value="2026-09-01T09:30"
        onChange={vi.fn()}
        timeAriaLabel="Startzeit"
      />,
    );

    expect(
      screen.getByRole("textbox", { name: "Startzeit" }),
    ).toHaveAccessibleDescription("Uhrzeit im Format 15:30");
  });

  it("keeps the time field locked until a day is chosen", () => {
    render(
      <DateTimePicker value="" onChange={vi.fn()} timeAriaLabel="Startzeit" />,
    );

    expect(screen.getByLabelText("Startzeit")).toBeDisabled();
  });

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
