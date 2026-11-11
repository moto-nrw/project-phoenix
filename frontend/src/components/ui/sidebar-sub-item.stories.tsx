import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { SidebarSubItem } from "./sidebar-sub-item";

const meta = {
  title: "components/ui/SidebarSubItem",
  component: SidebarSubItem,
  parameters: {
    layout: "padded",
  },
} satisfies Meta<typeof SidebarSubItem>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    href: "#",
    label: "Übersicht",
    isActive: false,
  },
};

export const Active: Story = {
  args: {
    href: "#",
    label: "Übersicht",
    isActive: true,
  },
};

export const WithCount: Story = {
  args: {
    href: "#",
    label: "Anfragen",
    isActive: false,
    count: 5,
  },
};

export const WithStringCount: Story = {
  args: {
    href: "#",
    label: "Benachrichtigungen",
    isActive: false,
    count: "99+",
  },
};

export const ActiveWithBadge: Story = {
  args: {
    href: "#",
    label: "Überblick",
    isActive: true,
    count: 4,
    badgeCount: 3,
    badgeTone: "staff",
    badgeNoun: "ungelesene Anmeldungen",
  },
};

export const Locked: Story = {
  args: {
    href: "#",
    label: "Speiseplan",
    isActive: false,
    count: 2,
    locked: true,
  },
};

export const LongLabelWithCount: Story = {
  decorators: [
    (Story) => (
      <div className="w-56">
        <Story />
      </div>
    ),
  ],
  args: {
    href: "#",
    label: "Eine sehr lange Gruppenbezeichnung, die gekürzt wird",
    isActive: false,
    count: "12/20",
  },
};
