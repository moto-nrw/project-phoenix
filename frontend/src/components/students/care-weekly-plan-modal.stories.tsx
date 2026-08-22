import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { ToastProvider } from "~/contexts/ToastContext";
import { CareWeeklyPlanModal } from "./care-weekly-plan-modal";

const meta: Meta<typeof CareWeeklyPlanModal> = {
  title: "students/CareWeeklyPlanModal",
  component: CareWeeklyPlanModal,
  decorators: [
    (Story) => (
      <ToastProvider>
        <Story />
      </ToastProvider>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof CareWeeklyPlanModal>;

export const Empty: Story = {
  args: {
    isOpen: true,
    onClose: () => {},
    initialArrivalSchedules: [],
    initialPickupSchedules: [],
    onSubmit: async () => {},
  },
};

export const WithSchedules: Story = {
  args: {
    isOpen: true,
    onClose: () => {},
    initialArrivalSchedules: [
      {
        weekday: 1,
        inCare: true,
        expected_arrival: "08:00",
        notes: "Kommt mit dem Bus",
      },
      { weekday: 3, inCare: true, expected_arrival: "08:15", notes: null },
    ],
    initialPickupSchedules: [
      { weekday: 1, pickupTime: "16:00", notes: "Oma holt ab" },
      { weekday: 3, pickupTime: "15:30" },
    ],
    onSubmit: async () => {},
  },
};
