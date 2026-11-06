import { useState } from "react";
import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { TimeField } from "./time-field";

function TimeFieldDemo({ initialValue }: Readonly<{ initialValue: string }>) {
  const [value, setValue] = useState(initialValue);

  return (
    <TimeField
      value={value}
      onChange={setValue}
      label="Uhrzeit"
      hint="Uhrzeit im Format 15:30"
      placeholder="HH:MM"
    />
  );
}

it("keeps a completed time when another digit is typed", async () => {
  const user = userEvent.setup();
  render(<TimeFieldDemo initialValue="12:34" />);

  const input = screen.getByPlaceholderText("HH:MM");
  await user.type(input, "5");

  expect(input).toHaveValue("12:34");
});

it("continues a sequential one-digit-hour entry", async () => {
  const user = userEvent.setup();
  render(<TimeFieldDemo initialValue="" />);

  const input = screen.getByPlaceholderText("HH:MM");
  await user.type(input, "1530");

  expect(input).toHaveValue("15:30");
});

it("does not continue a one-digit-hour entry after leaving the field", async () => {
  const user = userEvent.setup();
  render(<TimeFieldDemo initialValue="" />);

  const input = screen.getByPlaceholderText("HH:MM") as HTMLInputElement;
  await user.type(input, "130");
  expect(input).toHaveValue("01:30");

  fireEvent.blur(input);
  input.focus();
  input.setSelectionRange(input.value.length, input.value.length);
  await user.type(input, "5");

  expect(input).toHaveValue("01:30");
});

it("names an inline field for screen readers without a visible label", () => {
  render(
    <TimeField
      inline
      value="15:30"
      onChange={() => undefined}
      label="Beginn"
      hint="Uhrzeit im Format 15:30"
      placeholder="Uhrzeit"
    />,
  );

  const input = screen.getByRole("textbox", { name: "Beginn" });
  expect(input).toHaveAccessibleDescription("Uhrzeit im Format 15:30");
  // Anker: das Feld ist über seinen Namen gefunden.
  expect(input).toHaveValue("15:30");
  expect(screen.queryByText("Beginn")).not.toBeInTheDocument();
});

it("locks a disabled field", () => {
  render(
    <TimeField
      disabled
      value=""
      onChange={() => undefined}
      label="Beginn"
      hint="Uhrzeit im Format 15:30"
      placeholder="Uhrzeit"
    />,
  );

  expect(screen.getByPlaceholderText("Uhrzeit")).toBeDisabled();
});

it("reports leaving the field", () => {
  let left = false;
  render(
    <TimeField
      value="15:30"
      onChange={() => undefined}
      onBlur={() => {
        left = true;
      }}
      label="Beginn"
      hint="Uhrzeit im Format 15:30"
      placeholder="Uhrzeit"
    />,
  );

  fireEvent.blur(screen.getByPlaceholderText("Uhrzeit"));

  expect(left).toBe(true);
});

function openPicker() {
  return userEvent.click(
    screen.getByRole("button", { name: "Uhrzeit auswählen" }),
  );
}

function hourOption(name: string) {
  return within(screen.getByRole("listbox", { name: "Stunde" })).getByRole(
    "option",
    { name },
  );
}

function minuteOption(name: string) {
  return within(screen.getByRole("listbox", { name: "Minute" })).getByRole(
    "option",
    { name },
  );
}

describe("time picker", () => {
  it("sets the field from a picked hour and minute", async () => {
    const user = userEvent.setup();
    render(<TimeFieldDemo initialValue="" />);

    await openPicker();
    await user.click(hourOption("15"));
    expect(screen.getByPlaceholderText("HH:MM")).toHaveValue("15:00");

    // Anker: die Auswahl ist offen.
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    await user.click(minuteOption("30"));

    expect(screen.getByPlaceholderText("HH:MM")).toHaveValue("15:30");
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("marks the current value and focuses its hour", async () => {
    render(<TimeFieldDemo initialValue="15:30" />);

    await openPicker();

    expect(hourOption("15")).toHaveAttribute("aria-selected", "true");
    expect(minuteOption("30")).toHaveAttribute("aria-selected", "true");
    expect(hourOption("15")).toHaveFocus();
  });

  it("marks no minute for a time outside the five-minute steps", async () => {
    render(<TimeFieldDemo initialValue="15:37" />);

    await openPicker();

    expect(hourOption("15")).toHaveAttribute("aria-selected", "true");
    const minutes = within(
      screen.getByRole("listbox", { name: "Minute" }),
    ).getAllByRole("option");
    expect(minutes).toHaveLength(12);
    for (const minute of minutes) {
      expect(minute).toHaveAttribute("aria-selected", "false");
    }
  });

  it("keeps a typed minute when only the hour is picked", async () => {
    const user = userEvent.setup();
    render(<TimeFieldDemo initialValue="15:37" />);

    await openPicker();
    await user.click(hourOption("16"));

    expect(screen.getByPlaceholderText("HH:MM")).toHaveValue("16:37");
    expect(screen.getByRole("dialog")).toBeInTheDocument();
  });

  it("completes the time when the minute is picked before the hour", async () => {
    const user = userEvent.setup();
    render(<TimeFieldDemo initialValue="" />);

    await openPicker();
    await user.click(minuteOption("45"));
    expect(screen.getByPlaceholderText("HH:MM")).toHaveValue("");
    expect(minuteOption("45")).toHaveAttribute("aria-selected", "true");

    // Anker: die Auswahl ist offen.
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    await user.click(hourOption("08"));

    expect(screen.getByPlaceholderText("HH:MM")).toHaveValue("08:45");
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("works by keyboard and returns focus to the clock", async () => {
    const user = userEvent.setup();
    render(<TimeFieldDemo initialValue="15:30" />);

    await user.tab();
    await user.tab();
    const clock = screen.getByRole("button", { name: "Uhrzeit auswählen" });
    expect(clock).toHaveFocus();

    await user.keyboard("{Enter}");
    expect(hourOption("15")).toHaveFocus();

    await user.keyboard("{ArrowDown}");
    expect(hourOption("16")).toHaveFocus();
    // Pfeile wandern nur, erst Enter übernimmt.
    expect(screen.getByPlaceholderText("HH:MM")).toHaveValue("15:30");

    await user.keyboard("{Enter}");
    expect(screen.getByPlaceholderText("HH:MM")).toHaveValue("16:30");
    expect(minuteOption("30")).toHaveFocus();

    // Anker: die Auswahl ist offen.
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    await user.keyboard("{ArrowUp}{Enter}");
    expect(screen.getByPlaceholderText("HH:MM")).toHaveValue("16:25");
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(clock).toHaveFocus();
  });

  it("closes on Escape without changing the value", async () => {
    const user = userEvent.setup();
    render(<TimeFieldDemo initialValue="15:30" />);

    await openPicker();
    // Anker: die Auswahl ist offen.
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    await user.keyboard("{ArrowDown}{Escape}");

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(screen.getByPlaceholderText("HH:MM")).toHaveValue("15:30");
    expect(
      screen.getByRole("button", { name: "Uhrzeit auswählen" }),
    ).toHaveFocus();
  });

  it("keeps Escape away from a surrounding slide-over or modal", async () => {
    const user = userEvent.setup();
    const surroundingOverlay = vi.fn();
    document.addEventListener("keydown", surroundingOverlay, true);
    render(<TimeFieldDemo initialValue="15:30" />);

    await openPicker();
    // Anker: die Auswahl ist offen.
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    await user.keyboard("{Escape}");
    document.removeEventListener("keydown", surroundingOverlay, true);

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(surroundingOverlay).not.toHaveBeenCalled();
  });

  it("keeps Tab inside the open picker", async () => {
    const user = userEvent.setup();
    render(<TimeFieldDemo initialValue="15:30" />);

    await openPicker();
    await user.tab();
    expect(minuteOption("30")).toHaveFocus();
    await user.tab();
    expect(hourOption("15")).toHaveFocus();
  });

  it("locks the clock together with the field", () => {
    render(
      <TimeField
        value="15:30"
        onChange={() => undefined}
        label="Uhrzeit"
        hint="Uhrzeit im Format 15:30"
        placeholder="HH:MM"
        disabled
      />,
    );

    const clock = screen.getByRole("button", { name: "Uhrzeit auswählen" });
    // Anker: der Knopf ist da, nur gesperrt; offen geht sonst dieselbe Auswahl auf.
    expect(clock).toBeDisabled();
    fireEvent.click(clock);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("stays in the field when a click into the input closes the picker", async () => {
    const user = userEvent.setup();
    const onBlur = vi.fn();
    render(
      <TimeField
        value="15:30"
        onChange={() => undefined}
        onBlur={onBlur}
        label="Uhrzeit"
        hint="Uhrzeit im Format 15:30"
        placeholder="HH:MM"
      />,
    );

    await openPicker();
    // Anker: die Auswahl ist offen.
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    await user.click(screen.getByPlaceholderText("HH:MM"));

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(screen.getByPlaceholderText("HH:MM")).toHaveFocus();
    expect(onBlur).not.toHaveBeenCalled();
  });

  it("does not reopen by itself after the field was locked", async () => {
    const field = (disabled: boolean) => (
      <TimeField
        value="15:30"
        onChange={() => undefined}
        label="Uhrzeit"
        hint="Uhrzeit im Format 15:30"
        placeholder="HH:MM"
        disabled={disabled}
      />
    );
    const { rerender } = render(field(false));

    await openPicker();
    // Anker: die Auswahl ist offen.
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    rerender(field(true));
    rerender(field(false));

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("stays inside a modal focus trap", async () => {
    render(
      <div data-modal-focus-scope="true" data-testid="focus-trap">
        <TimeFieldDemo initialValue="15:30" />
      </div>,
    );

    await openPicker();

    expect(
      within(screen.getByTestId("focus-trap")).getByRole("dialog", {
        name: "Uhrzeit auswählen",
      }),
    ).toBeInTheDocument();
  });

  it("portals a slide-over picker to the body and keeps its focus", async () => {
    const user = userEvent.setup();
    render(
      <div data-date-picker-focus-trap="true" data-testid="focus-trap">
        <TimeFieldDemo initialValue="15:30" />
      </div>,
    );

    await openPicker();

    expect(
      screen
        .getByRole("dialog", { name: "Uhrzeit auswählen" })
        .closest("[data-date-picker-focus-trap]"),
    ).toBeNull();
    expect(hourOption("15")).toHaveFocus();
    await user.tab();
    expect(minuteOption("30")).toHaveFocus();
  });

  it("reports leaving only when focus left field, clock and picker", async () => {
    const user = userEvent.setup();
    const onBlur = vi.fn();
    render(
      <>
        <TimeField
          value="15:30"
          onChange={() => undefined}
          onBlur={onBlur}
          label="Uhrzeit"
          hint="Uhrzeit im Format 15:30"
          placeholder="HH:MM"
        />
        <button type="button">Weiter</button>
      </>,
    );

    await user.click(screen.getByPlaceholderText("HH:MM"));
    await openPicker();
    await user.click(hourOption("16"));
    // Anker: die Auswahl ist noch offen.
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(onBlur).not.toHaveBeenCalled();

    await user.click(screen.getByRole("button", { name: "Weiter" }));

    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
    expect(onBlur).toHaveBeenCalledTimes(1);
  });
});
