import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { RoomDetailPanel, TRANSIT_ROOM_ID } from "./room-detail-panel";

const meta = {
  title: "components/rooms/RoomDetailPanel",
  component: RoomDetailPanel,
  args: {
    roomId: null,
    onClose: () => undefined,
  },
} satisfies Meta<typeof RoomDetailPanel>;

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
