import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { TriangleAlert } from "lucide-react";

import { CoverageIndicator } from "./coverage-indicator";
import { PlanBlock } from "./plan-block";

const meta = {
  title: "components/ui/PlanBlock",
  component: PlanBlock,
  args: {
    timeRange: "12:00–14:00",
    label: "Mensa",
    color: "#3F6F12",
  },
} satisfies Meta<typeof PlanBlock>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const WithCoverage: Story = {
  args: {
    footer: <CoverageIndicator state="gap" current={1} total={3} />,
  },
};

export const Compact: Story = {
  args: {
    size: "compact",
  },
};

export const Hatched: Story = {
  args: {
    status: "hatched",
    statusIcon: (
      <TriangleAlert
        className="h-3.5 w-3.5"
        style={{ color: "#F78C10" }}
        aria-hidden="true"
      />
    ),
  },
};

export const Cancelled: Story = {
  args: {
    status: "cancelled",
  },
};

export const Selected: Story = {
  args: {
    selected: true,
  },
};

export const WithoutShiftType: Story = {
  args: {
    color: undefined,
  },
};

export const NonInteractive: Story = {
  args: {
    interactive: false,
  },
};
