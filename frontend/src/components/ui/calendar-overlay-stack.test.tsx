import { useState, type ReactNode } from "react";
import { beforeAll, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";

import { AnchoredPopover } from "./anchored-popover";
import { DatePicker } from "./date-picker";
import { DateRangePicker } from "./date-range-picker";
import { preloadDayPicker } from "./lazy-day-picker";
import { Modal } from "./modal";
import { SlideOver, SlideOverContent, SlideOverTitle } from "./slide-over";

// Both calendars render the same grid; one day button is enough to pick.
vi.mock("react-day-picker", () => ({
  DayPicker: ({
    onSelect,
    onDayClick,
  }: {
    onSelect?: (date: Date) => void;
    onDayClick?: (date: Date) => void;
  }) => (
    <div data-testid="day-picker">
      <button
        type="button"
        onClick={() => (onDayClick ?? onSelect)?.(new Date(2026, 8, 15))}
      >
        15. September
      </button>
    </div>
  ),
}));

beforeAll(() => preloadDayPicker());

// #6732: the calendars of `DatePicker` and `DateRangePicker` take their place
// from the overlay stack, like `Modal`, `SlideOver` and `CoachMark`.
const CALENDARS = [
  {
    name: "DatePicker",
    trigger: /datum auswählen/i,
    render: (onChange: () => void = vi.fn()) => (
      <DatePicker value={null} onChange={onChange} />
    ),
  },
  {
    name: "DateRangePicker",
    trigger: /zeitraum wählen/i,
    render: (onChange: () => void = vi.fn()) => (
      <DateRangePicker value={undefined} onChange={onChange} />
    ),
  },
] as const;

type Calendar = (typeof CALENDARS)[number];

async function openCalendar(calendar: Calendar) {
  fireEvent.click(screen.getByRole("button", { name: calendar.trigger }));
  await screen.findByTestId("day-picker");
}

function pressEscapeOnBody() {
  fireEvent.keyDown(document.body, { key: "Escape" });
}

function pressOn(element: Element) {
  fireEvent.pointerDown(element);
  fireEvent.mouseDown(element);
  fireEvent.click(element);
}

function Panel({
  onOpenChange,
  children,
}: {
  readonly onOpenChange: (open: boolean) => void;
  readonly children: ReactNode;
}) {
  return (
    <SlideOver open onOpenChange={onOpenChange}>
      <SlideOverContent aria-describedby={undefined}>
        <SlideOverTitle>Termin</SlideOverTitle>
        {children}
      </SlideOverContent>
    </SlideOver>
  );
}

function PopoverWithCalendar({ calendar }: { readonly calendar: Calendar }) {
  const [open, setOpen] = useState(false);
  return (
    <AnchoredPopover
      open={open}
      onOpenChange={setOpen}
      ariaLabel="Filter"
      renderTrigger={({ ref, toggle }) => (
        <button ref={ref} type="button" onClick={toggle}>
          Filter
        </button>
      )}
    >
      {() => calendar.render()}
    </AnchoredPopover>
  );
}

describe.each(CALENDARS)(
  "$name in overlay combinations (#6732)",
  (calendar) => {
    describe("a dialog opens without a click over the open calendar", () => {
      it("closes the calendar on a page and leaves Escape to the dialog", async () => {
        const closeDialog = vi.fn();
        const tree = (dialogOpen: boolean) => (
          <>
            {calendar.render()}
            <Modal isOpen={dialogOpen} onClose={closeDialog} title="Hinweis">
              <button type="button">Verstanden</button>
            </Modal>
          </>
        );
        const { rerender } = render(tree(false));
        await openCalendar(calendar);

        rerender(tree(true));

        // Anker: der Dialog ist offen.
        expect(screen.getByRole("dialog", { name: "Hinweis" })).toBeVisible();
        expect(screen.queryByTestId("day-picker")).not.toBeInTheDocument();
        pressEscapeOnBody();
        await waitFor(() => expect(closeDialog).toHaveBeenCalledTimes(1));
      });

      it("closes the calendar in a side panel and leaves Escape to the dialog", async () => {
        const closePanel = vi.fn();
        const closeDialog = vi.fn();
        const tree = (dialogOpen: boolean) => (
          <>
            <Panel onOpenChange={closePanel}>{calendar.render()}</Panel>
            <Modal isOpen={dialogOpen} onClose={closeDialog} title="Hinweis">
              <button type="button">Verstanden</button>
            </Modal>
          </>
        );
        const { rerender } = render(tree(false));
        await openCalendar(calendar);

        rerender(tree(true));

        // Anker: der Dialog ist offen.
        expect(screen.getByRole("dialog", { name: "Hinweis" })).toBeVisible();
        expect(screen.queryByTestId("day-picker")).not.toBeInTheDocument();
        pressEscapeOnBody();
        await waitFor(() => expect(closeDialog).toHaveBeenCalledTimes(1));
        expect(closePanel).not.toHaveBeenCalled();
      });
    });

    describe("a calendar in an anchored popover", () => {
      it("keeps the popover open on a press in the calendar", async () => {
        render(<PopoverWithCalendar calendar={calendar} />);
        fireEvent.click(screen.getByRole("button", { name: "Filter" }));
        await screen.findByRole("dialog", { name: "Filter" });
        await openCalendar(calendar);

        pressOn(screen.getByRole("button", { name: "15. September" }));

        expect(
          screen.getByRole("dialog", { name: "Filter" }),
        ).toBeInTheDocument();
      });

      it("keeps popover and side panel open on a press in the calendar", async () => {
        const closePanel = vi.fn();
        render(
          <Panel onOpenChange={closePanel}>
            <PopoverWithCalendar calendar={calendar} />
          </Panel>,
        );
        fireEvent.click(screen.getByRole("button", { name: "Filter" }));
        await screen.findByRole("dialog", { name: "Filter" });
        await openCalendar(calendar);

        pressOn(screen.getByRole("button", { name: "15. September" }));

        expect(
          screen.getByRole("dialog", { name: "Filter" }),
        ).toBeInTheDocument();
        expect(closePanel).not.toHaveBeenCalled();
      });
    });

    it("closes the calendar of a dialog that is suspended and leaves Escape to the dialog above", async () => {
      const closeLower = vi.fn();
      const closeUpper = vi.fn();
      const tree = (suspended: boolean) => (
        <>
          <Modal
            isOpen
            suspended={suspended}
            onClose={closeLower}
            title="Termin"
          >
            {calendar.render()}
          </Modal>
          <Modal isOpen={suspended} onClose={closeUpper} title="Bestätigen">
            <button type="button">Ja</button>
          </Modal>
        </>
      );
      const { rerender } = render(tree(false));
      await openCalendar(calendar);

      rerender(tree(true));

      // Anker: der obere Dialog ist offen.
      expect(screen.getByRole("dialog", { name: "Bestätigen" })).toBeVisible();
      expect(screen.queryByTestId("day-picker")).not.toBeInTheDocument();
      pressEscapeOnBody();
      await waitFor(() => expect(closeUpper).toHaveBeenCalledTimes(1));
      expect(closeLower).not.toHaveBeenCalled();
    });

    it("leaves Escape from the page to an anchored popover opened over the calendar", async () => {
      const changePopover = vi.fn();
      const tree = (popoverOpen: boolean) => (
        <>
          {calendar.render()}
          <AnchoredPopover
            open={popoverOpen}
            onOpenChange={changePopover}
            ariaLabel="Filter"
            renderTrigger={({ ref }) => (
              <button ref={ref} type="button">
                Filter
              </button>
            )}
          >
            {() => <p>Filterinhalt</p>}
          </AnchoredPopover>
        </>
      );
      const { rerender } = render(tree(false));
      await openCalendar(calendar);

      rerender(tree(true));

      // Popovers stay open over each other: only a key reaches both at once.
      expect(
        screen.getByRole("dialog", { name: "Filter" }),
      ).toBeInTheDocument();
      expect(screen.getByTestId("day-picker")).toBeInTheDocument();

      pressEscapeOnBody();

      expect(changePopover).toHaveBeenCalledWith(false);
      expect(screen.getByTestId("day-picker")).toBeInTheDocument();
    });

    it("orders a dialog that resumes after the calendar opened above it", async () => {
      const closeResumed = vi.fn();
      const closeCalendarDialog = vi.fn();
      // The resumed dialog stays mounted and so comes first in the DOM, before
      // the dialog whose calendar opened while it rested.
      const tree = (resumed: boolean) => (
        <>
          <Modal
            isOpen
            suspended={!resumed}
            onClose={closeResumed}
            title="Abwesenheit"
          >
            <button type="button">Speichern</button>
          </Modal>
          <Modal isOpen onClose={closeCalendarDialog} title="Termin">
            {calendar.render()}
          </Modal>
        </>
      );
      const { rerender } = render(tree(false));
      await openCalendar(calendar);

      rerender(tree(true));

      // Anker: beide Dialoge sind offen.
      expect(screen.getByRole("dialog", { name: "Abwesenheit" })).toBeVisible();
      expect(screen.getByRole("dialog", { name: "Termin" })).toBeVisible();
      expect(screen.queryByTestId("day-picker")).not.toBeInTheDocument();
      pressEscapeOnBody();
      await waitFor(() => expect(closeResumed).toHaveBeenCalledTimes(1));
      expect(closeCalendarDialog).not.toHaveBeenCalled();
    });
  },
);
