import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { RoomDetailModal, TRANSIT_ROOM_ID } from "./room-detail-modal";

const meta = {
  title: "components/rooms/RoomDetailModal",
  component: RoomDetailModal,
  args: {
    roomId: null,
    onClose: () => undefined,
  },
} satisfies Meta<typeof RoomDetailModal>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Closed: Story = {};

export const OpenWithRoom: Story = {
  args: {
    roomId: "1",
  },
};

export const OpenTransit: Story = {
  args: {
    roomId: TRANSIT_ROOM_ID,
  },
};
