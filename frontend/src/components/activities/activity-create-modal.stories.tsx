import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { ActivityCreateModal } from "./activity-create-modal";

const meta = {
  title: "components/activities/ActivityCreateModal",
  component: ActivityCreateModal,
  parameters: {
    layout: "fullscreen",
  },
} satisfies Meta<typeof ActivityCreateModal>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Open: Story = {
  args: {
    isOpen: true,
    onClose: () => {},
    onCreate: async () => {},
  },
};

export const Closed: Story = {
  args: {
    isOpen: false,
    onClose: () => {},
    onCreate: async () => {},
  },
};
