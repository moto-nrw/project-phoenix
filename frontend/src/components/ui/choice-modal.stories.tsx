import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { ChoiceModal } from "./choice-modal";

const meta = {
  title: "components/ui/ChoiceModal",
  component: ChoiceModal,
  args: {
    isOpen: true,
    onClose: () => {
      // no-op for stories
    },
    title: "Termin bearbeiten",
    onSelect: () => {
      // no-op for stories
    },
    options: [
      { value: "single", label: "Nur diesen Termin" },
      {
        value: "following",
        label: "Diesen und alle folgenden Termine",
        description: "Ändert auch alle zukünftigen Wiederholungen.",
      },
    ],
  },
} satisfies Meta<typeof ChoiceModal>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const WithDescription: Story = {
  args: {
    description: "Bitte wählen Sie, wie die Änderung angewendet werden soll.",
  },
};

export const Busy: Story = {
  args: {
    isBusy: true,
  },
};

export const Closed: Story = {
  args: {
    isOpen: false,
  },
};
