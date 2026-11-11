import { fireEvent, render, screen } from "@testing-library/react";
import { useState, type ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";

vi.mock("react-day-picker", () => ({
  DayPicker: ({ month }: { month: Date }) => (
    <div
      data-testid="day-grid"
      data-month={`${month.getFullYear()}-${month.getMonth() + 1}`}
    />
  ),
}));

import { AnchoredPopover } from "./anchored-popover";
import { DatePicker } from "./date-picker";
import { DateRangePicker } from "./date-range-picker";
import { ListboxDropdown } from "./listbox-dropdown";
import { Modal } from "./modal";

/**
 * Every panel type on the engine (`useAnchoredPanel`) behaves the same: a
 * press outside closes it, Escape closes only the panel, and in a dialog the
 * panel stays usable (#6576).
 */
interface PanelCase {
  readonly name: string;
  readonly render: () => ReactNode;
  /** Opens the panel; resolves to an element inside it. */
  readonly open: () => Promise<HTMLElement>;
  readonly isOpen: () => boolean;
  /** Uses the open panel; the use shows. */
  readonly use: () => Promise<void>;
  /** A single select closes once an option is chosen. */
  readonly closesOnUse?: boolean;
  /** A select leaves Escape from elsewhere alone: its trigger handles keys. */
  readonly ignoresPageEscape?: boolean;
}

function Popover() {
  const [open, setOpen] = useState(false);
  const [count, setCount] = useState(0);
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
      {() => (
        <button type="button" onClick={() => setCount((value) => value + 1)}>
          {`Gezählt: ${count}`}
        </button>
      )}
    </AnchoredPopover>
  );
}

function Select() {
  const [value, setValue] = useState("a");
  return (
    <ListboxDropdown
      ariaLabel="Gruppe"
      value={value}
      onChange={setValue}
      options={[
        { value: "a", label: "Sterne" },
        { value: "b", label: "Monde" },
      ]}
    />
  );
}

function Calendar() {
  const [value, setValue] = useState<Date | null>(new Date(2026, 8, 9));
  return (
    <DatePicker value={value} onChange={setValue} calendarLayout="popover" />
  );
}

function monthShown(): string | null {
  return screen.getAllByTestId("day-grid")[0]!.getAttribute("data-month");
}

const CASES: readonly PanelCase[] = [
  {
    name: "AnchoredPopover",
    render: () => <Popover />,
    open: async () => {
      fireEvent.click(screen.getByRole("button", { name: "Filter" }));
      return screen.findByRole("button", { name: "Gezählt: 0" });
    },
    isOpen: () => screen.queryByRole("dialog", { name: "Filter" }) !== null,
    use: async () => {
      fireEvent.mouseDown(screen.getByRole("button", { name: "Gezählt: 0" }));
      fireEvent.click(screen.getByRole("button", { name: "Gezählt: 0" }));
      await screen.findByRole("button", { name: "Gezählt: 1" });
    },
  },
  {
    name: "ListboxDropdown",
    render: () => <Select />,
    open: async () => {
      fireEvent.click(screen.getByRole("button", { name: "Gruppe" }));
      return screen.findByRole("option", { name: "Sterne" });
    },
    isOpen: () => screen.queryByRole("listbox") !== null,
    use: async () => {
      const option = screen.getByRole("option", { name: "Monde" });
      fireEvent.pointerDown(option);
      fireEvent.mouseDown(option);
      fireEvent.click(option);
      expect(screen.getByRole("button", { name: "Gruppe" })).toHaveTextContent(
        "Monde",
      );
    },
    closesOnUse: true,
    ignoresPageEscape: true,
  },
  {
    name: "DatePicker",
    render: () => <Calendar />,
    open: async () => {
      fireEvent.click(screen.getByRole("button", { name: "09.09.2026" }));
      return screen.findByRole("button", { name: "Nächster Monat" });
    },
    isOpen: () => screen.queryByTestId("day-grid") !== null,
    use: async () => {
      const next = screen.getByRole("button", { name: "Nächster Monat" });
      fireEvent.pointerDown(next);
      fireEvent.mouseDown(next);
      fireEvent.click(next);
      expect(monthShown()).toBe("2026-10");
    },
  },
  {
    name: "DateRangePicker",
    render: () => <DateRangePicker value={undefined} onChange={vi.fn()} />,
    open: async () => {
      fireEvent.click(screen.getByRole("button", { name: "Zeitraum wählen" }));
      return screen.findByRole("button", { name: "Nächster Monat" });
    },
    isOpen: () => screen.queryByTestId("day-grid") !== null,
    use: async () => {
      const next = screen.getByRole("button", { name: "Nächster Monat" });
      fireEvent.pointerDown(next);
      fireEvent.mouseDown(next);
      fireEvent.click(next);
      expect(monthShown()).toBe("2026-10");
    },
  },
];

describe.each(CASES)("$name on the panel engine", (panel) => {
  it("closes on a press outside", async () => {
    render(
      <>
        <p>Seite</p>
        {panel.render()}
      </>,
    );
    await panel.open();

    fireEvent.pointerDown(screen.getByText("Seite"));
    fireEvent.mouseDown(screen.getByText("Seite"));

    expect(panel.isOpen()).toBe(false);
  });

  it("closes only the panel on Escape in a dialog", async () => {
    const onClose = vi.fn();
    render(
      <Modal isOpen onClose={onClose} title="Termin">
        {panel.render()}
      </Modal>,
    );
    const inside = await panel.open();
    inside.focus();

    fireEvent.keyDown(inside, { key: "Escape" });

    expect(panel.isOpen()).toBe(false);
    expect(screen.getByRole("dialog", { name: "Termin" })).toBeInTheDocument();
    expect(onClose).not.toHaveBeenCalled();
  });

  it("leaves the dialog open on an Escape from the page", async () => {
    const onClose = vi.fn();
    render(
      <Modal isOpen onClose={onClose} title="Termin">
        {panel.render()}
      </Modal>,
    );
    await panel.open();

    fireEvent.keyDown(document.body, { key: "Escape" });

    expect(panel.isOpen()).toBe(panel.ignoresPageEscape === true);
    expect(onClose).not.toHaveBeenCalled();
  });

  it("stays usable in a dialog", async () => {
    const onClose = vi.fn();
    render(
      <Modal isOpen onClose={onClose} title="Termin">
        {panel.render()}
      </Modal>,
    );
    await panel.open();

    await panel.use();

    expect(panel.isOpen()).toBe(!panel.closesOnUse);
    expect(onClose).not.toHaveBeenCalled();
  });
});

describe("outside presses on a phone", () => {
  it("keeps a select open while a finger starts to scroll the page, and closes it on a tap", async () => {
    render(
      <>
        <p>Seite</p>
        <Select />
      </>,
    );
    fireEvent.click(screen.getByRole("button", { name: "Gruppe" }));
    await screen.findByRole("listbox");

    fireEvent.pointerDown(screen.getByText("Seite"));
    fireEvent.touchStart(screen.getByText("Seite"));

    expect(screen.getByRole("listbox")).toBeInTheDocument();

    fireEvent.mouseDown(screen.getByText("Seite"));

    expect(screen.queryByRole("listbox")).toBeNull();
  });
});

describe("a caller that keeps the panel open", () => {
  it("is asked again on the next press", async () => {
    const onOpenChange = vi.fn();
    render(
      <>
        <p>Seite</p>
        <AnchoredPopover
          open
          onOpenChange={onOpenChange}
          ariaLabel="Speichert"
          renderTrigger={({ ref }) => (
            <button ref={ref} type="button">
              Speichern
            </button>
          )}
        >
          {() => <p>Wird gespeichert</p>}
        </AnchoredPopover>
      </>,
    );
    const page = screen.getByText("Seite");

    fireEvent.pointerDown(page);
    fireEvent.mouseDown(page);
    fireEvent.mouseUp(page);
    fireEvent.mouseDown(page);
    fireEvent.keyDown(document.body, { key: "Escape" });

    expect(onOpenChange.mock.calls).toEqual([[false], [false], [false]]);
  });
});
