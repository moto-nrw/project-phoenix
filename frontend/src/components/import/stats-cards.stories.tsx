import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { StatsCards } from "./stats-cards";

const meta = {
  title: "components/import/StatsCards",
  component: StatsCards,
} satisfies Meta<typeof StatsCards>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    total: 120,
    newCount: 45,
    existing: 70,
    errors: 5,
  },
};

export const Empty: Story = {
  args: {
    total: 0,
    newCount: 0,
    existing: 0,
    errors: 0,
  },
};
