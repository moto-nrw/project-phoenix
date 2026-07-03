import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import type { ArrivalDayData } from "~/lib/arrival-schedule-helpers";
import { ArrivalDayEditModal } from "./arrival-day-edit-modal";

const baseDay: ArrivalDayData = {
  date: new Date("2026-07-01"),
  weekday: 3,
  isToday: true,
  showSick: false,
  showClassTrip: false,
  showExcused: false,
  exception: undefined,
  baseSchedule: {
    id: 1,
    student_id: 1,
    weekday: 3,
    weekday_name: "Mittwoch",
    expected_arrival: "13:30:00",
    created_by: 1,
    created_at: "2026-06-01T00:00:00Z",
    updated_at: "2026-06-01T00:00:00Z",
  },
  effectiveTime: "13:30",
  effectiveReason: undefined,
  isException: false,
  isAbsent: false,
  notes: [],
};

const meta = {
  title: "students/ArrivalDayEditModal",
  component: ArrivalDayEditModal,
  args: {
    isOpen: true,
    onClose: fn(),
    day: baseDay,
    onSaveException: fn(async () => {}),
    onDeleteException: fn(async () => {}),
    onCreateNote: fn(async () => {}),
    onUpdateNote: fn(async () => {}),
    onDeleteNote: fn(async () => {}),
  },
} satisfies Meta<typeof ArrivalDayEditModal>;

export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const NoBaseSchedule: Story = {
  args: {
    day: {
      ...baseDay,
      baseSchedule: undefined,
      effectiveTime: undefined,
    },
  },
};

export const WithException: Story = {
  args: {
    day: {
      ...baseDay,
      isException: true,
      exception: {
        id: 5,
        student_id: 1,
        exception_date: "2026-07-01",
        expected_arrival: "14:00:00",
        reason: "Zahnarzttermin",
        created_by: 1,
        created_at: "2026-06-30T00:00:00Z",
        updated_at: "2026-06-30T00:00:00Z",
      },
      effectiveTime: "14:00",
      effectiveReason: "Zahnarzttermin",
    },
  },
};

export const WithNotes: Story = {
  args: {
    day: {
      ...baseDay,
      notes: [
        {
          id: 1,
          student_id: 1,
          note_date: "2026-07-01",
          content: "Wird von der Oma abgeholt.",
          created_by: 1,
          created_at: "2026-06-30T00:00:00Z",
          updated_at: "2026-06-30T00:00:00Z",
        },
      ],
    },
  },
};

export const Closed: Story = {
  args: {
    isOpen: false,
  },
};
