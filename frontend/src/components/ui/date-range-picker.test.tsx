import { schoolDateCases, setSchoolDateClock } from "~/test/school-date-cases";
import { afterEach as restoreZoneAfterEach, vi as zoneVi } from "vitest";
restoreZoneAfterEach(() => zoneVi.unstubAllEnvs());
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
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
import { SlideOver, SlideOverContent } from "./slide-over";
import { AnchoredPopover } from "./anchored-popover";

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

  it("closes the calendar with Escape on a page and restores trigger focus", async () => {
    // Wie `DatePicker`: Escape schließt den Kalender auch außerhalb von Dialogen.
    const parentKeyDown = vi.fn();
    render(
      <div role="group" aria-label="Filter" onKeyDown={parentKeyDown}>
        <DateRangePicker value={undefined} onChange={vi.fn()} />
      </div>,
    );
    const trigger = screen.getByRole("button", { name: "Zeitraum wählen" });
    fireEvent.click(trigger);
    await waitFor(() => {
      expect(document.querySelector("[data-range-panel]")).toBeInTheDocument();
    });

    fireEvent.keyDown(trigger, { key: "Escape" });

    expect(document.querySelector("[data-range-panel]")).toBeNull();
    expect(trigger).toHaveFocus();
    expect(parentKeyDown).not.toHaveBeenCalled();
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

  it("closes the range picker whose calendar receives Escape", async () => {
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

    fireEvent.keyDown(panels[1]!, { key: "Escape" });

    expect(document.querySelectorAll("[data-range-panel]")).toHaveLength(1);
    expect(document.querySelector("[data-range-panel]")).toBe(panels[0]!);
    expect(triggers[1]).toHaveFocus();
  });

  it("closes the calendar with Escape when no control has focus", async () => {
    // Safari fokussiert einen geklickten Button nicht: Escape kommt dann vom body.
    render(<DateRangePicker value={undefined} onChange={vi.fn()} />);
    fireEvent.click(screen.getByRole("button", { name: "Zeitraum wählen" }));
    await waitFor(() => {
      expect(document.querySelector("[data-range-panel]")).toBeInTheDocument();
    });

    fireEvent.keyDown(document.body, { key: "Escape" });

    expect(document.querySelector("[data-range-panel]")).toBeNull();
  });

  it("closes the last opened calendar with Escape when no control has focus", async () => {
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

    fireEvent.keyDown(document.body, { key: "Escape" });

    expect(document.querySelectorAll("[data-range-panel]")).toHaveLength(1);
    expect(document.querySelector("[data-range-panel]")).toBe(panels[0]!);
    expect(triggers[1]).toHaveFocus();
  });

  it("leaves Escape in a higher modal to that modal", async () => {
    const onClose = vi.fn();
    render(
      <>
        <DateRangePicker value={undefined} onChange={vi.fn()} />
        <Modal isOpen onClose={onClose} title="Bestätigung">
          <button type="button">Abbrechen</button>
        </Modal>
      </>,
    );
    fireEvent.click(screen.getByRole("button", { name: "Zeitraum wählen" }));
    await waitFor(() => {
      expect(document.querySelector("[data-range-panel]")).toBeInTheDocument();
    });

    fireEvent.keyDown(screen.getByRole("dialog", { name: "Bestätigung" }), {
      key: "Escape",
    });

    await waitFor(() => {
      expect(onClose).toHaveBeenCalledOnce();
    });
    expect(document.querySelector("[data-range-panel]")).toBeInTheDocument();
  });

  it.each([
    [
      "Modal",
      (onClose: () => void) => (
        <Modal isOpen onClose={onClose} title="Bestätigung">
          <button type="button">Abbrechen</button>
        </Modal>
      ),
    ],
    [
      "Slide-over",
      (onClose: () => void) => (
        <SlideOver defaultOpen onOpenChange={(open) => !open && onClose()}>
          <SlideOverContent>Details</SlideOverContent>
        </SlideOver>
      ),
    ],
  ])(
    "leaves body-level Escape to a higher %s",
    async (_overlayName, renderOverlay) => {
      const onClose = vi.fn();
      const onChange = vi.fn();
      const { rerender } = render(
        <DateRangePicker value={undefined} onChange={onChange} />,
      );
      fireEvent.click(screen.getByRole("button", { name: "Zeitraum wählen" }));
      await waitFor(() => {
        expect(
          document.querySelector("[data-range-panel]"),
        ).toBeInTheDocument();
      });
      rerender(
        <>
          <DateRangePicker value={undefined} onChange={onChange} />
          {renderOverlay(onClose)}
        </>,
      );

      fireEvent.keyDown(document.body, { key: "Escape" });

      await waitFor(() => {
        expect(onClose).toHaveBeenCalledOnce();
      });
      expect(document.querySelector("[data-range-panel]")).toBeInTheDocument();
    },
  );
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

  it("leaves Escape from a select search field to the open select", async () => {
    render(
      <Modal isOpen onClose={vi.fn()} title="Termine absagen">
        <DateRangePicker value={undefined} onChange={vi.fn()} />
        <CustomSelect
          ariaLabel="Grund"
          searchable
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
    const search = await screen.findByRole("textbox", { name: "Suchen" });

    fireEvent.keyDown(search, { key: "Escape" });

    expect(select).toHaveAttribute("aria-expanded", "false");
    expect(document.querySelector("[data-range-panel]")).toBeInTheDocument();
  });

  it("keeps the calendar reachable by keyboard inside the dialog's focus trap", async () => {
    renderInModal();
    const preset = await screen.findByRole("button", { name: "Letzte 7 Tage" });

    preset.focus();

    expect(preset).toHaveFocus();
  });

  it("moves on with Tab inside the dialog like DatePicker", async () => {
    const user = userEvent.setup();
    renderInModal();
    const preset = await screen.findByRole("button", { name: "Letzte 7 Tage" });
    const previousMonth = screen.getByRole("button", {
      name: "Vorheriger Monat",
    });
    preset.focus();

    await user.tab();

    expect(previousMonth).toHaveFocus();
    expect(preset.closest("[data-modal-focus-scope]")).not.toBeNull();
  });

  it("keeps the calendar reachable by Tab inside a slide-over", async () => {
    const user = userEvent.setup();
    render(
      <SlideOver defaultOpen>
        <SlideOverContent>
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
        </SlideOverContent>
      </SlideOver>,
    );
    await user.click(screen.getByRole("button", { name: "Zeitraum wählen" }));
    const preset = await screen.findByRole("button", { name: "Letzte 7 Tage" });
    const previousMonth = screen.getByRole("button", {
      name: "Vorheriger Monat",
    });

    expect(preset).toHaveFocus();
    await user.tab();

    expect(previousMonth).toHaveFocus();
  });

  it("keeps its portaled panel clickable inside a slide-over", async () => {
    const onChange = vi.fn();
    render(
      <SlideOver defaultOpen>
        <SlideOverContent>
          <DateRangePicker
            value={undefined}
            onChange={onChange}
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
        </SlideOverContent>
      </SlideOver>,
    );
    fireEvent.click(screen.getByRole("button", { name: "Zeitraum wählen" }));
    await waitFor(() => {
      expect(document.querySelector("[data-range-panel]")).toBeInTheDocument();
    });
    const panel = document.querySelector("[data-range-panel]");

    expect(panel).toHaveStyle({ pointerEvents: "auto" });
    fireEvent.click(screen.getByRole("button", { name: "Letzte 7 Tage" }));

    expect(onChange).toHaveBeenCalledWith({
      from: new Date(2026, 8, 3),
      to: new Date(2026, 8, 9),
    });
  });

  it("keeps the slide-over open on a pointer press and focus in its calendar", async () => {
    const onOpenChange = vi.fn();
    render(
      <SlideOver defaultOpen onOpenChange={onOpenChange}>
        <SlideOverContent>
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
        </SlideOverContent>
      </SlideOver>,
    );
    fireEvent.click(screen.getByRole("button", { name: "Zeitraum wählen" }));
    const preset = await screen.findByRole("button", { name: "Letzte 7 Tage" });

    fireEvent.pointerDown(preset);
    fireEvent.focusIn(preset);

    expect(onOpenChange).not.toHaveBeenCalledWith(false);
    expect(document.querySelector("[data-range-panel]")).toBeInTheDocument();
  });

  it("closes its calendar first on a body Escape when its modal lies above a slide-over", async () => {
    const onClose = vi.fn();
    function Stack({ modalOpen }: Readonly<{ modalOpen: boolean }>) {
      return (
        <>
          <SlideOver defaultOpen>
            <SlideOverContent>
              <p>Liste</p>
            </SlideOverContent>
          </SlideOver>
          <Modal isOpen={modalOpen} onClose={onClose} title="Termine absagen">
            <DateRangePicker value={undefined} onChange={vi.fn()} />
          </Modal>
        </>
      );
    }
    const { rerender } = render(<Stack modalOpen={false} />);
    await screen.findByText("Liste");
    rerender(<Stack modalOpen />);
    // The slide-over hides its siblings from the accessibility tree in jsdom.
    fireEvent.click(
      await screen.findByRole("button", {
        name: "Zeitraum wählen",
        hidden: true,
      }),
    );
    await waitFor(() => {
      expect(document.querySelector("[data-range-panel]")).toBeInTheDocument();
    });

    fireEvent.keyDown(document.body, { key: "Escape" });

    expect(document.querySelector("[data-range-panel]")).toBeNull();
    expect(onClose).not.toHaveBeenCalled();
  });

  it("leaves a body Escape to a higher modal that lies earlier in the document", async () => {
    const onClose = vi.fn();
    render(
      <>
        <Modal isOpen onClose={onClose} title="Bestätigung">
          <p>Wirklich?</p>
        </Modal>
        <SlideOver defaultOpen>
          <SlideOverContent>
            <DateRangePicker value={undefined} onChange={vi.fn()} />
          </SlideOverContent>
        </SlideOver>
      </>,
    );
    fireEvent.click(
      await screen.findByRole("button", {
        name: "Zeitraum wählen",
        hidden: true,
      }),
    );
    await waitFor(() => {
      expect(document.querySelector("[data-range-panel]")).toBeInTheDocument();
    });

    fireEvent.keyDown(document.body, { key: "Escape" });

    await waitFor(() => {
      expect(onClose).toHaveBeenCalledOnce();
    });
    expect(document.querySelector("[data-range-panel]")).toBeInTheDocument();
  });

  it("leaves Escape from an open anchored popover to that popover", async () => {
    const onOpenChange = vi.fn();
    render(
      <>
        <DateRangePicker value={undefined} onChange={vi.fn()} />
        <AnchoredPopover
          open
          onOpenChange={onOpenChange}
          ariaLabel="Hinweise"
          renderTrigger={({ ref }) => (
            <button ref={ref} type="button">
              Hinweise
            </button>
          )}
        >
          {() => <button type="button">Im Popover</button>}
        </AnchoredPopover>
      </>,
    );
    fireEvent.click(screen.getByRole("button", { name: "Zeitraum wählen" }));
    await waitFor(() => {
      expect(document.querySelector("[data-range-panel]")).toBeInTheDocument();
    });

    fireEvent.keyDown(screen.getByRole("button", { name: "Im Popover" }), {
      key: "Escape",
    });

    expect(onOpenChange).toHaveBeenCalledWith(false);
    expect(document.querySelector("[data-range-panel]")).toBeInTheDocument();
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
