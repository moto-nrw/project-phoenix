import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { PageHeader } from "./PageHeader";

const meta = {
  title: "components/ui/page-header/PageHeader",
  component: PageHeader,
  parameters: {
    layout: "padded",
  },
} satisfies Meta<typeof PageHeader>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {
  args: {
    title: "Mitarbeitende",
  },
};

export const WithBadge: Story = {
  args: {
    title: "Kinder",
    badge: {
      count: 24,
      label: "Anwesend",
    },
  },
};

export const WithStatusIndicator: Story = {
  args: {
    title: "Raum 1",
    statusIndicator: {
      color: "green",
      tooltip: "Online",
    },
  },
};

export const WithOverflowMenu: Story = {
  args: {
    title: "Gruppe A",
    overflowMenu: [
      { label: "Exportieren", onClick: () => undefined },
      { label: "Gruppe übergeben", onClick: () => undefined },
    ],
  },
};
