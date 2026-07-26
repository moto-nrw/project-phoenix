import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { RequestStatusBadge } from "./request-status-badge";

const meta = {
  title: "components/messaging/RequestStatusBadge",
  component: RequestStatusBadge,
} satisfies Meta<typeof RequestStatusBadge>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Open: Story = {
  args: {
    label: "Offen",
  },
};

export const InProgress: Story = {
  args: {
    label: "In Bearbeitung",
  },
};

export const Done: Story = {
  args: {
    label: "Erledigt",
  },
};
