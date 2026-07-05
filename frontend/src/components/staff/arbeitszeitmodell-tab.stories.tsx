import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { ArbeitszeitmodellTab } from "./arbeitszeitmodell-tab";

// ArbeitszeitmodellTab fetches its schedule via useSWRAuth internally (no
// data props to inject). Storybook has no backend, so the SWR request
// resolves to an error and the component stays on its own Loading state —
// this still exercises mount/import wiring without crashing.
const meta = {
  title: "components/staff/ArbeitszeitmodellTab",
  component: ArbeitszeitmodellTab,
  args: {
    staffId: "1",
    canEdit: true,
  },
} satisfies Meta<typeof ArbeitszeitmodellTab>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const ReadOnly: Story = {
  args: {
    canEdit: false,
  },
};
