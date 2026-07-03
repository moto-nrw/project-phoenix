import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { AdminEnrollmentPhaseDetail } from "~/components/enrollment/admin-enrollment-phase-detail";
import { BreadcrumbProvider } from "~/lib/breadcrumb-context";
import { ToastProvider } from "~/contexts/ToastContext";

const meta: Meta<typeof AdminEnrollmentPhaseDetail> = {
  title: "components/enrollment/AdminEnrollmentPhaseDetail",
  component: AdminEnrollmentPhaseDetail,
  decorators: [
    (Story) => (
      <ToastProvider>
        <BreadcrumbProvider>
          <Story />
        </BreadcrumbProvider>
      </ToastProvider>
    ),
  ],
};

export default meta;
type Story = StoryObj<typeof AdminEnrollmentPhaseDetail>;

/**
 * The component fetches its phase + request data client-side (`listPhases`,
 * `listAdminRequests`). There is no backend in Storybook, so the fetches
 * fail and the component renders its own graceful error/loading state
 * instead of crashing — this story documents that fallback behavior.
 */
export const PhaseNotFound: Story = {
  args: {
    phaseId: "storybook-phase-id",
  },
};
