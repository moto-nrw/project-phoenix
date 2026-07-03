import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { HelpBackButton } from "./help-back-button";

const meta = {
  title: "components/help/HelpBackButton",
  component: HelpBackButton,
} satisfies Meta<typeof HelpBackButton>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};
