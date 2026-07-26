import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { Users } from "lucide-react";

import { BadgeDisplay, BadgeDisplayCompact } from "./BadgeDisplay";

const meta: Meta<typeof BadgeDisplay> = {
  title: "ui/page-header/BadgeDisplay",
  component: BadgeDisplay,
};

export default meta;

type Story = StoryObj<typeof BadgeDisplay>;

export const Default: Story = {
  args: {
    count: 42,
  },
};

export const WithLabel: Story = {
  args: {
    count: 10,
    label: "Kinder",
    showLabel: true,
  },
};

export const WithIcon: Story = {
  args: {
    count: 5,
    label: "Kinder",
    icon: <Users size={16} />,
  },
};

export const SmallSize: Story = {
  args: {
    count: 5,
    size: "sm",
  },
};

export const Compact: StoryObj<typeof BadgeDisplayCompact> = {
  render: (args) => <BadgeDisplayCompact {...args} />,
  args: {
    count: 25,
    icon: <Users size={16} />,
  },
};

export const Gallery: Story = {
  render: () => (
    <div className="flex flex-wrap items-center gap-4">
      <BadgeDisplay count={42} />
      <BadgeDisplay count={10} label="Kinder" icon={<Users size={16} />} />
      <BadgeDisplay count={5} size="sm" />
      <BadgeDisplayCompact count={25} icon={<Users size={16} />} />
    </div>
  ),
};
