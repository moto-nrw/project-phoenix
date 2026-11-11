import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { ExportDialog, type ExportDialogRun } from "./export-dialog";
import type { ExportFileFormat } from "./export-formats";

const noop = () => undefined;

function run(
  overrides: Partial<ExportDialogRun<ExportFileFormat>> = {},
): ExportDialogRun<ExportFileFormat> {
  return {
    busy: null,
    error: "",
    bodyRef: { current: null },
    download: noop,
    updateFormat: noop,
    ...overrides,
  };
}

const meta = {
  title: "components/ui/ExportDialog",
  component: ExportDialog<ExportFileFormat>,
  args: {
    isOpen: true,
    onClose: noop,
    title: "Klassenliste exportieren",
    description: "Alle Kinder der gewählten Klassen.",
    appliedFilters: ["Klasse 3a", "Nur anwesende Kinder"],
    formats: ["pdf", "docx", "xlsx"],
    run: run(),
  },
} satisfies Meta<typeof ExportDialog<ExportFileFormat>>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const WithPrint: Story = {
  args: { run: run({ print: noop }) },
};

export const SingleFormat: Story = {
  args: { formats: ["csv"], appliedFilters: [], description: undefined },
};

export const Downloading: Story = {
  args: { run: run({ busy: "download", print: noop }) },
};

export const Printing: Story = {
  args: { run: run({ busy: "print", print: noop }) },
};

export const Blocked: Story = {
  args: { blocked: true, run: run({ print: noop }) },
};

export const WithError: Story = {
  args: {
    run: run({ error: "Die Datei konnte nicht erstellt werden." }),
  },
};
