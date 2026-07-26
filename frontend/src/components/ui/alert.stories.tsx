import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { Alert } from "./alert";

const meta = {
  title: "components/ui/Alert",
  component: Alert,
  args: {
    type: "info",
    message: "Dies ist eine Beispielnachricht.",
  },
} satisfies Meta<typeof Alert>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Info: Story = {
  args: {
    type: "info",
    message: "Dies ist eine Informationsmeldung.",
  },
};

export const Success: Story = {
  args: {
    type: "success",
    message: "Die Änderungen wurden erfolgreich gespeichert.",
  },
};

export const Warning: Story = {
  args: {
    type: "warning",
    message: "Bitte überprüfen Sie Ihre Eingaben.",
  },
};

export const ErrorAlert: Story = {
  args: {
    type: "error",
    message: "Beim Speichern ist ein Fehler aufgetreten.",
  },
};

export const Empty: Story = {
  args: {
    type: "info",
    message: "",
  },
};
