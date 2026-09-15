import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import type { Room } from "~/lib/room-helpers";
import { RoomsList } from "./rooms-list";

const rooms: Room[] = [
  {
    id: "1",
    name: "Gruppenraum Sonne",
    category: "Gruppenraum",
    building: "Hauptgebäude",
    floor: 1,
    capacity: 24,
    isOccupied: true,
    activityName: "Hausaufgaben",
  },
  {
    id: "2",
    name: "Mensa",
    category: "Mensa",
    building: "Hauptgebäude",
    floor: 0,
    isOccupied: false,
  },
  {
    id: "3",
    name: "Turnhalle",
    category: "Sport",
    building: "Nebengebäude",
    floor: 0,
    isOccupied: false,
  },
];

const meta: Meta<typeof RoomsList> = {
  title: "components/rooms/RoomsList",
  component: RoomsList,
  args: {
    groupDefinitions: [
      { id: "Hauptgebäude", title: "Hauptgebäude", items: rooms.slice(0, 2) },
      { id: "Nebengebäude", title: "Nebengebäude", items: rooms.slice(2) },
    ],
    objectHref: (room) => `/rooms/${room.id}?from=%2Fdatabase%2Frooms`,
  },
};

export default meta;

type Story = StoryObj<typeof RoomsList>;

export const GroupedByBuilding: Story = {};

export const Empty: Story = {
  args: { groupDefinitions: [] },
};
