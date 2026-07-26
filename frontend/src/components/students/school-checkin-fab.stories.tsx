import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { useState } from "react";

import { SchoolCheckinFab } from "./school-checkin-fab";

const meta = {
  title: "students/SchoolCheckinFab",
  component: SchoolCheckinFab,
  parameters: {
    layout: "centered",
  },
} satisfies Meta<typeof SchoolCheckinFab>;

export default meta;

type Story = StoryObj<typeof meta>;

export const FloatingOff: Story = {
  args: {
    isActive: false,
    onToggle: () => {
      // no-op for story
    },
    successCount: 0,
    pendingCount: 0,
    variant: "floating",
  },
};

export const FloatingActive: Story = {
  args: {
    isActive: true,
    onToggle: () => {
      // no-op for story
    },
    successCount: 3,
    pendingCount: 0,
    variant: "floating",
  },
};

export const FloatingPending: Story = {
  args: {
    isActive: true,
    onToggle: () => {
      // no-op for story
    },
    successCount: 2,
    pendingCount: 1,
    variant: "floating",
  },
};

export const InlineOff: Story = {
  args: {
    isActive: false,
    onToggle: () => {
      // no-op for story
    },
    successCount: 0,
    pendingCount: 0,
    variant: "inline",
  },
};

export const InlineActive: Story = {
  args: {
    isActive: true,
    onToggle: () => {
      // no-op for story
    },
    successCount: 5,
    pendingCount: 0,
    variant: "inline",
  },
};

export const Interactive: Story = {
  args: {
    isActive: false,
    onToggle: () => {
      // no-op for story — state is managed internally by `render` below
    },
    successCount: 0,
    pendingCount: 0,
    variant: "inline",
  },
  render: function Render() {
    const [isActive, setIsActive] = useState(false);
    return (
      <SchoolCheckinFab
        isActive={isActive}
        onToggle={() => setIsActive((prev) => !prev)}
        successCount={0}
        pendingCount={0}
        variant="inline"
      />
    );
  },
};
