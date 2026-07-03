import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { GroupedList, type GroupDefinition } from "./grouped-list";

interface DemoItem {
  id: string;
  label: string;
}

const demoGroups: GroupDefinition<DemoItem>[] = [
  {
    id: "group-a",
    title: "Gruppe A",
    items: [
      { id: "1", label: "Erster Eintrag" },
      { id: "2", label: "Zweiter Eintrag" },
    ],
  },
  {
    id: "group-b",
    title: "Gruppe B",
    variant: "warning",
    countSuffix: "Kinder",
    items: [{ id: "3", label: "Dritter Eintrag" }],
  },
];

const meta = {
  title: "database/GroupedList",
  component: GroupedList<DemoItem>,
} satisfies Meta<typeof GroupedList<DemoItem>>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    groups: demoGroups,
    renderItem: (item: DemoItem) => <span>{item.label}</span>,
    keyFor: (item: DemoItem) => item.id,
  },
};

export const Empty: Story = {
  args: {
    groups: [],
    renderItem: (item: DemoItem) => <span>{item.label}</span>,
    keyFor: (item: DemoItem) => item.id,
  },
};
