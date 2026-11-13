import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { withNotBookedSlots } from "~storybook/fixtures/not-booked-slots";
import { FormErrorAlert, LoadErrorAlert } from "./form-error-alert";

const meta = {
  title: "components/ui/FormErrorAlert",
  component: FormErrorAlert,
  parameters: { layout: "padded" },
  args: {
    message: "Die Gruppe konnte nicht gespeichert werden.",
  },
} satisfies Meta<typeof FormErrorAlert>;

export default meta;

type Story = StoryObj<typeof meta>;

const notBookedError = {
  message: "Das Modul ist nicht gebucht.",
  notBookedModule: "zeiterfassung",
  attempt: 1,
} as const;

export const FormError: Story = {};

/** Speichern wird mit `module.not_booked` abgelehnt (#4894). */
export const FormModuleNotBooked: Story = {
  args: { message: notBookedError },
  decorators: [withNotBookedSlots()],
};

const retryError = {
  message: "Die Kinder konnten nicht geladen werden.",
  retry: { label: "Wiederholen", onClick: () => {} },
  attempt: 1,
};

export const LoadError: Story = {
  render: () => <LoadErrorAlert error={retryError} />,
};

export const LoadErrorResponsive: Story = {
  render: () => <LoadErrorAlert error={retryError} actionLayout="responsive" />,
};

/** Laden wird mit `module.not_booked` abgelehnt: Erklärung, kein Wiederholen. */
export const LoadModuleNotBooked: Story = {
  render: () => <LoadErrorAlert error={notBookedError} />,
  decorators: [withNotBookedSlots()],
};
