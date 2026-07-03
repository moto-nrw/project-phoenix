import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { SimpleAlert } from "./SimpleAlert";

const meta = {
  title: "components/simple/SimpleAlert",
  component: SimpleAlert,
  args: {
    onClose: () => {
      /* no-op */
    },
  },
} satisfies Meta<typeof SimpleAlert>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Success: Story = {
  args: {
    type: "success",
    message: "Erfolgreich gespeichert.",
  },
};

export const ErrorAlert: Story = {
  args: {
    type: "error",
    message: "Ein Fehler ist aufgetreten.",
  },
};

export const Info: Story = {
  args: {
    type: "info",
    message: "Dies ist eine Information.",
  },
};

export const Warning: Story = {
  args: {
    type: "warning",
    message: "Bitte überprüfen Sie Ihre Eingabe.",
  },
};

export const AutoClose: Story = {
  args: {
    type: "success",
    message: "Schließt automatisch nach 3 Sekunden.",
    autoClose: true,
    duration: 3000,
  },
};

export const NoCloseButton: Story = {
  args: {
    type: "info",
    message: "Kein Schließen-Button vorhanden.",
    onClose: undefined,
  },
};
