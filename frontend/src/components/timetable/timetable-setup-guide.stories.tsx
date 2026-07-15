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
    careOfferingLinkStatus: "unknown",
    careOfferingLinkLabel: null,
    hasPlan: false,
    plannedCount: 0,
    onManagePeriods: () => undefined,
    onCreateEvent: () => undefined,
    careOfferingsHref: "/care-offerings",
  },
};

export const PeriodDoneEnrollmentPending: Story = {
  args: {
    hasActivePeriod: true,
    activePeriodLabel: "Schuljahr 2026/27",
    careOfferingLinkStatus: "unlinked",
    careOfferingLinkLabel: null,
    hasPlan: false,
    plannedCount: 0,
    onManagePeriods: () => undefined,
    onCreateEvent: () => undefined,
    careOfferingsHref: "/care-offerings",
  },
};

export const Complete: Story = {
  args: {
    hasActivePeriod: true,
    activePeriodLabel: "Schuljahr 2026/27",
    careOfferingLinkStatus: "linked",
    careOfferingLinkLabel: "2 von 3 Angeboten verknüpft",
    hasPlan: true,
    plannedCount: 12,
    onManagePeriods: () => undefined,
    onCreateEvent: () => undefined,
    careOfferingsHref: "/care-offerings",
  },
};
