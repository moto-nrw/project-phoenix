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
    understaffedCount: 0,
    understaffedSublabel: "ausreichend besetzt",
    createLabel: "Neuer Termin",
    onCreate: () => {
      // no-op for story
    },
  },
};

export const WithUnderstaffedAppointments: Story = {
  args: {
    plannedLabel: "Regeltermine",
    plannedCount: 8,
    plannedSublabel: "aktive Serien",
    understaffedCount: 3,
    understaffedSublabel: "zusätzliches Personal nötig",
    createLabel: "Regeltermin anlegen",
    onCreate: () => {
      // no-op for story
    },
  },
};
