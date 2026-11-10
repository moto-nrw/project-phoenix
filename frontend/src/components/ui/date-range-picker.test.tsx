import { schoolDateCases, setSchoolDateClock } from "~/test/school-date-cases";
import { afterEach as restoreZoneAfterEach, vi as zoneVi } from "vitest";
restoreZoneAfterEach(() => zoneVi.unstubAllEnvs());
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

vi.mock("react-day-picker", () => ({
  DayPicker: ({ month }: { month: Date }) => (
    <div
      data-testid="range-day-picker"
      data-month={`${month.getFullYear()}-${month.getMonth() + 1}`}
    />
  ),
}));

import { DateRangePicker, RangeCalendarInline } from "./date-range-picker";
import { CustomSelect } from "./custom-select";
import { Modal } from "./modal";

describe("DateRangePicker", () => {
  it("collapses the label when both dates share month and year", () => {
    // Kurz genug für den Chip zwischen den Pfeilen; vorher rutschte auf dem
    // Telefon das Jahr in eine zweite Zeile.
    const { rerender } = render(
      <DateRangePicker
        value={{ from: new Date(2026, 7, 1), to: new Date(2026, 7, 30) }}
        onChange={() => undefined}
      />,
    );
    expect(screen.getByRole("button", { name: /Aug/ })).toHaveTextContent(
      "1.–30. Aug. 2026",
    );

    rerender(
      <DateRangePicker
        value={{ from: new Date(2026, 6, 20), to: new Date(2026, 7, 30) }}
        onChange={() => undefined}
      />,
    );
    expect(screen.getByRole("button", { name: /Aug/ })).toHaveTextContent(
      "20. Juli – 30. Aug. 2026",
    );
  });

  it("opens the calendar from the trigger between the arrows", async () => {
    render(
      <DateRangePicker
        embedded
        value={{ from: new Date(2026, 7, 1), to: new Date(2026, 7, 30) }}
        onChange={() => undefined}
      />,
    );

    fireEvent.click(screen.getByRole("button", { name: /Aug/ }));

    await waitFor(() => {
      expect(document.querySelector("[data-range-panel]")).toBeInTheDocument();
    });
  });

  it("marks the normal trigger while its calendar is open", () => {
    render(
      <DateRangePicker
        value={{ from: new Date(2026, 7, 1), to: new Date(2026, 7, 30) }}
        onChange={() => undefined}
      />,
    );

    const trigger = screen.getByRole("button", { name: /Aug/ });
    expect(trigger).not.toHaveClass("bg-gray-50!");

    fireEvent.click(trigger);

    expect(trigger).toHaveClass("bg-gray-50!");
  });

  it("shows a single day as one date", () => {
    render(
      <DateRangePicker
        value={{ from: new Date(2026, 9, 6), to: new Date(2026, 9, 6) }}
        onChange={() => undefined}
      />,
    );
    const trigger = screen.getByRole("button", { name: /Okt/ });
    expect(trigger).toHaveTextContent("6. Okt. 2026");
    expect(trigger).not.toHaveTextContent("6.–6.");
  });

  it("keeps its own panel open when another range picker is also open", async () => {
    render(
      <>
        <DateRangePicker value={undefined} onChange={vi.fn()} />
        <DateRangePicker value={undefined} onChange={vi.fn()} />
      </>,
    );

    const triggers = screen.getAllByRole("button", {
      name: /zeitraum wählen/i,
    });
    fireEvent.click(triggers[0]!);
    fireEvent.click(triggers[1]!);

    await waitFor(() => {
      expect(document.querySelectorAll("[data-range-panel]")).toHaveLength(2);
    });

    const panels = document.querySelectorAll("[data-range-panel]");
    fireEvent.mouseDown(panels[1]!);

    expect(document.querySelectorAll("[data-range-panel]")).toHaveLength(1);
    expect(document.querySelector("[data-range-panel]")).toBe(panels[1]!);
  });
});

describe("DateRangePicker in a dialog", () => {
  function renderInModal(onClose = vi.fn()) {
    render(
      <Modal isOpen onClose={onClose} title="Termine absagen">
        <DateRangePicker
          value={undefined}
          onChange={vi.fn()}
          presets={[
            {
              label: "Letzte 7 Tage",
              range: () => ({
                from: new Date(2026, 8, 3),
                to: new Date(2026, 8, 9),
              }),
            },
          ]}
        />
      </Modal>,
    );
    const trigger = screen.getByRole("button", { name: "Zeitraum wählen" });
    fireEvent.click(trigger);
    return { trigger, onClose };
  }

  it("closes only the calendar on Escape and returns focus to its trigger", async () => {
    const { trigger, onClose } = renderInModal();
    const preset = await screen.findByRole("button", { name: "Letzte 7 Tage" });

    fireEvent.keyDown(preset, { key: "Escape" });

    expect(document.querySelector("[data-range-panel]")).toBeNull();
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(onClose).not.toHaveBeenCalled();
    expect(trigger).toHaveFocus();
  });

  it("leaves Escape to an open select in the same dialog", async () => {
    render(
      <Modal isOpen onClose={vi.fn()} title="Termine absagen">
        <DateRangePicker value={undefined} onChange={vi.fn()} />
        <CustomSelect
          ariaLabel="Grund"
          value=""
          options={[{ value: "krank", label: "Krank" }]}
          onChange={vi.fn()}
        />
      </Modal>,
    );
    fireEvent.click(screen.getByRole("button", { name: "Zeitraum wählen" }));
    await waitFor(() => {
      expect(document.querySelector("[data-range-panel]")).toBeInTheDocument();
    });
    const select = screen.getByRole("combobox", { name: "Grund" });
    fireEvent.keyDown(select, { key: "ArrowDown" });
    expect(select).toHaveAttribute("aria-expanded", "true");

    fireEvent.keyDown(select, { key: "Escape" });

    expect(select).toHaveAttribute("aria-expanded", "false");
    expect(document.querySelector("[data-range-panel]")).toBeInTheDocument();
  });

  it("keeps the calendar reachable by keyboard inside the dialog's focus trap", async () => {
    renderInModal();
    const preset = await screen.findByRole("button", { name: "Letzte 7 Tage" });

    preset.focus();

    expect(preset).toHaveFocus();
  });
});

it.each(schoolDateCases("2026-06-01"))(
  "opens the Berlin month in %s",
  (zone, instant) => {
    setSchoolDateClock(zone, instant, "2026-06-01");
    render(<RangeCalendarInline value={undefined} onChange={vi.fn()} />);
    for (const picker of screen.getAllByTestId("range-day-picker"))
      expect(picker).toHaveAttribute("data-month", "2026-6");
  },
);
