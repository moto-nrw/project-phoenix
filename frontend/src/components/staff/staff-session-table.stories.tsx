import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { StaffSessionTable } from "./staff-session-table";
import type {
  StaffAbsenceRow,
  StaffHistorySession,
  StaffSchedule,
} from "~/lib/staff-api";

// Fixed range so the story is deterministic regardless of when it's rendered:
// a Monday through Friday in the past.
const from = new Date("2026-06-08T00:00:00");
const to = new Date("2026-06-12T00:00:00");
const today = new Date("2026-06-15T00:00:00");

const schedule: StaffSchedule = {
  mode: "custom",
  model: null,
  rotationLength: 1,
  rotationAnchorDate: "2026-06-08",
  entries: [
    { weekIndex: 0, dayOfWeek: 1, targetMinutes: 480 },
    { weekIndex: 0, dayOfWeek: 2, targetMinutes: 480 },
    { weekIndex: 0, dayOfWeek: 3, targetMinutes: 480 },
    { weekIndex: 0, dayOfWeek: 4, targetMinutes: 480 },
    { weekIndex: 0, dayOfWeek: 5, targetMinutes: 360 },
  ],
  weeklyTotals: [2280],
  validFrom: "2026-06-08",
};

const sessions: StaffHistorySession[] = [
  {
    id: 1,
    date: "2026-06-08",
    status: "present",
    source: "nfc",
    net_minutes: 480,
    check_in_time: "2026-06-08T07:00:00Z",
    check_out_time: "2026-06-08T15:00:00Z",
    break_minutes: 30,
    edit_count: 0,
    audit_count: 0,
  },
  {
    id: 2,
    date: "2026-06-09",
    status: "home_office",
    source: "app",
    net_minutes: 420,
    check_in_time: "2026-06-09T07:30:00Z",
    check_out_time: "2026-06-09T14:30:00Z",
    break_minutes: 30,
    edit_count: 1,
    audit_count: 1,
  },
  {
    id: 3,
    date: "2026-06-10",
    status: "present",
    source: "nfc",
    net_minutes: 500,
    check_in_time: "2026-06-10T07:00:00Z",
    check_out_time: null,
    break_minutes: 0,
    auto_checked_out: true,
    edit_count: 0,
    audit_count: 1,
  },
];

const absences: StaffAbsenceRow[] = [
  {
    id: 1,
    staff_id: 1,
    absence_type: "sick",
    date_start: "2026-06-11",
    date_end: "2026-06-11",
    half_day: false,
    note: "",
    status: "approved",
  },
];

const meta = {
  title: "staff/StaffSessionTable",
  component: StaffSessionTable,
} satisfies Meta<typeof StaffSessionTable>;

export default meta;

type Story = StoryObj<typeof meta>;

export const AdminView: Story = {
  args: {
    staffId: "1",
    from,
    to,
    sessions,
    absences,
    schedule,
    accountStartDate: "",
    accountStartDatePending: false,
    accountStartDateError: false,
    today,
    isAdminView: true,
  },
};

export const SelfView: Story = {
  args: {
    staffId: "1",
    from,
    to,
    sessions,
    absences,
    schedule,
    accountStartDate: "",
    accountStartDatePending: false,
    accountStartDateError: false,
    today,
    isAdminView: false,
  },
};

export const Empty: Story = {
  args: {
    staffId: "1",
    from,
    to,
    sessions: [],
    absences: [],
    schedule: null,
    accountStartDate: "",
    accountStartDatePending: false,
    accountStartDateError: false,
    today,
    isAdminView: true,
  },
};
