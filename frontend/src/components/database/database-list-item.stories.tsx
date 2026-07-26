import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { DatabaseListItem } from "./database-list-item";

const meta: Meta<typeof DatabaseListItem> = {
  title: "components/database/DatabaseListItem",
  component: DatabaseListItem,
  args: {
    onSelect: () => {
      // no-op for story
    },
  },
};

export default meta;

type Story = StoryObj<typeof DatabaseListItem>;

export const Default: Story = {
  args: {
    title: "Max Mustermann",
    subtitle: "Klasse 3a",
    isSelected: false,
  },
};

export const Selected: Story = {
  args: {
    title: "Max Mustermann",
    subtitle: "Klasse 3a",
    isSelected: true,
  },
};

export const WithTrailingAccessory: Story = {
  args: {
    title: "Erika Musterfrau",
    subtitle: "Klasse 4b",
    isSelected: false,
    trailingAccessory: <span className="text-xs text-gray-400">Aktiv</span>,
  },
};
