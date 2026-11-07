import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { Loading } from "~/components/ui/loading";

const meta: Meta<typeof Loading> = {
  title: "components/ui/Loading",
  component: Loading,
};

export default meta;
type Story = StoryObj<typeof Loading>;

export const FullPage: Story = {
  args: {
    fullPage: true,
  },
};

export const Inline: Story = {
  args: {},
};

export const CustomMessage: Story = {
  args: {
    message: "Daten werden geladen...",
  },
};
