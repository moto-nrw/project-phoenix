import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { DataTableStatusBadge } from "./data-table";

const meta = {
  title: "components/ui/DataTableStatusBadge",
  component: DataTableStatusBadge,
} satisfies Meta<typeof DataTableStatusBadge>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Active: Story = {
  args: { active: true },
};

export const Inactive: Story = {
  args: { active: false },
};

export const CustomLabels: Story = {
  args: {
    active: true,
    activeLabel: "Freigeschaltet",
    inactiveLabel: "Gesperrt",
  },
};
