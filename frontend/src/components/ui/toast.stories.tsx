import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { Toast } from "./toast";

const meta = {
  title: "components/ui/Toast",
  component: Toast,
  args: {
    type: "error",
    message: "Die Gruppe konnte nicht gespeichert werden.",
    accessibleLabel: "Fehler: Die Gruppe konnte nicht gespeichert werden.",
    closeLabel: "Schließen",
    onClose: () => undefined,
    copySucceededLabel: "Vorgangskennung kopiert.",
    copyFailedLabel: "Kopieren nicht möglich.",
    visible: true,
    reducedMotion: true,
    touchFriendly: false,
  },
  decorators: [
    (Story) => (
      <div className="max-w-sm">
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof Toast>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Error: Story = {};

export const Success: Story = {
  args: {
    type: "success",
    message: "Gespeichert",
    accessibleLabel: "Erfolgreich: Gespeichert",
  },
};
