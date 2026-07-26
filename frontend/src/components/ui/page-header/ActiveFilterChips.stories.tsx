import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { ActiveFilterChips } from "./ActiveFilterChips";

const meta = {
  title: "components/ui/page-header/ActiveFilterChips",
  component: ActiveFilterChips,
  parameters: {
    layout: "padded",
  },
} satisfies Meta<typeof ActiveFilterChips>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Empty: Story = {
  args: {
    filters: [],
  },
};

export const SingleFilter: Story = {
  args: {
    filters: [
      {
        id: "status",
        label: "Status: Aktiv",
        onRemove: () => {
          // no-op for story
        },
      },
    ],
  },
};

export const MultipleFiltersWithClearAll: Story = {
  args: {
    filters: [
      {
        id: "status",
        label: "Status: Aktiv",
        onRemove: () => {
          // no-op for story
        },
      },
      {
        id: "group",
        label: "Gruppe: Sonne",
        onRemove: () => {
          // no-op for story
        },
      },
      {
        id: "room",
        label: "Raum: Bibliothek",
        onRemove: () => {
          // no-op for story
        },
      },
    ],
    onClearAll: () => {
      // no-op for story
    },
  },
};
