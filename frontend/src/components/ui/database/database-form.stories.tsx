import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { DatabaseForm } from "./database-form";
import type { FormSection } from "./database-form";

const sections: FormSection[] = [
  {
    title: "Grunddaten",
    subtitle: "Allgemeine Informationen",
    columns: 2,
    fields: [
      {
        name: "first_name",
        label: "Vorname",
        type: "text",
        required: true,
        colSpan: 1,
      },
      {
        name: "last_name",
        label: "Nachname",
        type: "text",
        required: true,
        colSpan: 1,
      },
      {
        name: "email",
        label: "E-Mail",
        type: "email",
        placeholder: "name@beispiel.de",
        colSpan: 2,
      },
      {
        name: "role",
        label: "Rolle",
        type: "select",
        options: [
          { value: "teacher", label: "Lehrkraft" },
          { value: "assistant", label: "Assistenz" },
        ],
        colSpan: 1,
      },
      {
        name: "notes",
        label: "Notizen",
        type: "textarea",
        helperText: "Optionale interne Anmerkung",
        colSpan: 2,
      },
      {
        name: "active",
        label: "Aktiv",
        type: "checkbox",
        colSpan: 1,
      },
    ],
  },
];

const meta: Meta<typeof DatabaseForm> = {
  title: "ui/database/DatabaseForm",
  component: DatabaseForm,
  args: {
    sections,
    onSubmit: async () => undefined,
    onCancel: () => undefined,
    submitLabel: "Speichern",
  },
};

export default meta;

type Story = StoryObj<typeof DatabaseForm>;

export const Default: Story = {};

export const Loading: Story = {
  args: {
    isLoading: true,
  },
};

export const WithError: Story = {
  args: {
    error: "Es ist ein Fehler aufgetreten.",
  },
};

export const StickyActions: Story = {
  args: {
    stickyActions: true,
  },
};
