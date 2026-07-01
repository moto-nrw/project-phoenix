import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { SpontaneousActivityStart } from "./spontaneous-activity-start";

const meta: Meta<typeof SpontaneousActivityStart> = {
  title: "components/active-supervisions/SpontaneousActivityStart",
  component: SpontaneousActivityStart,
  parameters: {
    layout: "fullscreen",
  },
};

export default meta;
type Story = StoryObj<typeof SpontaneousActivityStart>;

export const Default: Story = {
  args: {
    onStart: () => {
      // no-op for story
    },
  },
};

export const Disabled: Story = {
  args: {
    disabled: true,
    onStart: () => {
      // no-op for story
    },
  },
};

export const Starting: Story = {
  args: {
    isStarting: true,
    onStart: () => {
      // no-op for story
    },
  },
};

export const Opened: Story = {
  args: {
    onStart: () => {
      // no-op for story
    },
  },
  play: async ({ canvasElement }) => {
    const trigger = canvasElement.querySelector<HTMLButtonElement>("button");
    trigger?.click();
  },
};
