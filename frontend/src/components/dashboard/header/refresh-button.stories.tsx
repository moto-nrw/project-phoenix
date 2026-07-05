import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { RefreshButton } from "./refresh-button";

const meta = {
  title: "components/dashboard/header/RefreshButton",
  component: RefreshButton,
} satisfies Meta<typeof RefreshButton>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};
