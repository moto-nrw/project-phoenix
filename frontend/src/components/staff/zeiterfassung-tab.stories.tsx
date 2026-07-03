import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { ZeiterfassungTab } from "./zeiterfassung-tab";

const meta = {
  title: "components/staff/ZeiterfassungTab",
  component: ZeiterfassungTab,
  args: {
    staffId: "1",
  },
} satisfies Meta<typeof ZeiterfassungTab>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};
