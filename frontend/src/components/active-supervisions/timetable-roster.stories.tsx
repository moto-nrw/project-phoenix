import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";

import type {
  TimetableRoster,
  TimetableRosterRow,
} from "~/lib/timetable-operations-types";
import { TimetableRosterContent } from "./timetable-roster";

const expected: TimetableRosterRow = {
  studentId: "1",
  studentName: "Marie Muster",
  schoolClass: "2b",
  groupName: "Sonnengruppe",
  planned: true,
  isUnplanned: false,
  currentlyPresent: false,
  visitId: null,
  status: "expected",
  substatus: null,
  note: null,
  checkedInAt: null,
  checkedOutAt: null,
  visitEntryTime: null,
  pickupTime: "16:00",
  warnings: [],
  careDayStatus: "scheduled",
  parallelPresentIn: null,
};

const roster: TimetableRoster = {
  instance: {
    id: "1",
    title: "Lernzeit",
    status: "active",
    isSpontaneous: false,
    activeGroupId: "1",
    roomId: "1",
    roomName: "Lernraum",
    date: "2026-09-09",
    startTime: "13:00",
    endTime: "14:00",
    canComplete: false,
    completeAvailableAt: "2026-09-09T12:00:00Z",
  },
  rows: [
    expected,
    {
      ...expected,
      studentId: "2",
      studentName: "Ben Beispiel",
      currentlyPresent: true,
      status: "present",
      visitId: "2",
    },
    {
      ...expected,
      studentId: "3",
      studentName: "Lea Muster",
      status: "absent",
      substatus: "excused",
    },
  ],
  pickupTimesLoaded: true,
  canOperate: false,
  canEditAttendance: false,
  canReportAbsence: false,
};

const meta: Meta<typeof TimetableRosterContent> = {
  title: "components/active-supervisions/TimetableRosterRights",
  component: TimetableRosterContent,
  parameters: { layout: "padded" },
  args: {
    roster,
    addStudentResults: [],
    addStudentSearch: "",
    attendanceWebEnabled: true,
    isAddingStudent: false,
    isCompletingInstance: false,
    isConfirmingExpected: false,
    showTimetableCounts: true,
    onAddStudent: fn().mockResolvedValue(true),
    onComplete: fn().mockResolvedValue(undefined),
    onConfirmExpected: fn().mockResolvedValue(undefined),
    onRosterAction: fn().mockResolvedValue(undefined),
    onExcuseRestOfDay: fn().mockResolvedValue(undefined),
    onSearchChange: fn(),
  },
};

export default meta;
type Story = StoryObj<typeof TimetableRosterContent>;

export const OwnResponsibilities: Story = {};

export const SchoolWideAttendance: Story = {
  args: {
    roster: { ...roster, canEditAttendance: true, canReportAbsence: true },
  },
};

export const AbsenceReportsOnly: Story = {
  args: { roster: { ...roster, canReportAbsence: true } },
};

export const WebDisabled: Story = {
  args: {
    attendanceWebEnabled: false,
    roster: { ...roster, canEditAttendance: true, canReportAbsence: true },
  },
};
