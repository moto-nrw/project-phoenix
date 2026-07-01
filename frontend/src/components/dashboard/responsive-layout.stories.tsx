import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import ResponsiveLayout from "./responsive-layout";

const meta = {
  title: "components/dashboard/ResponsiveLayout",
  component: ResponsiveLayout,
} satisfies Meta<typeof ResponsiveLayout>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    children: (
      <div className="rounded-2xl border border-gray-200 bg-white p-6 shadow-sm">
        Seiteninhalt
      </div>
    ),
  },
};
