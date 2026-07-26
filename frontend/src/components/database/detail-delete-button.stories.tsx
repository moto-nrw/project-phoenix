import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { DetailDeleteButton } from "./detail-delete-button";

const meta: Meta<typeof DetailDeleteButton> = {
  title: "database/DetailDeleteButton",
  component: DetailDeleteButton,
  args: {
    onClick: () => undefined,
  },
};

export default meta;

type Story = StoryObj<typeof DetailDeleteButton>;

export const Default: Story = {};

export const CustomLabel: Story = {
  args: {
    label: "Konto löschen",
  },
};
