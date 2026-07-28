import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { CarePlanEditorModal } from "./care-plan-editor-modal";
import type { ArrivalDayData } from "~/lib/arrival-schedule-helpers";
import type { DayData as PickupDayData } from "~/lib/pickup-schedule-helpers";

const baseDate = new Date("2026-07-01T00:00:00");

const regularArrivalDay: ArrivalDayData = {
  date: baseDate,
  weekday: 3,
  isToday: false,
  showSick: false,
  showClassTrip: false,
  showExcused: false,
  exception: undefined,
  baseSchedule: {
    id: 1,
    student_id: 1,
    weekday: 3,
    weekday_name: "Mittwoch",
    expected_arrival: "08:00",
    notes: null,
    created_by: 1,
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-01T00:00:00Z",
  },
  effectiveTime: "08:00",
  effectiveReason: undefined,
  isException: false,
  isAbsent: false,
  notes: [],
};

const regularPickupDay: PickupDayData = {
  date: baseDate,
  weekday: 3,
  isToday: false,
  showSick: false,
  showClassTrip: false,
  showExcused: false,
  exception: undefined,
  baseSchedule: {
    id: "1",
    studentId: "1",
    weekday: 3,
    weekdayName: "Mittwoch",
    pickupTime: "16:00",
    notes: "Bus",
    createdBy: "1",
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  },
  effectiveTime: "16:00",
  effectiveNotes: "Bus",
  isException: false,
  notes: [],
};

const guardianArrivalDay: ArrivalDayData = {
  ...regularArrivalDay,
  exception: {
    id: 10,
    student_id: 1,
    exception_date: "2026-07-01",
    expected_arrival: "09:30",
    reason: "Zahnarzttermin",
    source: "guardian",
    created_by: 2,
    created_at: "2026-06-30T00:00:00Z",
    updated_at: "2026-06-30T00:00:00Z",
  },
  effectiveTime: "09:30",
  effectiveReason: "Zahnarzttermin",
  isException: true,
  notes: [
    {
      id: 5,
      student_id: 1,
      note_date: "2026-07-01",
      content: "Bitte an der Tür abholen",
      created_by: 2,
      created_at: "2026-06-30T00:00:00Z",
      updated_at: "2026-06-30T00:00:00Z",
    },
  ],
};

const guardianPickupDay: PickupDayData = {
  ...regularPickupDay,
  exception: {
    id: "20",
    studentId: "1",
    exceptionDate: "2026-07-01",
    pickupTime: "15:00",
    reason: "Fußballtraining",
    source: "guardian",
    createdBy: "2",
    createdAt: "2026-06-30T00:00:00Z",
    updatedAt: "2026-06-30T00:00:00Z",
  },
  effectiveTime: "15:00",
  effectiveNotes: "Fußballtraining",
  isException: true,
  notes: [
    {
      id: "6",
      studentId: "1",
      noteDate: "2026-07-01",
      content: "Wird von Oma abgeholt",
      createdBy: "2",
      createdAt: "2026-06-30T00:00:00Z",
      updatedAt: "2026-06-30T00:00:00Z",
    },
  ],
};

const weeklyArrival = [
  { weekday: 1, expected_arrival: "08:00", notes: null },
  { weekday: 2, expected_arrival: "08:00", notes: null },
  { weekday: 3, expected_arrival: "08:00", notes: null },
  { weekday: 4, expected_arrival: "", notes: null },
  { weekday: 5, expected_arrival: "", notes: null },
];

const weeklyPickup = [
  { weekday: 1, pickupTime: "16:00", notes: "Bus" },
  { weekday: 2, pickupTime: "16:00" },
  { weekday: 3, pickupTime: "16:00", notes: "Bus" },
  { weekday: 4, pickupTime: "" },
  { weekday: 5, pickupTime: "" },
];

const meta: Meta<typeof CarePlanEditorModal> = {
  title: "students/CarePlanEditorModal",
  component: CarePlanEditorModal,
  parameters: {
    layout: "fullscreen",
  },
  args: {
    isOpen: true,
    onClose: fn(),
    date: baseDate,
    arrivalDay: regularArrivalDay,
    pickupDay: regularPickupDay,
    weeklyArrival,
    weeklyPickup,
    onSubmitException: fn().mockResolvedValue(undefined),
    onSubmitWeekly: fn().mockResolvedValue(undefined),
  },
};

export default meta;

type Story = StoryObj<typeof CarePlanEditorModal>;

/** Opened from a day card: the exception for that date. */
export const Ausnahme: Story = {};

/** Opened from the week header: the recurring plan. */
export const Wochenplan: Story = {
  args: {
    date: null,
    arrivalDay: null,
    pickupDay: null,
  },
};

export const GuardianAuthored: Story = {
  args: {
    arrivalDay: guardianArrivalDay,
    pickupDay: guardianPickupDay,
  },
};

export const Closed: Story = {
  args: {
    isOpen: false,
  },
};
