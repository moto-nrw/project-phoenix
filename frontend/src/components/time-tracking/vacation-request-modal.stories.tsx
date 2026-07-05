import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { fn } from "storybook/test";

import { ToastProvider } from "~/contexts/ToastContext";
import type { StaffAbsence } from "~/lib/time-tracking-helpers";

import { VacationRequestModal } from "./vacation-request-modal";

const existingVacations: readonly StaffAbsence[] = [
  {
    id: "1",
    staffId: "1",
    absenceType: "vacation",
    dateStart: "2026-07-10",
    dateEnd: "2026-07-14",
    halfDay: false,
    startHalfDay: false,
    endHalfDay: false,
    note: "",
    status: "approved",
    approvedBy: null,
    approvedAt: null,
    createdBy: "1",
    createdAt: "2026-06-01T00:00:00Z",
    updatedAt: "2026-06-01T00:00:00Z",
    durationDays: 5,
    workingDays: 5,
    decisionNote: "",
    requestedAt: "2026-06-01T00:00:00Z",
    substituteStaffId: null,
  },
];

const meta = {
  title: "components/time-tracking/VacationRequestModal",
  component: VacationRequestModal,
  decorators: [
    (Story) => (
      <ToastProvider>
        <Story />
      </ToastProvider>
    ),
  ],
  args: {
    isOpen: true,
    onClose: fn(),
    onSubmitted: fn(),
    remainingDays: 10,
    existingVacations: [],
  },
} satisfies Meta<typeof VacationRequestModal>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Default: Story = {};

export const WithExistingVacations: Story = {
  args: {
    remainingDays: 3,
    existingVacations,
  },
};
