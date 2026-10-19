import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { BackLink } from "./back-link";

const meta = {
  title: "components/ui/BackLink",
  component: BackLink,
  parameters: {
    layout: "padded",
  },
} satisfies Meta<typeof BackLink>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    href: "/rooms",
    label: "Zurück zu den Räumen",
  },
};
