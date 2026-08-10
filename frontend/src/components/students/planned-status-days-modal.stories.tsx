import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { PlannedStatusDaysModal } from "./planned-status-days-modal";
import type { StudentStatusDay } from "~/lib/student-status-days-api";

const existingDays: StudentStatusDay[] = [
  {
    id: "1",
    student_id: "1",
    date: "2026-07-02",
    status: "sick",
    label: "Krank",
    reported_at: "2026-07-01T08:00:00Z",
    source: "staff",
    note: "Fieber",
    created_at: "2026-07-01T08:00:00Z",
    updated_at: "2026-07-01T08:00:00Z",
  },
];

const meta = {
  title: "students/PlannedStatusDaysModal",
  component: PlannedStatusDaysModal,
  parameters: {
    layout: "fullscreen",
  },
  args: {
    isOpen: true,
    status: "sick",
    studentName: "Max Mustermann",
    isSubmitting: false,
    onClose: () => {
      // no-op for story
    },
    loadExistingDays: async () => [],
    onSubmit: async () => {
      // no-op for story
    },
  },
} satisfies Meta<typeof PlannedStatusDaysModal>;

export default meta;

type Story = StoryObj<typeof meta>;

export const Sick: Story = {};

export const Excused: Story = {
  args: {
    status: "excused",
  },
};

export const ClassTrip: Story = {
  args: {
    status: "class_trip",
  },
};

export const WithExistingDays: Story = {
  args: {
    existingDays,
    onDeleteStatusDay: async () => {
      // no-op for story
    },
  },
};

export const Submitting: Story = {
  args: {
    isSubmitting: true,
  },
};
