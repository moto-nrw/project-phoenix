import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { UsersIcon } from "@phosphor-icons/react";

import { NavSceneIcon } from "./nav-scene-icon/nav-scene-icon";
import { SIDEBAR_ICON_CLASSES } from "./sidebar-geometry";
import { SidebarRowLink } from "./sidebar-row";

// Zeile mit Zähler, ausgeklappt und im 64-px-Streifen: das Vorbild für
// Gruppe und Akkordeon-Bereich (#6566).
const meta = {
  title: "components/ui/SidebarRowLink",
  component: SidebarRowLink,
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
    href: "#",
    icon: <NavSceneIcon icon={UsersIcon} className={SIDEBAR_ICON_CLASSES} />,
    label: "Nachrichten",
    active: false,
  },
} satisfies Meta<typeof SidebarRowLink>;

export default meta;

type Story = StoryObj<typeof meta>;

export const WithCount: Story = {
  args: {
    badge: { count: 3, tone: "blue", ariaLabel: "3 ungelesene Nachrichten" },
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
    badge: { count: 3, tone: "blue", ariaLabel: "3 ungelesene Nachrichten" },
    collapsed: true,
    labelsMounted: false,
    labelsVisible: false,
  },
};
