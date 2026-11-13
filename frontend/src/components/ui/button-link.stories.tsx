import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { Plus } from "lucide-react";

import { ButtonLink } from "./button";

const meta = {
  title: "components/ui/ButtonLink",
  component: ButtonLink,
  parameters: {
    layout: "padded",
  },
  args: {
    href: "/rooms",
    children: "Zu den Räumen",
  },
} satisfies Meta<typeof ButtonLink>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Primary: Story = {};

/** Gleichrangige Zeilenaktion (frontend-ui-kit.md, Buttons). */
export const SurfaceCompact: Story = {
  args: {
    variant: "surface",
    size: "compact",
    "aria-label": "Raum 101 öffnen",
    children: "Öffnen",
  },
};

export const Outline: Story = {
  args: {
    variant: "outline",
    size: "md",
  },
};

/** Reines Symbol: trägt `data-icon-only` für die Kopfkarte. */
export const Icon: Story = {
  args: {
    variant: "ghost",
    size: "icon",
    "aria-label": "Raum anlegen",
    children: <Plus className="h-5 w-5" aria-hidden="true" />,
  },
};
