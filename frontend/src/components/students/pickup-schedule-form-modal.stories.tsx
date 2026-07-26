import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";
import { PickupScheduleFormModal } from "./pickup-schedule-form-modal";
import type { PickupScheduleFormData } from "@/lib/pickup-schedule-helpers";
import { WEEKDAYS } from "@/lib/pickup-schedule-helpers";

const emptySchedules: PickupScheduleFormData[] = WEEKDAYS.map((day) => ({
  weekday: day.value,
  pickupTime: "",
  notes: undefined,
}));

const filledSchedules: PickupScheduleFormData[] = WEEKDAYS.map((day) => ({
  weekday: day.value,
  pickupTime: day.value === 5 ? "" : "16:00",
  notes: day.value === 1 ? "Wird von der Oma abgeholt" : undefined,
}));

const meta = {
  title: "students/PickupScheduleFormModal",
  component: PickupScheduleFormModal,
  parameters: {
    layout: "fullscreen",
  },
  args: {
    isOpen: true,
    onClose: fn(),
    onSubmit: fn(async () => {}),
    initialSchedules: emptySchedules,
  },
} satisfies Meta<typeof PickupScheduleFormModal>;

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
