import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { PARENT_VISIBLE_DEFAULT_HINT } from "~/lib/parent-visible-fields";

import { ParentVisibilityLegend, VisibilityBadge } from "./visibility-badge";

const hint = PARENT_VISIBLE_DEFAULT_HINT;

const meta = {
  title: "components/ui/VisibilityBadge",
} satisfies Meta;

export default meta;
type Story = StoryObj;

export const Audiences: Story = {
  render: () => (
    <div className="flex items-center gap-3">
      <VisibilityBadge audience="parents" hint={hint} />
      <VisibilityBadge audience="ogs" hint={hint} />
    </div>
  ),
};

export const Compact: Story = {
  render: () => (
    <div className="flex items-center gap-3">
      <VisibilityBadge audience="parents" compact hint={hint} />
      <VisibilityBadge audience="ogs" compact hint={hint} />
    </div>
  ),
};

export const Legend: Story = {
  render: () => <ParentVisibilityLegend hint={hint} className="max-w-sm" />,
};
