import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { PickupDayEditModal } from "./pickup-day-edit-modal";
import type { DayData } from "@/lib/pickup-schedule-helpers";

const meta = {
  title: "students/PickupDayEditModal",
  component: PickupDayEditModal,
  parameters: {
    layout: "fullscreen",
  },
  args: {
    isOpen: true,
    onClose: fn(),
    onSaveException: fn(async () => {}),
    onDeleteException: fn(async () => {}),
    onCreateNote: fn(async () => {}),
    onUpdateNote: fn(async () => {}),
    onDeleteNote: fn(async () => {}),
  },
} satisfies Meta<typeof PickupDayEditModal>;

export default meta;

type Story = StoryObj<typeof meta>;

const baseDay: DayData = {
  date: new Date("2026-07-06"),
  weekday: 1,
  isToday: false,
  showSick: false,
  showClassTrip: false,
  showExcused: false,
  exception: undefined,
  baseSchedule: {
    id: "1",
    studentId: "42",
    weekday: 1,
    weekdayName: "Montag",
    pickupTime: "15:00",
    createdBy: "1",
    createdAt: "2026-07-01T10:00:00Z",
    updatedAt: "2026-07-01T10:00:00Z",
  },
  effectiveTime: "15:00",
  effectiveNotes: undefined,
  isException: false,
  notes: [],
};

export const Default: Story = {
  args: {
    day: baseDay,
  },
};

export const WithException: Story = {
  args: {
    day: {
      ...baseDay,
      isException: true,
      exception: {
        id: "10",
        studentId: "42",
        exceptionDate: "2026-07-06",
        pickupTime: "16:30",
        reason: "Arzttermin",
        source: "staff",
        createdBy: "1",
        createdAt: "2026-07-01T10:00:00Z",
        updatedAt: "2026-07-01T10:00:00Z",
      },
      effectiveTime: "16:30",
    },
  },
};

export const WithNotes: Story = {
  args: {
    day: {
      ...baseDay,
      notes: [
        {
          id: "1",
          studentId: "42",
          noteDate: "2026-07-06",
          content: "Wird heute von der Oma abgeholt.",
          createdBy: "1",
          createdAt: "2026-07-01T10:00:00Z",
          updatedAt: "2026-07-01T10:00:00Z",
        },
        {
          id: "2",
          studentId: "42",
          noteDate: "2026-07-06",
          content: "Bitte Hausaufgabenheft mitgeben.",
          createdBy: "1",
          createdAt: "2026-07-01T10:00:00Z",
          updatedAt: "2026-07-01T10:00:00Z",
        },
      ],
    },
  },
};

export const Closed: Story = {
  args: {
    isOpen: false,
    day: baseDay,
  },
};
