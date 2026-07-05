import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { FilterButton } from "./FilterButton";

const meta = {
  title: "ui/page-header/FilterButton",
  component: FilterButton,
  args: {
    isOpen: false,
    onClick: () => {},
    hasActiveFilters: false,
  },
} satisfies Meta<typeof FilterButton>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const Open: Story = {
  args: {
    isOpen: true,
  },
};

export const ActiveNoCount: Story = {
  args: {
    hasActiveFilters: true,
  },
};

export const ActiveWithCount: Story = {
  args: {
    hasActiveFilters: true,
    activeCount: 3,
  },
};
