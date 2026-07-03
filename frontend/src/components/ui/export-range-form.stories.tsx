import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { ExportRangeForm } from "./export-range-form";

const anchor = new Date(2026, 4, 1);

const meta = {
  title: "components/ui/ExportRangeForm",
  component: ExportRangeForm,
  args: {
    exportUrl: "/api/time-tracking/export",
    anchor,
  },
} satisfies Meta<typeof ExportRangeForm>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const WithInitialRange: Story = {
  args: {
    initialRange: {
      from: new Date(2026, 5, 1),
      to: new Date(2026, 5, 15),
    },
  },
};
