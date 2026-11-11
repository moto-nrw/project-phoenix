import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { OccupancyBadges } from "./occupancy-badges";

const meta = {
  title: "components/ui/planning/OccupancyBadges",
  component: OccupancyBadges,
  args: { occupancy: { count: 44, limit: 45 }, nfcEnabled: true },
  decorators: [
    (StoryFn) => (
      <div className="flex flex-wrap items-center gap-2">
        <StoryFn />
      </div>
    ),
  ],
} satisfies Meta<typeof OccupancyBadges>;

export default meta;
type Story = StoryObj<typeof meta>;

export const WithinLimit: Story = {};

export const WithoutLimit: Story = {
  args: { occupancy: { count: 12, limit: null } },
};

export const Overbooked: Story = {
  args: { occupancy: { count: 66, limit: 45 } },
};

export const OverbookedCompact: Story = {
  args: { occupancy: { count: 66, limit: 45 }, compact: true },
};
