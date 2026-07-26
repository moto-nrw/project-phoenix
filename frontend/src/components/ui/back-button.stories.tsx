import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { BackButton } from "./back-button";

const meta = {
  title: "components/ui/BackButton",
  component: BackButton,
  parameters: {
    layout: "padded",
  },
} satisfies Meta<typeof BackButton>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    referrer: "/dashboard",
  },
};
