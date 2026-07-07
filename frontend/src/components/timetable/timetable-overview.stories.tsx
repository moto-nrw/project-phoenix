import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { TimetableOverview } from "./timetable-overview";

const meta: Meta<typeof TimetableOverview> = {
  title: "components/timetable/TimetableOverview",
  component: TimetableOverview,
  parameters: {
    layout: "padded",
  },
};

export default meta;

type Story = StoryObj<typeof TimetableOverview>;

export const Default: Story = {
  args: {
    plannedLabel: "Geplant",
    plannedCount: 12,
    plannedSublabel: "diese Woche",
    staffGapCount: 0,
    staffGapSublabel: "keine Lücken",
    createLabel: "Neuer Termin",
    onCreate: () => {
      // no-op for story
    },
  },
};

export const WithStaffGaps: Story = {
  args: {
    plannedLabel: "Regeltermine",
    plannedCount: 8,
    plannedSublabel: "aktive Serien",
    staffGapCount: 3,
    staffGapSublabel: "offene Termine",
    createLabel: "Regeltermin anlegen",
    onCreate: () => {
      // no-op for story
    },
  },
};
