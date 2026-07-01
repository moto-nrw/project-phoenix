import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { SessionWarning } from "./session-warning";

const meta = {
  title: "dashboard/header/SessionWarning",
  component: SessionWarning,
} satisfies Meta<typeof SessionWarning>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Hidden: Story = {
  args: {
    isExpired: false,
    variant: "desktop",
  },
};

export const Desktop: Story = {
  args: {
    isExpired: true,
    variant: "desktop",
  },
};

export const Mobile: Story = {
  args: {
    isExpired: true,
    variant: "mobile",
  },
};
