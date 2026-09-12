import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import type { Room } from "~/lib/room-helpers";
import { RoomStammdatenTab } from "./room-stammdaten-tab";

const room: Room = {
  id: "1",
  name: "Gruppenraum Sonne",
  category: "Gruppenraum",
  building: "Hauptgebäude",
  floor: 1,
  capacity: 24,
  color: "#4F46E5",
  isOccupied: true,
  activityName: "Hausaufgaben",
  groupName: "Füchse",
};

const meta: Meta<typeof RoomStammdatenTab> = {
  title: "components/rooms/RoomStammdatenTab",
  component: RoomStammdatenTab,
  args: {
    room,
    onSave: async () => {},
  },
};

export default meta;

type Story = StoryObj<typeof RoomStammdatenTab>;

export const Occupied: Story = {};

export const FreeSystemRoom: Story = {
  args: {
    room: {
      ...room,
      id: "9",
      name: "Schulhof",
      category: undefined,
      isOccupied: false,
      activityName: undefined,
      groupName: undefined,
    },
  },
};
