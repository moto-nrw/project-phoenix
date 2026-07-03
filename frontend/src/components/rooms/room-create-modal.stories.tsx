import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { RoomCreateModal } from "./room-create-modal";

const meta = {
  title: "components/rooms/RoomCreateModal",
  component: RoomCreateModal,
  parameters: {
    layout: "fullscreen",
  },
} satisfies Meta<typeof RoomCreateModal>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Open: Story = {
  args: {
    isOpen: true,
    onClose: () => {},
    onCreate: async () => {},
  },
};
