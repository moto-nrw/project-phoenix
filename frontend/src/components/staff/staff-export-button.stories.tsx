import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { StaffExportButton } from "./staff-export-button";

const meta = {
  title: "components/staff/StaffExportButton",
  component: StaffExportButton,
  args: {
    staffId: "1",
    yearStart: new Date("2026-08-01"),
  },
} satisfies Meta<typeof StaffExportButton>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};
