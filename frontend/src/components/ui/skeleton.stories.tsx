import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { Skeleton, SkeletonRows } from "~/components/ui/skeleton";

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
    shape: "round",
    className: "h-12 w-12",
  },
};

export const CardLayout: Story = {
  render: () => (
    <div className="flex flex-col gap-3">
      <Skeleton shape="round" className="h-12 w-12" />
      <Skeleton className="h-4 w-48" />
      <Skeleton className="h-4 w-32" />
    </div>
  ),
};

export const Rows: Story = {
  render: () => (
    <div className="w-80">
      <SkeletonRows />
    </div>
  ),
};
