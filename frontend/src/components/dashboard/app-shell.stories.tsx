import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { AppShell } from "./app-shell";

const meta = {
  title: "components/dashboard/AppShell",
  component: AppShell,
} satisfies Meta<typeof AppShell>;

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
