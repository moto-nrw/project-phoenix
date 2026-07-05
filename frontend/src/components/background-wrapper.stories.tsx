import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { BackgroundWrapper } from "./background-wrapper";

const meta = {
  title: "components/BackgroundWrapper",
  component: BackgroundWrapper,
} satisfies Meta<typeof BackgroundWrapper>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    children: (
      <div className="p-8">
        <h1 className="text-xl font-semibold">Page content</h1>
        <p className="text-gray-600">
          Rendered inside BackgroundWrapper without any modal open.
        </p>
      </div>
    ),
  },
};
