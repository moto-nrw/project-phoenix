import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";
import { expect, screen, userEvent, within } from "storybook/test";
import { DatePicker, ISODateInput, ISODatePicker } from "./date-picker";
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

function ISOFieldDemo() {
  const [value, setValue] = useState("2026-09-09");
  return (
    <div className="max-w-xs p-10">
      <ISODatePicker
        id="start-date"
        label="Startdatum"
        required
        hint="Der erste Betreuungstag."
        error="Das Datum liegt in den Ferien."
        value={value}
        onChange={setValue}
        controlSize="md"
      />
    </div>
  );
}

/** Beschriftung, Hinweis und Fehler aus der Feld-Hülle (#6579). */
export const ISOWithLabelHintAndError: Story = {
  render: () => <ISOFieldDemo />,
};

function SizesDemo() {
  return (
    <div className="flex max-w-xs flex-col gap-3 p-10">
      <DatePicker value={null} onChange={() => undefined} ariaLabel="Klein" />
      <DatePicker
        value={null}
        onChange={() => undefined}
        ariaLabel="Mittel"
        controlSize="md"
      />
      <DatePicker
        value={null}
        onChange={() => undefined}
        ariaLabel="Groß"
        controlSize="lg"
        hint="Neben einem großen Eingabefeld."
      />
      <DatePicker
        value={null}
        onChange={() => undefined}
        ariaLabel="Kalender öffnen"
        iconOnly
      />
    </div>
  );
}

/** Die Auslöser-Größen und der reine Symbol-Auslöser. */
export const Sizes: Story = {
  render: () => <SizesDemo />,
};

function ISOInputDemo() {
  const [value, setValue] = useState("");
  return (
    <div className="max-w-xs p-10">
      <ISODateInput
        id="birthday"
        label="Geburtsdatum"
        value={value}
        onChange={setValue}
        error="Bitte geben Sie das Geburtsdatum ein."
      />
    </div>
  );
}

/** Getipptes Datum mit Kalender daneben und Fehler darunter. */
export const ISOInputWithError: Story = {
  render: () => <ISOInputDemo />,
};
