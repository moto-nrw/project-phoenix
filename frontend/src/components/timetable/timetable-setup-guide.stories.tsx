import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { TimetableSetupGuide } from "./timetable-setup-guide";

const meta: Meta<typeof TimetableSetupGuide> = {
  title: "timetable/TimetableSetupGuide",
  component: TimetableSetupGuide,
  parameters: {
    layout: "padded",
  },
};

export default meta;

type Story = StoryObj<typeof TimetableSetupGuide>;

export const NotStarted: Story = {
  args: {
    hasActivePeriod: false,
    activePeriodLabel: null,
    enrollmentStatus: "unknown",
    enrollmentLabel: null,
    hasPlan: false,
    plannedCount: 0,
    onManagePeriods: () => undefined,
    onCreateEvent: () => undefined,
    enrollmentHref: "/enrollment",
  },
};

export const PeriodDoneEnrollmentPending: Story = {
  args: {
    hasActivePeriod: true,
    activePeriodLabel: "Schuljahr 2026/27",
    enrollmentStatus: "none",
    enrollmentLabel: null,
    hasPlan: false,
    plannedCount: 0,
    onManagePeriods: () => undefined,
    onCreateEvent: () => undefined,
    enrollmentHref: "/enrollment",
  },
};

export const Complete: Story = {
  args: {
    hasActivePeriod: true,
    activePeriodLabel: "Schuljahr 2026/27",
    enrollmentStatus: "active",
    enrollmentLabel: "Anmeldephase Herbst",
    hasPlan: true,
    plannedCount: 12,
    onManagePeriods: () => undefined,
    onCreateEvent: () => undefined,
    enrollmentHref: "/enrollment",
  },
};
