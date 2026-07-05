import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";

import { SchoolCheckinModeMobile } from "./school-checkin-mode-mobile";

const meta: Meta<typeof SchoolCheckinModeMobile> = {
  title: "students/SchoolCheckinModeMobile",
  component: SchoolCheckinModeMobile,
  parameters: {
    layout: "padded",
  },
  args: {
    onToggle: fn(),
  },
};

export default meta;
type Story = StoryObj<typeof SchoolCheckinModeMobile>;

export const Inactive: Story = {
  args: {
    isActive: false,
    successCount: 0,
    pendingCount: 0,
  },
};

export const ActiveIdle: Story = {
  args: {
    isActive: true,
    successCount: 0,
    pendingCount: 0,
  },
};

export const ActiveWithProgress: Story = {
  args: {
    isActive: true,
    successCount: 5,
    pendingCount: 2,
  },
};
