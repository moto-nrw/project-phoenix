import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { Skeleton } from "~/components/ui/skeleton";

const meta = {
  title: "components/ui/Skeleton",
  component: Skeleton,
} satisfies Meta<typeof Skeleton>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    className: "h-4 w-32",
  },
};

export const Circle: Story = {
  args: {
    className: "h-12 w-12 rounded-full",
  },
};

export const CardLayout: Story = {
  render: () => (
    <div className="flex flex-col gap-3">
      <Skeleton className="h-12 w-12 rounded-full" />
      <Skeleton className="h-4 w-48" />
      <Skeleton className="h-4 w-32" />
    </div>
  ),
};
