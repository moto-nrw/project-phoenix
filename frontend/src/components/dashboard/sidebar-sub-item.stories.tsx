import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { SidebarSubItem } from "./sidebar-sub-item";

const meta = {
  title: "components/dashboard/SidebarSubItem",
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
