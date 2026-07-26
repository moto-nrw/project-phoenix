import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { UebersichtTab } from "./uebersicht-tab";

const meta = {
  title: "components/staff/UebersichtTab",
  component: UebersichtTab,
  args: {
    staffId: "1",
  },
} satisfies Meta<typeof UebersichtTab>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};
