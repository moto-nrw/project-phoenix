import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { Copy, Pencil, Trash2 } from "lucide-react";

import { OverflowMenu, type OverflowMenuItem } from "./OverflowMenu";

const meta = {
  title: "ui/page-header/OverflowMenu",
  component: OverflowMenu,
  parameters: {
    layout: "centered",
  },
} satisfies Meta<typeof OverflowMenu>;

export default meta;

type Story = StoryObj<typeof meta>;

const basicItems: readonly OverflowMenuItem[] = [
  {
    label: "Bearbeiten",
    icon: <Pencil className="size-4" />,
    onClick: () => {
      // no-op for story
    },
  },
  {
    label: "Duplizieren",
    icon: <Copy className="size-4" />,
    onClick: () => {
      // no-op for story
    },
  },
];

export const Default: Story = {
  args: {
    items: basicItems,
  },
};

export const WithBadgeAndDestructive: Story = {
  args: {
    items: [
      {
        label: "Anfragen",
        badge: 3,
        onClick: () => {
          // no-op for story
        },
      },
      {
        label: "Deaktiviert",
        disabled: true,
        onClick: () => {
          // no-op for story
        },
      },
      {
        label: "Löschen",
        icon: <Trash2 className="size-4" />,
        destructive: true,
        onClick: () => {
          // no-op for story
        },
      },
    ],
  },
};

export const Empty: Story = {
  args: {
    items: [],
  },
};
