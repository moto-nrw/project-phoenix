import type { Meta, StoryObj } from "@storybook/nextjs-vite";
import { ToastProvider } from "~/contexts/ToastContext";
import GuardianApprovalQueue from "~/components/admin/guardian-approval-queue";

const meta = {
  title: "admin/GuardianApprovalQueue",
  component: GuardianApprovalQueue,
  decorators: [
    (Story) => (
      <ToastProvider>
        <div className="max-w-xl">
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
