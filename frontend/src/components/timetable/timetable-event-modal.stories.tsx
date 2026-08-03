import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { TimetableEventModal } from "./timetable-event-modal";

const meta: Meta<typeof TimetableEventModal> = {
  title: "components/timetable/TimetableEventModal",
  component: TimetableEventModal,
  parameters: {
    layout: "fullscreen",
  },
};

export default meta;

type Story = StoryObj<typeof TimetableEventModal>;

export const Default: Story = {
  args: {
    isOpen: true,
    onClose: () => {
      // no-op for story
    },
    onSaved: () => {
      // no-op for story
    },
    defaultDate: "2026-07-01",
    calendarPeriods: [],
    canCheckShiftCoverage: true,
    canManageCategories: true,
  },
};

export const QuickCreate: Story = {
  args: {
    isOpen: true,
    onClose: () => {
      // no-op for story
    },
    onSaved: () => {
      // no-op for story
    },
    defaultDate: "2026-07-01",
    calendarPeriods: [],
    canCheckShiftCoverage: true,
    canManageCategories: true,
    variant: "quick",
  },
};
