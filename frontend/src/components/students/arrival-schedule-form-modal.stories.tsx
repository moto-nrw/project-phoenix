import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { ArrivalScheduleFormModal } from "./arrival-schedule-form-modal";
import type { ArrivalScheduleFormEntry } from "~/lib/arrival-schedule-helpers";
import { WEEKDAYS } from "~/lib/arrival-schedule-helpers";

const emptySchedules: ArrivalScheduleFormEntry[] = WEEKDAYS.map((day) => ({
  weekday: day.value,
  inCare: true,
  expected_arrival: "",
  notes: null,
}));

const filledSchedules: ArrivalScheduleFormEntry[] = WEEKDAYS.map((day) => ({
  weekday: day.value,
  inCare: true,
  expected_arrival: day.value === 5 ? "" : "08:00",
  notes: day.value === 1 ? "Kommt mit dem Bus" : null,
}));

const meta = {
  title: "students/ArrivalScheduleFormModal",
  component: ArrivalScheduleFormModal,
  parameters: {
    layout: "fullscreen",
  },
  args: {
    isOpen: true,
    careDaysSource: "weekly_plan",
    onClose: fn(),
    onSubmit: fn(async () => {}),
    initialSchedules: emptySchedules,
  },
} satisfies Meta<typeof ArrivalScheduleFormModal>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Empty: Story = {
  args: {
    initialSchedules: emptySchedules,
  },
};

export const WithSchedules: Story = {
  args: {
    initialSchedules: filledSchedules,
  },
};

export const Closed: Story = {
  args: {
    isOpen: false,
    initialSchedules: emptySchedules,
  },
};
