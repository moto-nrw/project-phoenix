import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { InstanceDetailModal } from "./instance-detail-modal";
import type { EnrichedInstance } from "~/lib/timetable-types";

function makeInstance(
  overrides: Partial<EnrichedInstance> = {},
): EnrichedInstance {
  return {
    id: "1",
    date: "2026-07-01",
    startTime: "14:00",
    endTime: "15:30",
    title: "Fußball AG",
    status: "planned",
    isSpontaneous: false,
    isLive: false,
    activityType: "activity",
    roomId: "10",
    roomName: "Sporthalle",
    staff: [
      { staffId: "1", isPrimary: true, isAbsent: false, isSubstitute: false },
    ],
    studentIds: ["1", "2"],
    students: [
      { studentId: "1", status: "expected" },
      { studentId: "2", status: "present" },
    ],
    staffCount: 1,
    absentStaffCount: 0,
    expectedStudentsCount: 1,
    notScheduledStudentsCount: 0,
    presentStudentsCount: 1,
    requiredStaffCount: 1,
    assignedStaffCount: 1,
    conflictWarnings: [],
    ...overrides,
  };
}

const staffNames = new Map<string, string>([["1", "Frau Müller"]]);
const studentNames = new Map<string, string>([
  ["1", "Anna Schmidt"],
  ["2", "Ben Weber"],
]);

const meta = {
  title: "components/timetable/InstanceDetailModal",
  component: InstanceDetailModal,
  args: {
    instance: makeInstance(),
    onClose: () => undefined,
    onLifecycleAction: async () => undefined,
    staffNames,
    studentNames,
    editDeferred: true,
  },
} satisfies Meta<typeof InstanceDetailModal>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Planned: Story = {};

export const Active: Story = {
  args: {
    instance: makeInstance({
      status: "active",
      isLive: true,
    }),
  },
};

export const Completed: Story = {
  args: {
    instance: makeInstance({
      status: "completed",
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

export const WithConflictsAndAbsences: Story = {
  args: {
    instance: makeInstance({
      staffCount: 2,
      absentStaffCount: 1,
      staff: [
        {
          staffId: "1",
          isPrimary: true,
          isAbsent: false,
          isSubstitute: false,
        },
        { staffId: "2", isPrimary: false, isAbsent: true, isSubstitute: false },
      ],
      conflictWarnings: [
        {
          kind: "staff",
          resourceId: "10",
          message:
            "Personal ist von 14:30–15:00 auch bei „Lernzeit“ eingeplant (anderer Raum).",
          canOverride: true,
        },
      ],
    }),
    staffNames: new Map([
      ["1", "Frau Müller"],
      ["2", "Herr Fischer"],
    ]),
  },
};

export const Closed: Story = {
  args: {
    instance: null,
  },
};
