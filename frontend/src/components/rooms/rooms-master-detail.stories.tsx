import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { GroupDefinition } from "~/components/database/grouped-list";
import type { Room } from "@/lib/room-helpers";
import { RoomsMasterDetail } from "./rooms-master-detail";

const rooms: Room[] = [
  {
    id: "1",
    name: "Raum 101",
    building: "Hauptgebäude",
    floor: 1,
    capacity: 20,
    category: "Gruppenraum",
    isOccupied: false,
  },
  {
    id: "2",
    name: "Turnhalle",
    building: "Sporthaus",
    floor: 0,
    capacity: 40,
    category: "Sport",
    isOccupied: true,
    activityName: "Fußball AG",
    groupName: "Gruppe Blau",
  },
];

const groupDefinitions: GroupDefinition<Room>[] = [
  {
    id: "all",
    title: "Alle Räume",
    items: rooms,
  },
];

const noop = async () => {};

const meta: Meta<typeof RoomsMasterDetail> = {
  title: "components/rooms/RoomsMasterDetail",
  component: RoomsMasterDetail,
  parameters: {
    layout: "fullscreen",
  },
};

export default meta;

type Story = StoryObj<typeof RoomsMasterDetail>;

export const NoSelection: Story = {
  args: {
    groupDefinitions,
    selectedId: null,
    selectedRoom: null,
    onSelect: () => {},
    onSaveRoom: noop,
    onDeleteClick: () => {},
  },
};

export const SelectedRoom: Story = {
  args: {
    groupDefinitions,
    selectedId: "1",
    selectedRoom: rooms[0] ?? null,
    onSelect: () => {},
    onSaveRoom: noop,
    onDeleteClick: () => {},
  },
};

export const SelectedOccupiedRoom: Story = {
  args: {
    groupDefinitions,
    selectedId: "2",
    selectedRoom: rooms[1] ?? null,
    onSelect: () => {},
    onSaveRoom: noop,
    onDeleteClick: () => {},
  },
};

export const EmptyList: Story = {
  args: {
    groupDefinitions: [],
    selectedId: null,
    selectedRoom: null,
    onSelect: () => {},
    onSaveRoom: noop,
    onDeleteClick: () => {},
  },
};
