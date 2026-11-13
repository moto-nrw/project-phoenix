import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { CheckIcon, SpinnerIcon } from "./icons";

const meta = {
  title: "components/ui/Icons",
} satisfies Meta;

export default meta;

type Story = StoryObj<typeof meta>;

export const Check: Story = {
  render: () => <CheckIcon />,
};

export const Spinner: Story = {
  render: () => <SpinnerIcon />,
};

export const Gallery: Story = {
  render: () => (
    <div className="flex items-center gap-6 text-gray-700">
      <CheckIcon />
      <SpinnerIcon />
    </div>
  ),
};
