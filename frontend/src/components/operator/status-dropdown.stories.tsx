import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";

import { StatusDropdown } from "~/components/operator/status-dropdown";

const meta = {
  title: "components/operator/StatusDropdown",
  component: StatusDropdown,
  parameters: {
    layout: "padded",
  },
  args: {
    value: "open",
    onChange: fn(),
  },
} satisfies Meta<typeof StatusDropdown>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Open: Story = {};

export const Planned: Story = {
  args: {
    value: "planned",
  },
};

export const Done: Story = {
  args: {
    value: "done",
  },
};

export const Rejected: Story = {
  args: {
    value: "rejected",
  },
};

export const Medium: Story = {
  args: {
    size: "md",
  },
};

export const Disabled: Story = {
  args: {
    disabled: true,
  },
};
