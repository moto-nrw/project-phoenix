import type { Meta, StoryObj } from "@storybook/nextjs-vite";

import { installStorybookFetch, jsonResponse } from "~storybook/mocks/fetch";
import type { StudentDeletionImpact } from "~/lib/student-api";

import { StudentDeletionModal } from "./student-deletion-modal";

const impact: StudentDeletionImpact = {
  confirmation_name: "Mia Musterkind",
  fingerprint: "storybook-student-deletion-fingerprint",
  total: 28,
  counts: {
    timetable_assignments: 4,
    activity_enrollments: 2,
    attendance_records: 8,
    care_schedules: 5,
    guardian_links: 2,
    companion_links: 1,
    communications: 3,
    consents: 1,
    enrollment_references: 1,
    other_records: 1,
  },
  preserved: {
    guardian_profiles: true,
    parent_accounts: true,
    other_students: true,
    shared_instances: true,
  },
};

function withStudentDeletionApi() {
  return installStorybookFetch(({ url, method }) => {
    if (url.includes("/delete-impact") && method === "GET") {
      return jsonResponse({ data: impact });
    }
    if (url.endsWith("/api/students/42") && method === "DELETE") {
      return jsonResponse({ data: null });
    }
    return undefined;
  });
}

const meta = {
  title: "students/StudentDeletionModal",
  component: StudentDeletionModal,
  args: {
    isOpen: true,
    studentId: "42",
    displayName: "Mia Musterkind",
    onClose: () => undefined,
    onDeleted: () => undefined,
  },
  parameters: {
    layout: "fullscreen",
  },
  beforeEach: withStudentDeletionApi,
} satisfies Meta<typeof StudentDeletionModal>;

export default meta;
type Story = StoryObj<typeof meta>;

export const ImpactPreview: Story = {};
