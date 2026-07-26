import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { EmptyDetailState } from "./empty-detail-state";

const meta = {
  title: "components/database/EmptyDetailState",
  component: EmptyDetailState,
} satisfies Meta<typeof EmptyDetailState>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const CustomText: Story = {
  args: {
    title: "Kein Kind ausgewählt",
    description: "Wähle ein Kind aus der Liste links, um Details anzuzeigen.",
  },
};
