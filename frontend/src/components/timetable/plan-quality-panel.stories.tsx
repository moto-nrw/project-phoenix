import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import type { Staff } from "~/lib/staff-api";
import type {
  EnrichedInstance,
  ExceptionConflict,
  GapInstance,
} from "~/lib/timetable-types";

import { PlanQualityPanel } from "./plan-quality-panel";

const staff: Staff[] = [
  {
    id: "1",
    name: "Anna Meyer",
    firstName: "Anna",
    lastName: "Meyer",
    hasRfid: true,
    isTeacher: true,
    isSupervising: true,
    supervisions: [],
  },
  {
    id: "2",
    name: "Ben Schulz",
    firstName: "Ben",
    lastName: "Schulz",
    hasRfid: true,
    isTeacher: false,
    isSupervising: false,
    supervisions: [],
  },
];

const gaps: GapInstance[] = [
  {
    instanceId: "inst-1",
    date: "2026-07-01",
    title: "Hausaufgabenbetreuung",
    startTime: "14:00",
    endTime: "15:00",
    roomId: "room-1",
    status: "planned",
    assignedStaffCount: 0,
    absentStaffCount: 0,
  },
  {
    instanceId: "inst-2",
    date: "2026-07-01",
    title: "Kreativ-AG",
    startTime: "15:00",
    endTime: "16:00",
    roomId: "room-2",
    status: "planned",
    assignedStaffCount: 1,
    absentStaffCount: 1,
  },
];

const conflicts: ExceptionConflict[] = [
  {
    kind: "cancelled_instance_with_scheduled_arrivals",
    date: "2026-07-01",
    activityGroupId: "group-1",
    instanceId: "inst-3",
    activityTitle: "Sport-AG",
    studentId: "student-1",
    arrivalSource: "template",
  },
];

const instances: EnrichedInstance[] = [
  {
    id: "inst-2",
    date: "2026-07-01",
    startTime: "15:00",
    endTime: "16:00",
    title: "Kreativ-AG",
    status: "planned",
    isSpontaneous: false,
    isLive: false,
    activityType: "activity",
    roomId: "room-2",
    roomName: "Kreativraum",
    staff: [
      {
        staffId: "2",
        isPrimary: true,
        isAbsent: true,
        isSubstitute: false,
      },
    ],
    studentIds: [],
    students: [],
    staffCount: 1,
    absentStaffCount: 1,
    expectedStudentsCount: 0,
    presentStudentsCount: 0,
    requiredStaffCount: 0,
    assignedStaffCount: 0,
    conflictWarnings: [],
  },
];

const meta: Meta<typeof PlanQualityPanel> = {
  title: "timetable/PlanQualityPanel",
  component: PlanQualityPanel,
  args: {
    onSelectInstance: () => {},
    onEditInstance: () => {},
    onSubstitute: async () => {},
  },
};

export default meta;

type Story = StoryObj<typeof PlanQualityPanel>;

export const NoIssues: Story = {
  args: {
    instances: [],
    gaps: [],
    conflicts: [],
    staff,
    loading: false,
    error: null,
  },
};

export const WithGapsAndConflicts: Story = {
  args: {
    instances,
    gaps,
    conflicts,
    staff,
    loading: false,
    error: null,
  },
};

export const Loading: Story = {
  args: {
    instances: [],
    gaps: [],
    conflicts: [],
    staff: [],
    loading: true,
    error: null,
  },
};

export const ErrorState: Story = {
  args: {
    instances: [],
    gaps: [],
    conflicts: [],
    staff: [],
    loading: false,
    error: "Verbindung zum Server fehlgeschlagen.",
  },
};
