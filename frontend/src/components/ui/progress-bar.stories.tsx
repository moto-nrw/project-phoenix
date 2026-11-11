import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { ProgressBar } from "./progress-bar";

const meta = {
  title: "components/ui/ProgressBar",
  component: ProgressBar,
  args: { value: 3, max: 6, label: "3 von 6 Schritten erledigt" },
  decorators: [
    (Story) => (
      <div className="w-72">
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof ProgressBar>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const Tones: Story = {
  render: (args) => (
    <div className="flex flex-col gap-3">
      <ProgressBar {...args} tone="green" />
      <ProgressBar {...args} tone="orange" />
      <ProgressBar {...args} tone="dark" />
    </div>
  ),
};

export const Sizes: Story = {
  render: (args) => (
    <div className="flex flex-col gap-3">
      <ProgressBar {...args} size="xs" />
      <ProgressBar {...args} size="sm" />
      <ProgressBar {...args} size="md" />
    </div>
  ),
};

export const DataColor: Story = {
  args: { color: "#5080D8" },
};

export const Full: Story = {
  args: { value: 6, tone: "orange" },
};
