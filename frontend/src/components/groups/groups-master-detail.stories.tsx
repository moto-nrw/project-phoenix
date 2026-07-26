import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import type { Group } from "@/lib/group-helpers";
import { GroupsMasterDetail } from "./groups-master-detail";

const groups: Group[] = [
  {
    id: "1",
    name: "Gruppe Sonne",
    room_name: "Raum 101",
    student_count: 18,
    supervisors: [{ id: "s1", name: "Anna Muster" }],
  },
  {
    id: "2",
    name: "Gruppe Mond",
    room_name: "",
    student_count: 0,
    supervisors: [],
  },
];

const meta: Meta<typeof GroupsMasterDetail> = {
  title: "components/groups/GroupsMasterDetail",
  component: GroupsMasterDetail,
  parameters: {
    layout: "fullscreen",
  },
  args: {
    groups,
    selectedId: null,
    selectedGroup: null,
    onSelect: () => {},
    onSaveGroup: async () => {},
    onDeleteClick: () => {},
  },
};

export default meta;
type Story = StoryObj<typeof GroupsMasterDetail>;

export const NoSelection: Story = {};

export const GroupSelected: Story = {
  args: {
    selectedId: groups[0]?.id ?? null,
    selectedGroup: groups[0] ?? null,
  },
};

export const GroupWithoutRoomOrSupervisor: Story = {
  args: {
    selectedId: groups[1]?.id ?? null,
    selectedGroup: groups[1] ?? null,
  },
};

export const Empty: Story = {
  args: {
    groups: [],
  },
};
