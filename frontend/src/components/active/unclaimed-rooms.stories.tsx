import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { UnclaimedRooms } from "./unclaimed-rooms";

const meta = {
  title: "components/active/UnclaimedRooms",
  component: UnclaimedRooms,
} satisfies Meta<typeof UnclaimedRooms>;

export default meta;

type Story = StoryObj<typeof meta>;

/**
 * The Schulhof banner is currently disabled via the `ENABLE_SCHULHOF_BANNER`
 * feature flag (a permanent tab in active-supervisions handles this now), so
 * the component always renders `null`. This story documents the current
 * disabled state.
 */
export const Disabled: Story = {
  args: {
    onClaimed: () => undefined,
    activeGroups: [],
  },
};
