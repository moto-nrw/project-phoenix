import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { UsersIcon } from "@phosphor-icons/react";

import { SidebarGroup } from "./sidebar-group";

// Kopfzeile einer Gruppe mit Sammelzähler, ausgeklappt und im 64-px-Streifen.
const meta = {
  title: "components/ui/SidebarGroup",
  component: SidebarGroup,
  parameters: {
    layout: "padded",
  },
  decorators: [
    (Story) => (
      <div className="w-64">
        <Story />
      </div>
    ),
  ],
  args: {
    label: "Eltern",
    icon: UsersIcon,
    isOpen: false,
    onToggle: () => {},
    containsActive: false,
    children: <div className="px-3 py-2 text-sm text-gray-600">Zeilen</div>,
  },
} satisfies Meta<typeof SidebarGroup>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Closed: Story = {};

export const ClosedWithCount: Story = {
  args: {
    badgeCount: 5,
    badgeTone: "parents",
  },
};

export const OpenWithCount: Story = {
  args: {
    isOpen: true,
    badgeCount: 5,
    badgeTone: "parents",
  },
};

export const RailWithCount: Story = {
  decorators: [
    (Story) => (
      <div className="w-11">
        <Story />
      </div>
    ),
  ],
  args: {
    badgeCount: 5,
    badgeTone: "parents",
    collapsed: true,
    labelsMounted: false,
    labelsVisible: false,
  },
};
