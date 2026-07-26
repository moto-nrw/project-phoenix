import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";

import {
  StudentCheckoutSection,
  StudentCheckinSection,
  StudentSickReportSection,
  StudentExcusedReportSection,
  StudentStatusActionsMenu,
} from "./student-checkout-section";

const meta = {
  title: "students/StudentCheckoutSection",
} satisfies Meta;

export default meta;

export const Checkout: StoryObj<typeof StudentCheckoutSection> = {
  render: () => <StudentCheckoutSection onCheckoutClick={fn()} />,
};

export const Checkin: StoryObj<typeof StudentCheckinSection> = {
  render: () => <StudentCheckinSection onCheckinClick={fn()} />,
};

export const SickReportHealthy: StoryObj<typeof StudentSickReportSection> = {
  render: () => (
    <StudentSickReportSection
      isSick={false}
      onToggle={fn()}
      isLoading={false}
    />
  ),
};

export const SickReportSick: StoryObj<typeof StudentSickReportSection> = {
  render: () => (
    <StudentSickReportSection
      isSick={true}
      sickSince="2026-06-01"
      onToggle={fn()}
      isLoading={false}
    />
  ),
};

export const SickReportLoading: StoryObj<typeof StudentSickReportSection> = {
  render: () => (
    <StudentSickReportSection isSick={false} onToggle={fn()} isLoading={true} />
  ),
};

export const ExcusedReportNotExcused: StoryObj<
  typeof StudentExcusedReportSection
> = {
  render: () => (
    <StudentExcusedReportSection
      isExcused={false}
      onToggle={fn()}
      isLoading={false}
    />
  ),
};

export const ExcusedReportExcused: StoryObj<
  typeof StudentExcusedReportSection
> = {
  render: () => (
    <StudentExcusedReportSection
      isExcused={true}
      excusedSince="2026-06-01"
      onToggle={fn()}
      isLoading={false}
    />
  ),
};

export const StatusActionsMenu: StoryObj<typeof StudentStatusActionsMenu> = {
  render: () => (
    <StudentStatusActionsMenu
      isClassTrip={false}
      onPlanClassTrip={fn()}
      isLoading={false}
    />
  ),
};

export const StatusActionsMenuClassTrip: StoryObj<
  typeof StudentStatusActionsMenu
> = {
  render: () => (
    <StudentStatusActionsMenu
      isClassTrip={true}
      classTripSince="2026-05-15"
      onPlanClassTrip={fn()}
      isLoading={false}
    />
  ),
};

export const Gallery: StoryObj = {
  render: () => (
    <div className="flex gap-4">
      <StudentCheckoutSection onCheckoutClick={fn()} />
      <StudentCheckinSection onCheckinClick={fn()} />
      <StudentSickReportSection
        isSick={false}
        onToggle={fn()}
        isLoading={false}
      />
      <StudentExcusedReportSection
        isExcused={false}
        onToggle={fn()}
        isLoading={false}
      />
      <StudentStatusActionsMenu
        isClassTrip={false}
        onPlanClassTrip={fn()}
        isLoading={false}
      />
    </div>
  ),
};
