import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";
import { expect, screen, userEvent, within } from "storybook/test";
import { DatePicker } from "./date-picker";
import { Modal } from "./modal";

const meta: Meta<typeof DatePicker> = {
  title: "components/ui/DatePicker",
  component: DatePicker,
};

export default meta;
type Story = StoryObj<typeof DatePicker>;

function SingleDemo() {
  const [value, setValue] = useState<Date | null>(null);
  return (
    <div className="p-10">
      <DatePicker value={value} onChange={setValue} />
    </div>
  );
}

function SingleWithValueDemo() {
  const [value, setValue] = useState<Date | null>(new Date());
  return (
    <div className="p-10">
      <DatePicker value={value} onChange={setValue} />
    </div>
  );
}

function MultipleDemo() {
  const [values, setValues] = useState<Date[]>([]);
  return (
    <div className="p-10">
      <DatePicker mode="multiple" values={values} onChangeDates={setValues} />
    </div>
  );
}

function MonthYearDemo() {
  const [value, setValue] = useState<Date | null>(new Date());
  return (
    <div className="p-10">
      <DatePicker
        value={value}
        onChange={setValue}
        calendarLayout="popover"
        monthYearNavigation
      />
    </div>
  );
}

function InDialogDemo() {
  const [value, setValue] = useState<Date | null>(new Date());
  return (
    <Modal isOpen onClose={() => undefined} title="Termin verschieben">
      <DatePicker value={value} onChange={setValue} calendarLayout="popover" />
    </Modal>
  );
}

function InlineDemo() {
  const [value, setValue] = useState<Date | null>(null);
  return (
    <div className="p-10">
      <DatePicker value={value} onChange={setValue} calendarLayout="inline" />
    </div>
  );
}

export const Default: Story = {
  render: () => <SingleDemo />,
};

export const WithSelectedValue: Story = {
  render: () => <SingleWithValueDemo />,
};

export const MultipleMode: Story = {
  render: () => <MultipleDemo />,
};

export const InlineLayout: Story = {
  render: () => <InlineDemo />,
};

/** Öffnet den Kalender und wartet auf das Monatsraster. */
async function openCalendar(canvasElement: HTMLElement, name: RegExp) {
  await userEvent.click(within(canvasElement).getByRole("button", { name }));
  await expect(await screen.findByRole("grid")).toBeVisible();
}

/** Offener Kalender mit gewähltem Tag (#6576, Baseline des Pixel-Vergleichs). */
export const Opened: Story = {
  render: () => <SingleWithValueDemo />,
  play: async ({ canvasElement }) => {
    await openCalendar(canvasElement, /\d{2}\.\d{2}\.\d{4}/);
  },
};

/** Offener Kalender im Mehrfach-Modus. */
export const MultipleModeOpened: Story = {
  render: () => <MultipleDemo />,
  play: async ({ canvasElement }) => {
    await openCalendar(canvasElement, /Datum auswählen/);
  },
};

/** Monats- und Jahresauswahl im Kalenderkopf, Monatsliste offen. */
export const MonthMenuOpened: Story = {
  render: () => <MonthYearDemo />,
  play: async ({ canvasElement }) => {
    await openCalendar(canvasElement, /\d{2}\.\d{2}\.\d{4}/);
    await userEvent.click(screen.getByRole("combobox", { name: "Monat" }));
    await expect(await screen.findByRole("listbox")).toBeVisible();
  },
};

/** Kalender aus einem Dialog: bleibt im Fokusbereich des Dialogs. */
export const InDialogOpened: Story = {
  render: () => <InDialogDemo />,
  play: async () => {
    const dialog = await screen.findByRole("dialog");
    await userEvent.click(
      within(dialog).getByRole("button", { name: /\d{2}\.\d{2}\.\d{4}/ }),
    );
    await expect(await screen.findByRole("grid")).toBeVisible();
  },
};
