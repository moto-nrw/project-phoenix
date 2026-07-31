import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { installStorybookFetch, jsonResponse } from "~storybook/mocks/fetch";
import { ToastProvider } from "~/contexts/ToastContext";
import GuardianApprovalQueue from "~/components/admin/guardian-approval-queue";

function mockApprovals(rows: unknown[]) {
  return installStorybookFetch(({ url }) => {
    if (!url.includes("/api/guardians/invitations/pending-approval")) {
      return undefined;
    }
    return jsonResponse({ status: "success", data: rows });
  });
}

const sampleRequest = {
  id: "42",
  guardian_profile_id: "10",
  guardian_name: "Julia Schröder",
  guardian_email: "julia.schroeder@example.com",
  student_id: "1",
  student_name: "Felix Schneider",
  requested_by_email: "karin.klein@example.com",
  created_at: "2026-06-10T08:00:00Z",
  expires_at: "2026-06-12T08:00:00Z",
};

const meta = {
  title: "admin/GuardianApprovalQueue",
  component: GuardianApprovalQueue,
  args: { inviteModeState: { status: "ready", mode: "staff_approval" } },
  decorators: [
    (Story) => (
      <ToastProvider>
        <div className="max-w-3xl">
          <Story />
        </div>
      </ToastProvider>
    ),
  ],
} satisfies Meta<typeof GuardianApprovalQueue>;

export default meta;

type Story = StoryObj<typeof meta>;

// The component fetches pending guardian approvals on mount via the real
// guardian-api client. In Storybook there is no backend to answer that
// request, so it exercises the component's own error-handling path
// (loading -> failed fetch -> inline error banner with retry button),
// which is a real, user-visible state of this component.
export const Default: Story = {};

export const WithRequest: Story = {
  loaders: [
    () => {
      mockApprovals([sampleRequest]);
      return {};
    },
  ],
  args: { inviteModeState: { status: "ready", mode: "staff_approval" } },
};

// Approval mode active, queue drained: the calm resting state.
export const EmptyWaitingForRequests: Story = {
  loaders: [
    () => {
      mockApprovals([]);
      return {};
    },
  ],
  args: { inviteModeState: { status: "ready", mode: "staff_approval" } },
};

// Parents cannot invite at all, so this queue can never fill up. The empty
// state says so and links to the setting that changes it.
export const EmptyInvitesDisabled: Story = {
  loaders: [
    () => {
      mockApprovals([]);
      return {};
    },
  ],
  args: { inviteModeState: { status: "ready", mode: "disabled" } },
};

// Invitations go out without a review step, same consequence for this page.
export const EmptyInvitesDirect: Story = {
  loaders: [
    () => {
      mockApprovals([]);
      return {};
    },
  ],
  args: { inviteModeState: { status: "ready", mode: "direct" } },
};
