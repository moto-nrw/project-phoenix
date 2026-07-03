import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { ToastProvider } from "~/contexts/ToastContext";
import { LeaveRequestsCard } from "./leave-requests-card";

const meta: Meta<typeof LeaveRequestsCard> = {
  title: "components/time-tracking/LeaveRequestsCard",
  component: LeaveRequestsCard,
  decorators: [
    (Story) => (
      <ToastProvider>
        <Story />
      </ToastProvider>
    ),
  ],
};

export default meta;

type Story = StoryObj<typeof LeaveRequestsCard>;

export const Default: Story = {};
