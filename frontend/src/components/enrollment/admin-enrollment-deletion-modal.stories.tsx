import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { installStorybookFetch, jsonResponse } from "~storybook/mocks/fetch";
import type { AdminEnrollmentDeletionImpact } from "~/lib/enrollment-admin-api";
import { AdminEnrollmentDeletionModal } from "./admin-enrollment-deletion-modal";

const safeImpact: AdminEnrollmentDeletionImpact = {
  request_id: "1842",
  deletes_request: true,
  can_delete: true,
  counts: {
    requests: 1,
    request_children: 2,
    request_child_offerings: 4,
    request_guardians: 1,
    change_requests: 1,
    change_request_messages: 2,
    late_invites: 0,
    offering_adjustments: 1,
    email_outbox: 3,
    rollover_links_cleared: 0,
    student_source_links_cleared: 0,
    total: 15,
  },
  blocking_student_ids: [],
  preserved_guardian_profiles: 2,
  preserved_parent_accounts: 1,
  unlinked_guardian_profiles: 1,
  parent_accounts_without_students: 1,
};

function withDeletionImpact(impact: AdminEnrollmentDeletionImpact) {
  return installStorybookFetch(({ url, method }) => {
    if (url.includes("/delete-impact") && method === "GET") {
      return jsonResponse({ data: impact });
    }
    return undefined;
  });
}

const meta = {
  title: "enrollment/AdminEnrollmentDeletionModal",
  component: AdminEnrollmentDeletionModal,
  args: {
    isOpen: true,
    requestId: "1842",
    studentHref: (studentId: string) => `/students/${studentId}`,
    onClose: () => undefined,
    onDeleted: () => undefined,
  },
  parameters: {
    layout: "fullscreen",
  },
} satisfies Meta<typeof AdminEnrollmentDeletionModal>;

export default meta;
type Story = StoryObj<typeof meta>;

export const ImpactPreview: Story = {
  beforeEach: () => withDeletionImpact(safeImpact),
};

export const ExistingStudentBlocked: Story = {
  beforeEach: () =>
    withDeletionImpact({
      ...safeImpact,
      can_delete: false,
      blocking_student_ids: ["731"],
    }),
};

export const LastChildDeletesRequest: Story = {
  args: {
    childId: "901",
    childLabel: "Zurückgezogenes Kind",
  },
  beforeEach: () =>
    withDeletionImpact({
      ...safeImpact,
      child_id: "901",
      counts: {
        ...safeImpact.counts,
        request_children: 1,
        request_child_offerings: 2,
        total: 10,
      },
    }),
};
