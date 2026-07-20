import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";

import type { EnrichedInstance } from "~/lib/timetable-types";
import { InstanceBlock } from "./instance-block";

function makeInstance(
  overrides: Partial<EnrichedInstance> = {},
): EnrichedInstance {
  return {
    id: "1",
    date: "2026-07-01",
    startTime: "14:00",
    endTime: "15:00",
    title: "Hausaufgabenbetreuung",
    status: "planned",
    isSpontaneous: false,
    isLive: false,
    activityType: "care",
    roomId: "1",
    roomName: "Raum 101",
    staff: [],
    studentIds: [],
    students: [],
    staffCount: 2,
    absentStaffCount: 0,
    expectedStudentsCount: 10,
    notScheduledStudentsCount: 0,
    presentStudentsCount: 0,
    requiredStaffCount: 1,
    assignedStaffCount: 2,
    conflictWarnings: [],
    ...overrides,
  };
}

const meta: Meta<typeof InstanceBlock> = {
  title: "components/timetable/InstanceBlock",
  component: InstanceBlock,
  parameters: {
    layout: "padded",
  },
  args: {
    top: 0,
    height: 96,
    left: "0%",
    width: "100%",
    isSelected: false,
    onClick: fn(),
  },
  decorators: [
    (Story) => (
      <div className="relative h-32 w-64">
        <Story />
      </div>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof InstanceBlock>;

export const Planned: Story = {
  args: {
    instance: makeInstance(),
  },
};

export const Active: Story = {
  args: {
    instance: makeInstance({
      status: "active",
      isLive: true,
      presentStudentsCount: 6,
    }),
  },
};

export const Cancelled: Story = {
  args: {
    instance: makeInstance({
      status: "cancelled",
    }),
  },
};

export const Spontaneous: Story = {
  args: {
    instance: makeInstance({
      isSpontaneous: true,
      activityType: "activity",
    }),
  },
};

export const WithConflict: Story = {
  args: {
    instance: makeInstance({
      conflictWarnings: [
        {
          kind: "room",
          resourceId: "1",
          message: "Raum bereits belegt",
          canOverride: true,
        },
      ],
    }),
  },
};

export const Selected: Story = {
  args: {
    instance: makeInstance(),
    isSelected: true,
  },
};

export const Compact: Story = {
  args: {
    instance: makeInstance(),
    height: 40,
  },
};

export const Tiny: Story = {
  args: {
    instance: makeInstance(),
    height: 24,
  },
};
