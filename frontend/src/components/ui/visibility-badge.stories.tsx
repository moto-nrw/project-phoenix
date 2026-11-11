import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { VisibilityBadge } from "./visibility-badge";

const meta = {
  title: "components/ui/VisibilityBadge",
} satisfies Meta;

export default meta;
type Story = StoryObj;

export const Audiences: Story = {
  render: () => (
    <div className="flex items-center gap-3">
      <VisibilityBadge audience="parents" />
      <VisibilityBadge audience="ogs" />
    </div>
  ),
};

export const Compact: Story = {
  render: () => (
    <div className="flex items-center gap-3">
      <VisibilityBadge audience="parents" compact />
      <VisibilityBadge audience="ogs" compact />
    </div>
  ),
};
