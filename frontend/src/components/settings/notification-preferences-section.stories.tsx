import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { NotificationPreferencesSection } from "./notification-preferences-section";

const meta = {
  title: "components/settings/NotificationPreferencesSection",
  component: NotificationPreferencesSection,
  parameters: {
    docs: {
      description: {
        component:
          "Per-account notification consent. Sits above the per-device push card: this one answers what to hear about, the other on which device.",
      },
    },
  },
} satisfies Meta<typeof NotificationPreferencesSection>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Staff: Story = {
  args: { portal: "tenant" },
};

export const Parent: Story = {
  args: { portal: "parent" },
};
