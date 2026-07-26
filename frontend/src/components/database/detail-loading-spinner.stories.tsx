import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { DetailLoadingSpinner } from "./detail-loading-spinner";

const meta = {
  title: "database/DetailLoadingSpinner",
  component: DetailLoadingSpinner,
} satisfies Meta<typeof DetailLoadingSpinner>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    label: "Wird geladen …",
  },
};
