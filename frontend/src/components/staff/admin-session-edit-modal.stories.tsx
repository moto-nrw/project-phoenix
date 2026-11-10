import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import type { StaffHistorySession } from "~/lib/staff-api";

import { AdminSessionEditModal } from "./admin-session-edit-modal";

const sampleSession: StaffHistorySession = {
  id: "42",
  date: "2026-06-10",
  status: "present",
  net_minutes: 450,
  check_in_time: "2026-06-10T08:00:00.000Z",
  check_out_time: "2026-06-10T16:00:00.000Z",
  break_minutes: 30,
};

const meta = {
  title: "components/staff/AdminSessionEditModal",
  component: AdminSessionEditModal,
  args: {
    isOpen: true,
    mode: "nachtragen",
    staffId: "1",
    date: new Date("2026-06-10"),
    session: null,
    onClose: () => {},
    onSaved: () => {},
  },
} satisfies Meta<typeof AdminSessionEditModal>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Nachtragen: Story = {};

export const Edit: Story = {
  args: {
    mode: "edit",
    session: sampleSession,
  },
};

const recordedBreaks = [
  {
    id: "81",
    started_at: "2026-06-10T10:00:00Z",
    ended_at: "2026-06-10T10:15:00Z",
    duration_minutes: 15,
  },
  {
    id: "82",
    started_at: "2026-06-10T12:00:00Z",
    ended_at: "2026-06-10T12:15:00Z",
    duration_minutes: 15,
  },
];

export const RecordedBreaks: Story = {
  args: { mode: "edit", session: { ...sampleSession, breaks: recordedBreaks } },
};
