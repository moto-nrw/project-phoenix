import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { ModalLoadingState } from "./modal-loading-state";

const meta = {
  title: "components/ui/ModalLoadingState",
  component: ModalLoadingState,
} satisfies Meta<typeof ModalLoadingState>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {},
};

export const CustomMessage: Story = {
  args: {
    accentColor: "blue",
    message: "Bitte warten...",
  },
};

export const Orange: Story = {
  args: {
    accentColor: "orange",
  },
};

export const Indigo: Story = {
  args: {
    accentColor: "indigo",
  },
};

export const Green: Story = {
  args: {
    accentColor: "green",
  },
};
