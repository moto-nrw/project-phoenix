import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { PlannedNowSection } from "./planned-now-section";
import type {
  PlannedTimetableInstance,
  TimetableRosterRow,
} from "~/lib/timetable-operations-types";

function makeRosterRow(
  overrides: Partial<TimetableRosterRow> = {},
): TimetableRosterRow {
  return {
    studentId: "student-1",
    studentName: "Mia Schmidt",
    schoolClass: "3a",
    groupName: "Gruppe Sonne",
    planned: true,
    isUnplanned: false,
    currentlyPresent: false,
    visitId: null,
    status: "expected",
    substatus: null,
    note: null,
    checkedInAt: null,
    visitEntryTime: null,
    warnings: [],
    careDayStatus: "unknown",
    ...overrides,
  };
}

function makeInstance(
  overrides: Partial<PlannedTimetableInstance> = {},
): PlannedTimetableInstance {
  return {
    id: "instance-1",
    title: "Hausaufgabenbetreuung",
    date: "2026-07-01",
    startTime: "14:00",
    endTime: "15:00",
    roomId: "room-1",
    roomName: "Raum 12",
    status: "planned",
    isOverdue: false,
    minutesUntilStart: 30,
    expectedStudentsCount: 6,
    notScheduledStudentsCount: 0,
    presentStudentsCount: 0,
    assignedStaffIds: ["staff-1"],
    isAssigned: true,
    isPrimary: true,
    isSubstitute: false,
    isAbsent: false,
    rosterPreview: [
      makeRosterRow({ studentId: "s1", studentName: "Mia Schmidt" }),
      makeRosterRow({ studentId: "s2", studentName: "Leon Weber" }),
    ],
    ...overrides,
  };
}

const meta = {
  title: "components/active-supervisions/PlannedNowSection",
  component: PlannedNowSection,
} satisfies Meta<typeof PlannedNowSection>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Empty: Story = {
  args: {
    plannedNow: [],
    hasActiveTimetableSession: false,
    isStartingInstance: null,
    onStart: () => undefined,
  },
};

export const SingleUpcoming: Story = {
  args: {
    plannedNow: [makeInstance()],
    hasActiveTimetableSession: false,
    isStartingInstance: null,
    onStart: () => undefined,
  },
};

export const OverdueAndSoon: Story = {
  args: {
    plannedNow: [
      makeInstance({
        id: "instance-overdue",
        title: "Kunst-AG",
        isOverdue: true,
        minutesUntilStart: -5,
        expectedStudentsCount: 8,
        notScheduledStudentsCount: 0,
        presentStudentsCount: 3,
      }),
      makeInstance({
        id: "instance-soon",
        title: "Sport-AG",
        isOverdue: false,
        minutesUntilStart: 5,
        isPrimary: false,
        isSubstitute: true,
        rosterPreview: [
          makeRosterRow({
            studentId: "s3",
            studentName: "Noah Fischer",
            currentlyPresent: true,
            status: "present",
          }),
          makeRosterRow({
            studentId: "s4",
            studentName: "Emma Klein",
            status: "absent",
          }),
          makeRosterRow({
            studentId: "s5",
            studentName: "Ben Hoffmann",
            warnings: [
              {
                kind: "arrival_after_slot_start",
                message: "Kind kommt erfahrungsgemäß später",
                expectedArrival: null,
                slotStart: null,
                expectedGroupId: null,
                expectedGroupName: null,
                currentEducationGroupId: null,
              },
            ],
          }),
          makeRosterRow({ studentId: "s6", studentName: "Lea Wolf" }),
          makeRosterRow({ studentId: "s7", studentName: "Finn Bauer" }),
        ],
      }),
    ],
    hasActiveTimetableSession: false,
    isStartingInstance: null,
    onStart: () => undefined,
  },
};

export const Starting: Story = {
  args: {
    plannedNow: [makeInstance({ id: "instance-starting" })],
    hasActiveTimetableSession: false,
    isStartingInstance: "instance-starting",
    onStart: () => undefined,
  },
};
