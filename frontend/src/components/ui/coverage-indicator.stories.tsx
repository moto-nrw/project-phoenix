import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { CoverageIndicator } from "./coverage-indicator";

const meta = {
  title: "components/ui/CoverageIndicator",
  component: CoverageIndicator,
  args: {
    state: "covered",
    current: 3,
    total: 3,
  },
} satisfies Meta<typeof CoverageIndicator>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Covered: Story = {};

export const Gap: Story = {
  args: {
    state: "gap",
    current: 1,
    total: 3,
  },
};

export const Acknowledged: Story = {
  args: {
    state: "acknowledged",
    current: 0,
    total: 1,
    note: "bewusst unbesetzt",
  },
};

export const FreeLabel: Story = {
  args: {
    state: "covered",
    current: undefined,
    total: undefined,
    label: "19,5/20,25 h",
  },
};

export const Small: Story = {
  args: {
    state: "gap",
    current: 1,
    total: 3,
    size: "sm",
  },
};

export const WeeklySumUnder: Story = {
  args: {
    state: "covered",
    current: undefined,
    total: undefined,
    label: "18/20,25 h",
    tone: "under",
    size: "sm",
  },
};

export const WeeklySumOver: Story = {
  args: {
    state: "covered",
    current: undefined,
    total: undefined,
    label: "22/20,25 h",
    tone: "over",
    size: "sm",
  },
};
