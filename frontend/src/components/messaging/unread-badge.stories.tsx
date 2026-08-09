import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { UnreadBadge } from "./unread-badge";

const meta = {
  title: "messaging/UnreadBadge",
  component: UnreadBadge,
} satisfies Meta<typeof UnreadBadge>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    count: 3,
    tone: "parents",
  },
};

export const DoubleDigit: Story = {
  args: {
    count: 42,
    tone: "parents",
  },
};

export const Capped: Story = {
  args: {
    count: 150,
    tone: "parents",
  },
};

export const Zero: Story = {
  args: {
    count: 0,
    tone: "parents",
  },
};
