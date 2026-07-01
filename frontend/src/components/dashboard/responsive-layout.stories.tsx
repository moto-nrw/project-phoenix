import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import ResponsiveLayout from "./responsive-layout"; // NOSONAR - deprecated component kept for compatibility story coverage.

const meta = {
  title: "components/dashboard/ResponsiveLayout",
  // The component is explicitly deprecated but still tested for compatibility.
  component: ResponsiveLayout, // NOSONAR
} satisfies Meta<typeof ResponsiveLayout>; // NOSONAR

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
